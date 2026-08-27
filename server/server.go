package server

import (
	"context"
	"errors"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"drugsync/fdaclient"
	"drugsync/normalize"
	drugpb "drugsync/proto"
	"drugsync/store"
)

type DrugServer struct {
	drugpb.UnimplementedDrugServiceServer
	store *store.Store
}

func New(s *store.Store) *DrugServer {
	return &DrugServer{store: s}
}

func (s *DrugServer) GetDrug(ctx context.Context, req *drugpb.GetDrugRequest) (*drugpb.Drug, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	d, err := s.store.GetByID(ctx, req.Id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, status.Errorf(codes.NotFound, "no drug with id %s", req.Id)
	}
	if err != nil {
		log.Printf("GetDrug: %v", err)
		return nil, status.Error(codes.Internal, "could not fetch drug")
	}

	return d, nil
}

func (s *DrugServer) SearchDrugs(req *drugpb.SearchRequest, stream grpc.ServerStreamingServer[drugpb.Drug]) error {
	if req.Query == "" {
		return status.Error(codes.InvalidArgument, "query is required")
	}

	drugs, err := s.store.SearchByBrand(stream.Context(), req.Query, req.Limit)
	if err != nil {
		log.Printf("SearchDrugs: %v", err)
		return status.Error(codes.Internal, "search failed")
	}

	for _, d := range drugs {
		if err := stream.Send(d); err != nil {
			return err
		}
	}

	return nil
}

func (s *DrugServer) SyncFromFDA(ctx context.Context, req *drugpb.SyncRequest) (*drugpb.SyncResponse, error) {
	count := req.Count
	if count <= 0 || count > 100 {
		count = 20
	}

	records, err := fdaclient.Fetch(int(count))
	if err != nil {
		log.Printf("SyncFromFDA fetch: %v", err)
		return nil, status.Error(codes.Unavailable, "could not reach openFDA")
	}

	var saved, skipped int32

	for _, r := range records {
		d, ok := normalize.ToDrug(r)
		if !ok {
			skipped++
			continue
		}

		if err := s.store.Save(ctx, d); err != nil {
			log.Printf("SyncFromFDA save: %v", err)
			skipped++
			continue
		}

		saved++
	}

	return &drugpb.SyncResponse{
		Fetched: int32(len(records)),
		Saved:   saved,
		Skipped: skipped,
	}, nil
}
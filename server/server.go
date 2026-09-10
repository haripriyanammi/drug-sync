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
	"drugsync/notify"
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

// storeError turns a store error into the right gRPC status code.
func storeError(err error, action string) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return status.Error(codes.NotFound, "drug not found")
	case errors.Is(err, store.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, "a drug with that id already exists")
	case errors.Is(err, store.ErrNoFields):
		return status.Error(codes.InvalidArgument, "no fields to update")
	default:
		log.Printf("%s: %v", action, err)
		return status.Error(codes.Internal, action+" failed")
	}
}

// ---------- READ ----------

func (s *DrugServer) GetDrug(ctx context.Context, req *drugpb.GetDrugRequest) (*drugpb.Drug, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	d, err := s.store.GetByID(ctx, req.Id)
	if err != nil {
		return nil, storeError(err, "get drug")
	}
	return d, nil
}

func (s *DrugServer) SearchDrugs(req *drugpb.SearchRequest, stream grpc.ServerStreamingServer[drugpb.Drug]) error {
	if req.Query == "" {
		return status.Error(codes.InvalidArgument, "query is required")
	}

	drugs, err := s.store.SearchByBrand(stream.Context(), req.Query, req.Limit)
	if err != nil {
		return storeError(err, "search drugs")
	}

	for _, d := range drugs {
		if err := stream.Send(d); err != nil {
			return err
		}
	}
	return nil
}

// ---------- CREATE ----------

func (s *DrugServer) CreateDrug(ctx context.Context, req *drugpb.CreateDrugRequest) (*drugpb.Drug, error) {
	if req.Drug == nil {
		return nil, status.Error(codes.InvalidArgument, "drug is required")
	}
	if req.Drug.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "drug.id is required")
	}
	if req.Drug.BrandName == "" && req.Drug.GenericName == "" {
		return nil, status.Error(codes.InvalidArgument, "brand_name or generic_name is required")
	}

	created, err := s.store.Create(ctx, req.Drug)
	if err != nil {
		return nil, storeError(err, "create drug")
	}
	go notify.Drugf("New drug added: **%s** (%s)", created.BrandName, created.Id)

	return created, nil
}

// ---------- UPDATE (full replace) ----------

func (s *DrugServer) UpdateDrug(ctx context.Context, req *drugpb.UpdateDrugRequest) (*drugpb.Drug, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if req.Drug == nil {
		return nil, status.Error(codes.InvalidArgument, "drug is required")
	}
	if req.Drug.BrandName == "" && req.Drug.GenericName == "" {
		return nil, status.Error(codes.InvalidArgument, "brand_name or generic_name is required")
	}

	updated, err := s.store.Update(ctx, req.Id, req.Drug)
	if err != nil {
		return nil, storeError(err, "update drug")
	}
	return updated, nil
}

// ---------- PATCH (partial update) ----------

func (s *DrugServer) PatchDrug(ctx context.Context, req *drugpb.PatchDrugRequest) (*drugpb.Drug, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	fields := map[string]string{}

	if req.BrandName != nil {
		fields["brand_name"] = *req.BrandName
	}
	if req.GenericName != nil {
		fields["generic_name"] = *req.GenericName
	}
	if req.Manufacturer != nil {
		fields["manufacturer"] = *req.Manufacturer
	}
	if req.ProductNdc != nil {
		fields["product_ndc"] = *req.ProductNdc
	}
	if req.ProductType != nil {
		fields["product_type"] = *req.ProductType
	}
	if req.Route != nil {
		fields["route"] = *req.Route
	}
	if req.SubstanceName != nil {
		fields["substance_name"] = *req.SubstanceName
	}

	patched, err := s.store.Patch(ctx, req.Id, fields)
	if err != nil {
		return nil, storeError(err, "patch drug")
	}
	return patched, nil
}

// ---------- DELETE ----------

func (s *DrugServer) DeleteDrug(ctx context.Context, req *drugpb.DeleteDrugRequest) (*drugpb.DeleteDrugResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	if err := s.store.Delete(ctx, req.Id); err != nil {
		return nil, storeError(err, "delete drug")
	}
	go notify.Drugf("Drug deleted: `%s`", req.Id)
	return &drugpb.DeleteDrugResponse{Deleted: true}, nil
}

// ---------- ADMIN ----------

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
	go notify.Drugf("Sync complete: %d fetched, %d saved, %d skipped", len(records), saved, skipped)
	return &drugpb.SyncResponse{
		Fetched: int32(len(records)),
		Saved:   saved,
		Skipped: skipped,
	}, nil
}

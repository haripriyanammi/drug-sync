package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	drugpb "drugsync/proto"
)

var client drugpb.DrugServiceClient

func main() {
	conn, err := grpc.NewClient("localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connecting to gRPC server: %v", err)
	}
	defer conn.Close()

	client = drugpb.NewDrugServiceClient(conn)

	http.HandleFunc("/drugs", handleSearch)
	http.HandleFunc("/drugs/", handleGetOne)

	log.Println("REST gateway listening on :8080")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("serving: %v", err)
	}
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "missing ?q= parameter")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	stream, err := client.SearchDrugs(ctx, &drugpb.SearchRequest{
		Query: query,
		Limit: int32(limit),
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	drugs := []*drugpb.Drug{}
	for {
		d, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		drugs = append(drugs, d)
	}

	writeJSON(w, http.StatusOK, drugs)
}

func handleGetOne(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/drugs/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing drug id")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	d, err := client.GetDrug(ctx, &drugpb.GetDrugRequest{Id: id})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, d)
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func writeGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		writeError(w, http.StatusInternalServerError, "unexpected error")
		return
	}

	httpCode := http.StatusInternalServerError
	switch st.Code() {
	case codes.NotFound:
		httpCode = http.StatusNotFound
	case codes.InvalidArgument:
		httpCode = http.StatusBadRequest
	case codes.Unavailable:
		httpCode = http.StatusServiceUnavailable
	}

	writeError(w, httpCode, st.Message())
}
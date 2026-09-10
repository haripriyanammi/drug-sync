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

// drugJSON is the shape the browser sends and receives.
// Pointers so we can tell "not sent" from "sent as empty" for PATCH.
type drugJSON struct {
	Id            *string `json:"id,omitempty"`
	BrandName     *string `json:"brand_name,omitempty"`
	GenericName   *string `json:"generic_name,omitempty"`
	Manufacturer  *string `json:"manufacturer,omitempty"`
	ProductNdc    *string `json:"product_ndc,omitempty"`
	ProductType   *string `json:"product_type,omitempty"`
	Route         *string `json:"route,omitempty"`
	SubstanceName *string `json:"substance_name,omitempty"`
}

func main() {
	conn, err := grpc.NewClient("localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connecting to gRPC server: %v", err)
	}
	defer conn.Close()

	client = drugpb.NewDrugServiceClient(conn)

	http.HandleFunc("/drugs", handleCollection)
	http.HandleFunc("/drugs/", handleItem)
	http.HandleFunc("/sync", handleSync)

	log.Println("REST gateway listening on :8080")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("serving: %v", err)
	}
}

// ---------- CORS ----------

// setCORS allows the browser page on :3000 to call this gateway on :8080.
func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// handlePreflight answers the browser's OPTIONS check that comes
// before every DELETE, PUT and PATCH.
func handlePreflight(w http.ResponseWriter) {
	setCORS(w)
	w.WriteHeader(http.StatusNoContent)
}

// ---------- ROUTERS ----------

// /drugs  → GET (search) or POST (create)
func handleCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodOptions:
		handlePreflight(w)
	case http.MethodGet:
		handleSearch(w, r)
	case http.MethodPost:
		handleCreate(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// /drugs/{id}  → GET, PUT, PATCH or DELETE
func handleItem(w http.ResponseWriter, r *http.Request) {
	// A preflight carries no real id, so answer it before checking one.
	if r.Method == http.MethodOptions {
		handlePreflight(w)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/drugs/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing drug id")
		return
	}

	switch r.Method {
	case http.MethodGet:
		handleGetOne(w, r, id)
	case http.MethodPut:
		handleUpdate(w, r, id)
	case http.MethodPatch:
		handlePatch(w, r, id)
	case http.MethodDelete:
		handleDelete(w, r, id)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ---------- READ ----------

func handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "missing ?q= parameter")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	ctx, cancel := reqContext(r)
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

func handleGetOne(w http.ResponseWriter, r *http.Request, id string) {
	ctx, cancel := reqContext(r)
	defer cancel()

	d, err := client.GetDrug(ctx, &drugpb.GetDrugRequest{Id: id})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, d)
}

// ---------- CREATE ----------

func handleCreate(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeBody(w, r)
	if !ok {
		return
	}

	ctx, cancel := reqContext(r)
	defer cancel()

	created, err := client.CreateDrug(ctx, &drugpb.CreateDrugRequest{
		Drug: toDrug(body),
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

// ---------- UPDATE (full replace) ----------

func handleUpdate(w http.ResponseWriter, r *http.Request, id string) {
	body, ok := decodeBody(w, r)
	if !ok {
		return
	}

	ctx, cancel := reqContext(r)
	defer cancel()

	updated, err := client.UpdateDrug(ctx, &drugpb.UpdateDrugRequest{
		Id:   id,
		Drug: toDrug(body),
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

// ---------- PATCH (partial update) ----------

func handlePatch(w http.ResponseWriter, r *http.Request, id string) {
	body, ok := decodeBody(w, r)
	if !ok {
		return
	}

	ctx, cancel := reqContext(r)
	defer cancel()

	// Pointers pass straight through: nil here means nil in the request,
	// which means "don't change this column".
	patched, err := client.PatchDrug(ctx, &drugpb.PatchDrugRequest{
		Id:            id,
		BrandName:     body.BrandName,
		GenericName:   body.GenericName,
		Manufacturer:  body.Manufacturer,
		ProductNdc:    body.ProductNdc,
		ProductType:   body.ProductType,
		Route:         body.Route,
		SubstanceName: body.SubstanceName,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, patched)
}

// ---------- DELETE ----------

func handleDelete(w http.ResponseWriter, r *http.Request, id string) {
	ctx, cancel := reqContext(r)
	defer cancel()

	_, err := client.DeleteDrug(ctx, &drugpb.DeleteDrugRequest{Id: id})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	setCORS(w)
	w.WriteHeader(http.StatusNoContent)
}

// ---------- ADMIN ----------

func handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		handlePreflight(w)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	count, _ := strconv.Atoi(r.URL.Query().Get("count"))

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	resp, err := client.SyncFromFDA(ctx, &drugpb.SyncRequest{Count: int32(count)})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// ---------- HELPERS ----------

func reqContext(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 10*time.Second)
}

// decodeBody reads the JSON body into a drugJSON.
func decodeBody(w http.ResponseWriter, r *http.Request) (*drugJSON, bool) {
	var body drugJSON
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return nil, false
	}
	return &body, true
}

// toDrug converts the JSON body into a protobuf Drug.
// Missing fields become empty strings, which is correct for POST and PUT.
func toDrug(b *drugJSON) *drugpb.Drug {
	return &drugpb.Drug{
		Id:            str(b.Id),
		BrandName:     str(b.BrandName),
		GenericName:   str(b.GenericName),
		Manufacturer:  str(b.Manufacturer),
		ProductNdc:    str(b.ProductNdc),
		ProductType:   str(b.ProductType),
		Route:         str(b.Route),
		SubstanceName: str(b.SubstanceName),
	}
}

// str turns a *string into a string, treating nil as "".
func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	setCORS(w)
	w.Header().Set("Content-Type", "application/json")
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
	case codes.AlreadyExists:
		httpCode = http.StatusConflict
	case codes.Unavailable:
		httpCode = http.StatusServiceUnavailable
	}

	writeError(w, httpCode, st.Message())
}

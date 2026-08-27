package main

import (
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	drugpb "drugsync/proto"
	"drugsync/server"
	"drugsync/store"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://haripriya@localhost:5432/drugsync?sslmode=disable"
	}

	st, err := store.New(dsn)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer st.Close()

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("listening: %v", err)
	}

	grpcServer := grpc.NewServer()
	drugpb.RegisterDrugServiceServer(grpcServer, server.New(st))
	reflection.Register(grpcServer)

	log.Println("gRPC server listening on :50051")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("serving: %v", err)
	}
}
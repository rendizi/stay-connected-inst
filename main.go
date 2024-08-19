package main

import (
	"fmt"
	grpc2 "github.com/rendizi/stay-connected-inst/internal/grpc"
	server2 "github.com/rendizi/stay-connected-inst/internal/server"
	"google.golang.org/grpc"
	"log"
	"net"
)

func main() {
	grpcServer := grpc.NewServer()

	grpc2.RegisterStoriesSummarizerServer(grpcServer, &server2.Server{})

	// Listen on port 50051
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Serve gRPC server
	fmt.Println("Server is running on port 50051")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

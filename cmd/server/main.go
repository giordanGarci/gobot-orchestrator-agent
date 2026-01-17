package main

import (
	"fmt"
	"log"
	"net"
	"orchestrator/internal/orchestrator"
	"orchestrator/pb"

	"google.golang.org/grpc"
)

func main() {
	fmt.Println("starting server grpc")
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	orchestratorService := orchestrator.NewOrchestratorServiceServer()
	pb.RegisterOrchestratorServiceServer(grpcServer, orchestratorService)

	if err := grpcServer.Serve(lis); err != nil {
		panic(err)
	}

}

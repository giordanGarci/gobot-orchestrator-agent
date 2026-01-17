package main

import (
	"fmt"
	"log"
	"net/http"
	"orchestrator/internal/handlers"
	"orchestrator/pb"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	initTime := time.Now()
	defer func() {
		fmt.Printf("Execution time: %s\n", time.Since(initTime))
	}()

	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	orchestratorClient := pb.NewOrchestratorServiceClient(conn)

	handler := &handlers.BotHandler{
		AgentClient: orchestratorClient,
	}

	http.HandleFunc("POST /run", handler.RunBotHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))

}

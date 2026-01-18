package main

import (
	"fmt"
	"log"
	"net/http"
	"orchestrator/internal/handlers"
	"orchestrator/internal/templates"
	"orchestrator/pb"
	"time"

	"github.com/a-h/templ"
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
	fmt.Println("Started grpc client")

	handler := &handlers.BotHandler{
		AgentClient: orchestratorClient,
	}

	http.HandleFunc("POST /bots/run", handler.RunBotHandler)
	http.Handle("/", templ.Handler(templates.Layout(templates.DeployForm())))

	fmt.Println("and starting HTTP server on :8080")

	log.Fatal(http.ListenAndServe(":8080", nil))

}

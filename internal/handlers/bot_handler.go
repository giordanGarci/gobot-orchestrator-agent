package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"orchestrator/pb"
	"orchestrator/structs"
)

type BotHandler struct {
	AgentClient pb.OrchestratorServiceClient
}

func NewBotHandler(agentClient pb.OrchestratorServiceClient) *BotHandler {
	return &BotHandler{
		AgentClient: agentClient,
	}
}

func (h *BotHandler) RunBotHandler(w http.ResponseWriter, r *http.Request) {
	var bot structs.Bot
	if err := json.NewDecoder(r.Body).Decode(&bot); err != nil {
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	stream, err := h.AgentClient.ExecuteDeploy(ctx, &pb.DeployRequest{
		BotId:   bot.BotID,
		GitRepo: bot.GitRepo,
		Version: bot.Version,
	})
	if err != nil {
		http.Error(w, "Failed to start bot deployment: "+err.Error(), http.StatusInternalServerError)
		return
	}
	go func() {
		for {
			logMsg, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("Erro no streaming: %v", err)
				break
			}
			// Here you would typically write the logMsg to a websocket or another streaming mechanism
			// For simplicity, we are just printing it to the server console
			fmt.Printf("[%s] %s: %s\n", bot.BotID, logMsg.Status, logMsg.Line)
		}
	}()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Deploy concluído com sucesso!"))
}

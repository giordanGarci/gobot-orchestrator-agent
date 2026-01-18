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

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming não suportado", http.StatusInternalServerError)
		return
	}

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

	// Loop síncrono - mantém a conexão aberta até terminar
	for {
		logMsg, err := stream.Recv()
		if err == io.EOF {
			// Envia mensagem final
			fmt.Fprintf(w, "data: <div class='text-green-400 font-bold'>✓ Execução finalizada!</div>\n\n")
			flusher.Flush()
			break
		}
		if err != nil {
			log.Printf("Erro no streaming: %v", err)
			fmt.Fprintf(w, "data: <div class='text-red-400'>[ERROR] %v</div>\n\n", err)
			flusher.Flush()
			break
		}

		// Define cor baseada no status
		colorClass := "text-gray-300"
		switch logMsg.Status {
		case "SUCCESS":
			colorClass = "text-green-400"
		case "ERROR":
			colorClass = "text-red-400"
		case "INFO":
			colorClass = "text-blue-300"
		}

		fmt.Fprintf(w, "data: <div class='%s'>[%s] %s</div>\n\n",
			colorClass, logMsg.Status, logMsg.Line)
		flusher.Flush()

		// Log no console do servidor também
		fmt.Printf("[%s] %s: %s\n", bot.BotID, logMsg.Status, logMsg.Line)
	}
}

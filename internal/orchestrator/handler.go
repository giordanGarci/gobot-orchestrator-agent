package orchestrator

import (
	"fmt"
	"orchestrator/pb"
	"orchestrator/structs"
)

type Handler struct {
	pb.UnimplementedOrchestratorServiceServer
	service *OrchestratorService
}

func NewOrchestratorServiceServer() *Handler {
	return &Handler{
		service: NewOrchestratorService(),
	}
}

func (h *Handler) ExecuteDeploy(req *pb.DeployRequest, stream pb.OrchestratorService_ExecuteDeployServer) error {
	fmt.Printf("Received DeployRequest: %+v\n", req)
	logStream := make(chan *pb.LogResponse)
	done := make(chan error, 1)

	deployRequest := &structs.Bot{
		BotID:   req.BotId,
		GitRepo: req.GitRepo,
		Version: req.Version,
	}

	go func() {
		err := h.service.ExecuteDeployment(deployRequest, logStream)
		if err != nil {
			done <- err
		}
		err = h.service.RunBot(deployRequest, logStream)
		if err != nil {
			done <- err
		}
		close(logStream)
		done <- nil
	}()

	for {
		select {
		case logMsg, ok := <-logStream:
			if !ok {
				return <-done
			}
			if err := stream.Send(logMsg); err != nil {
				return err
			}

		case err := <-done:
			return err

		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

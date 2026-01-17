package orchestrator

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"orchestrator/pb"
	"orchestrator/structs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type OrchestratorService struct {
	bases_path map[string]string
	mu         sync.Mutex
}

func NewOrchestratorService() *OrchestratorService {
	return &OrchestratorService{bases_path: make(map[string]string)}
}

func (s *OrchestratorService) ExecuteDeployment(deployRequest *structs.Bot, logStream chan<- *pb.LogResponse) error {
	basePath := fmt.Sprintf("./bots/%s/%s", deployRequest.BotID, deployRequest.Version)
	sourceDir := filepath.Join(basePath, "source")

	if _, err := os.Stat(sourceDir); err == nil {
		logStream <- &pb.LogResponse{Line: "Versão já existe localmente. Pulando clone.", Status: "INFO"}
		return nil
	}

	if err := os.MkdirAll(basePath, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório base: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "clone", "-b", deployRequest.Version, deployRequest.GitRepo, sourceDir)

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("falha ao iniciar git clone: %v", err)
	}

	senLogs := func(r io.Reader) {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			logStream <- &pb.LogResponse{
				Line:   scanner.Text(),
				Status: "INFO",
			}
		}
	}

	go senLogs(stdout)
	go senLogs(stderr)

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("erro durante o git clone: %v", err)
	}

	logStream <- &pb.LogResponse{Line: "Clone finalizado com sucesso!", Status: "SUCCESS"}
	return nil
}

func (s *OrchestratorService) RunBot(bot *structs.Bot, logStream chan<- *pb.LogResponse) error {

	if err := s.installRequirements(bot, logStream); err != nil {
		logStream <- &pb.LogResponse{Line: fmt.Sprintf("Erro ao instalar dependências: %v", err), Status: "ERROR"}
		return err
	}
	botPath := fmt.Sprintf("./bots/%s/%s/source", bot.BotID, bot.Version)
	venvPath := fmt.Sprintf("./bots/%s/%s/venv", bot.BotID, bot.Version)
	pythonPath := filepath.Join(venvPath, "bin", "python3")

	cmd := exec.Command(pythonPath, "main.py")
	cmd.Dir = botPath
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		logStream <- &pb.LogResponse{Line: fmt.Sprintf("Falha ao iniciar o bot: %v", err), Status: "ERROR"}
		return err
	}
	senLogs := func(r io.Reader) {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			logStream <- &pb.LogResponse{
				Line:   scanner.Text(),
				Status: "INFO",
			}
		}
	}

	go senLogs(stdout)
	go senLogs(stderr)
	if err := cmd.Wait(); err != nil {
		logStream <- &pb.LogResponse{Line: fmt.Sprintf("Erro durante a execução do bot: %v", err), Status: "ERROR"}
		return err
	}

	logStream <- &pb.LogResponse{Line: "Bot executado com sucesso!", Status: "SUCCESS"}
	return nil
}

func (s *OrchestratorService) installRequirements(bot *structs.Bot, logStream chan<- *pb.LogResponse) error {
	basePath := fmt.Sprintf("./bots/%s/%s", bot.BotID, bot.Version)
	sourceDir := filepath.Join(basePath, "source")
	venvPath := filepath.Join(basePath, "venv")
	reqFile := filepath.Join(sourceDir, "requirements.txt") // O arquivo está dentro de 'source'

	if _, err := os.Stat(reqFile); os.IsNotExist(err) {
		logStream <- &pb.LogResponse{Line: "requirements.txt não encontrado em 'source/'. Pulando.", Status: "INFO"}
		return nil
	}
	cmd := exec.Command("python3", "-m", "venv", venvPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("erro ao criar ambiente virtual: %v", err)
	}
	logStream <- &pb.LogResponse{Line: "Ambiente virtual criado com sucesso.", Status: "SUCCESS"}

	pipPath := filepath.Join(venvPath, "bin", "pip")
	installCmd := exec.Command(pipPath, "install", "-r", reqFile)
	installCmd.Dir = sourceDir
	stdout, _ := installCmd.StdoutPipe()
	stderr, _ := installCmd.StderrPipe()
	if err := installCmd.Start(); err != nil {
		return fmt.Errorf("falha ao iniciar instalação de dependências: %v", err)
	}
	senLogs := func(r io.Reader) {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			logStream <- &pb.LogResponse{
				Line:   scanner.Text(),
				Status: "INFO",
			}
		}
	}

	go senLogs(stdout)
	go senLogs(stderr)

	if err := installCmd.Wait(); err != nil {
		return fmt.Errorf("erro durante a instalação de dependências: %v", err)
	}
	logStream <- &pb.LogResponse{Line: "Dependências instaladas com sucesso.", Status: "SUCCESS"}
	return nil
}

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
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type OrchestratorService struct {
	bases_path map[string]string
	mu         sync.Mutex
}

// sanitizeUTF8 remove caracteres inválidos de UTF-8
func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	// Substitui caracteres inválidos
	var builder strings.Builder
	for _, r := range s {
		if r == utf8.RuneError {
			builder.WriteRune('?')
		} else {
			builder.WriteRune(r)
		}
	}
	return builder.String()
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

	logStream <- &pb.LogResponse{
		Line:   fmt.Sprintf("Executando: git clone -b %s %s %s", deployRequest.Version, deployRequest.GitRepo, sourceDir),
		Status: "INFO",
	}

	cmd := exec.CommandContext(ctx, "git", "clone", "-b", deployRequest.Version, deployRequest.GitRepo, sourceDir)

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("falha ao iniciar git clone: %v", err)
	}

	var wg sync.WaitGroup
	var stderrLines []string
	var stderrMu sync.Mutex

	sendStdout := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			logStream <- &pb.LogResponse{
				Line:   sanitizeUTF8(scanner.Text()),
				Status: "INFO",
			}
		}
	}

	sendStderr := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := sanitizeUTF8(scanner.Text())
			stderrMu.Lock()
			stderrLines = append(stderrLines, line)
			stderrMu.Unlock()
			logStream <- &pb.LogResponse{
				Line:   line,
				Status: "INFO",
			}
		}
	}

	wg.Add(2)
	go sendStdout(stdout)
	go sendStderr(stderr)

	cmdErr := cmd.Wait()
	wg.Wait() // Aguarda todas as goroutines terminarem de ler

	if cmdErr != nil {
		errMsg := strings.Join(stderrLines, "; ")
		return fmt.Errorf("erro durante o git clone: %v - stderr: %s", cmdErr, errMsg)
	}

	logStream <- &pb.LogResponse{Line: "Clone finalizado com sucesso!", Status: "SUCCESS"}
	return nil
}

func (s *OrchestratorService) RunBot(bot *structs.Bot, logStream chan<- *pb.LogResponse) error {

	if err := s.installRequirements(bot, logStream); err != nil {
		logStream <- &pb.LogResponse{Line: fmt.Sprintf("Erro ao instalar dependências: %v", err), Status: "ERROR"}
		return err
	}
	botPath, _ := filepath.Abs(fmt.Sprintf("./bots/%s/%s/source", bot.BotID, bot.Version))
	venvPath, _ := filepath.Abs(fmt.Sprintf("./bots/%s/%s/venv", bot.BotID, bot.Version))
	pythonPath := filepath.Join(venvPath, "Scripts", "python.exe")

	cmd := exec.Command(pythonPath, "main.py")
	cmd.Dir = botPath
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		logStream <- &pb.LogResponse{Line: fmt.Sprintf("Falha ao iniciar o bot: %v", err), Status: "ERROR"}
		return err
	}
	var wg sync.WaitGroup
	senLogs := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			logStream <- &pb.LogResponse{
				Line:   sanitizeUTF8(scanner.Text()),
				Status: "INFO",
			}
		}
	}

	wg.Add(2)
	go senLogs(stdout)
	go senLogs(stderr)

	cmdErr := cmd.Wait()
	wg.Wait() // Aguarda todas as goroutines terminarem de ler

	if cmdErr != nil {
		logStream <- &pb.LogResponse{Line: fmt.Sprintf("Erro durante a execução do bot: %v", cmdErr), Status: "ERROR"}
		return cmdErr
	}

	logStream <- &pb.LogResponse{Line: "Bot executado com sucesso!", Status: "SUCCESS"}
	return nil
}

func (s *OrchestratorService) installRequirements(bot *structs.Bot, logStream chan<- *pb.LogResponse) error {
	basePath := fmt.Sprintf("./bots/%s/%s", bot.BotID, bot.Version)
	sourceDir, _ := filepath.Abs(filepath.Join(basePath, "source"))
	venvPath, _ := filepath.Abs(filepath.Join(basePath, "venv"))
	reqFile := filepath.Join(sourceDir, "requirements.txt") // O arquivo está dentro de 'source'

	if _, err := os.Stat(reqFile); os.IsNotExist(err) {
		logStream <- &pb.LogResponse{Line: "requirements.txt não encontrado em 'source/'. Pulando.", Status: "INFO"}
		return nil
	}
	cmd := exec.Command("python", "-m", "venv", venvPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("erro ao criar ambiente virtual: %v", err)
	}
	logStream <- &pb.LogResponse{Line: "Ambiente virtual criado com sucesso.", Status: "SUCCESS"}

	pipPath := filepath.Join(venvPath, "Scripts", "pip.exe")
	installCmd := exec.Command(pipPath, "install", "-r", reqFile)
	installCmd.Dir = sourceDir
	stdout, _ := installCmd.StdoutPipe()
	stderr, _ := installCmd.StderrPipe()
	if err := installCmd.Start(); err != nil {
		return fmt.Errorf("falha ao iniciar instalação de dependências: %v", err)
	}
	var wg sync.WaitGroup
	senLogs := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			logStream <- &pb.LogResponse{
				Line:   sanitizeUTF8(scanner.Text()),
				Status: "INFO",
			}
		}
	}

	wg.Add(2)
	go senLogs(stdout)
	go senLogs(stderr)

	cmdErr := installCmd.Wait()
	wg.Wait() // Aguarda todas as goroutines terminarem de ler

	if cmdErr != nil {
		return fmt.Errorf("erro durante a instalação de dependências: %v", cmdErr)
	}
	logStream <- &pb.LogResponse{Line: "Dependências instaladas com sucesso.", Status: "SUCCESS"}
	return nil
}

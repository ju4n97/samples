package backend

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"sync"
	"time"
)

// ServerManager manages server processes.
type ServerManager struct {
	servers map[string]*ServerProcess
	mu      sync.RWMutex
}

// ServerProcess represents a server running process.
type ServerProcess struct {
	cmd     *exec.Cmd
	baseURL string
	cancel  context.CancelFunc
}

// ServerConfig defines how to start and check a backend server
type ServerConfig struct {
	Name         string            // Unique identifier, e.g. "llama", "whisper"
	BinPath      string            // Path to the binary
	Args         []string          // Arguments passed to the binary
	Port         int               // Port to bind the server
	HealthPath   string            // Health endpoint path (e.g. "/health" or "/ready")
	Env          map[string]string // Optional environment variables
	ReadyTimeout time.Duration     // How long to wait for readiness
}

// NewServerManager initializes a ServerManager
func NewServerManager() *ServerManager {
	return &ServerManager{
		servers: make(map[string]*ServerProcess),
	}
}

// StartServer starts a backend server based on a generic configuration
func (sm *ServerManager) StartServer(cfg ServerConfig) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := fmt.Sprintf("%s-%d", cfg.Name, cfg.Port)
	if _, exists := sm.servers[key]; exists {
		return nil // Already running
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, cfg.BinPath, cfg.Args...)

	// Apply environment variables if provided
	if len(cfg.Env) > 0 {
		env := make([]string, 0, len(cfg.Env))
		for k, v := range cfg.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = append(cmd.Env, env...)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start %s server: %w", cfg.Name, err)
	}

	baseURL := fmt.Sprintf("http://localhost:%d", cfg.Port)

	healthPath := cfg.HealthPath
	if healthPath == "" {
		healthPath = "/health"
	}

	timeout := cfg.ReadyTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	if err := sm.waitForServer(baseURL+healthPath, timeout); err != nil {
		cancel()
		cmd.Process.Kill()
		return fmt.Errorf("%s server did not become ready: %w", cfg.Name, err)
	}

	sm.servers[key] = &ServerProcess{
		cmd:     cmd,
		baseURL: baseURL,
		cancel:  cancel,
	}

	slog.Info("Server started", "name", cfg.Name, "port", cfg.Port)
	return nil
}

// StopServer terminates a backend server.
func (sm *ServerManager) StopServer(name string, port int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := fmt.Sprintf("%s-%d", name, port)
	if srv, exists := sm.servers[key]; exists {
		srv.cancel()
		srv.cmd.Process.Kill()
		delete(sm.servers, key)
		slog.Info("Server stopped", "name", name, "port", port)
		return nil
	} else {
		return fmt.Errorf("server %s-%d not found", name, port)
	}
}

// StopAll terminates all running servers
func (sm *ServerManager) StopAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, srv := range sm.servers {
		srv.cancel()
		srv.cmd.Process.Kill()
	}
	sm.servers = make(map[string]*ServerProcess)

	slog.Info("All servers stopped")
}

// waitForServer waits for a server to be ready.
func (sm *ServerManager) waitForServer(url string, timeout time.Duration) error {
	client := &http.Client{Timeout: 1 * time.Second}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("server failed to respond at %s within %v", url, timeout)
}

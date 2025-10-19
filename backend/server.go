package backend

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
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
	cmd    *exec.Cmd
	cancel context.CancelFunc
}

// ServerConfig defines how to start and check a backend server.
type ServerConfig struct {
	Env          map[string]string
	Name         string
	BinPath      string
	HealthPath   string
	Args         []string
	Port         int
	ReadyTimeout time.Duration
}

// NewServerManager initializes a ServerManager.
func NewServerManager() *ServerManager {
	return &ServerManager{
		servers: map[string]*ServerProcess{},
	}
}

// StartServer starts a backend server based on a generic configuration.
func (sm *ServerManager) StartServer(cfg ServerConfig) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := fmt.Sprintf("%s-%d", cfg.Name, cfg.Port)
	if _, exists := sm.servers[key]; exists {
		return nil // Already running
	}

	if info, err := os.Stat(cfg.BinPath); err != nil || info.IsDir() {
		return fmt.Errorf("manager: failed to start %s server: %w", cfg.Name, err)
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
		return fmt.Errorf("manager: failed to start %s server: %w", cfg.Name, err)
	}

	baseURL := fmt.Sprintf("http://localhost:%d", cfg.Port)

	healthPath := cfg.HealthPath
	if healthPath == "" {
		healthPath = "/health"
	}

	timeout := cfg.ReadyTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	if err := sm.waitForServer(ctx, baseURL+healthPath, timeout); err != nil {
		cancel()
		if err := cmd.Process.Kill(); err != nil {
			slog.Error("Failed to kill server process", "error", err)
		}
		return fmt.Errorf("manager: %s server did not become ready: %w", cfg.Name, err)
	}

	sm.servers[key] = &ServerProcess{
		cmd:    cmd,
		cancel: cancel,
	}

	slog.Info("Server started", "name", cfg.Name, "port", cfg.Port)
	return nil
}

// StopServer terminates a backend server.
func (sm *ServerManager) StopServer(name string, port int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := fmt.Sprintf("%s-%d", name, port)
	srv, exists := sm.servers[key]
	if !exists {
		return fmt.Errorf("server %s-%d not found", name, port)
	}

	srv.cancel()
	if err := srv.cmd.Process.Kill(); err != nil {
		slog.Error("Failed to kill server process", "error", err)
	}

	delete(sm.servers, key)
	slog.Info("Server stopped", "name", name, "port", port)
	return nil
}

// StopAll terminates all running servers.
func (sm *ServerManager) StopAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, srv := range sm.servers {
		srv.cancel()
		if err := srv.cmd.Process.Kill(); err != nil {
			slog.Error("Failed to kill server process", "error", err)
		}
	}
	sm.servers = map[string]*ServerProcess{}

	slog.Info("All servers stopped")
}

// waitForServer waits for a server to be ready.
func (sm *ServerManager) waitForServer(ctx context.Context, url string, timeout time.Duration) error {
	client := &http.Client{Timeout: 1 * time.Second}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("manager: server failed to respond at %s within %v", url, timeout)
}

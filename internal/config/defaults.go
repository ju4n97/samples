package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/ekisa-team/syn4pse/internal/envvar"
)

// DefaultHTTPPort returns the default HTTP port.
// Precedence:
// 1. SYN4PSE_SERVER_HTTP_PORT environment variable.
// 2. 8080
func DefaultHTTPPort() int {
	if p := os.Getenv(envvar.Syn4pseServerHTTPPort); p != "" {
		value, err := strconv.ParseInt(p, 10, 32)
		if err == nil {
			return int(value)
		}
	}

	return 8080
}

// DefaultGRPCPort returns the default gRPC port.
// Precedence:
// 1. SYN4PSE_SERVER_GRPC_PORT environment variable.
// 2. 50051
func DefaultGRPCPort() int {
	if p := os.Getenv(envvar.Syn4pseServerGRPCPort); p != "" {
		value, err := strconv.ParseInt(p, 10, 32)
		if err == nil {
			return int(value)
		}
	}

	return 50051
}

// DefaultConfigPath returns the default path for SYN4PSE config directory.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "syn4pse", "config")
	}

	switch runtime.GOOS {
	case "windows":
		return filepath.Join(home, "AppData", "Roaming", "syn4pse")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "syn4pse")
	default: // Linux, BSD, etc.
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "syn4pse")
		}
		return filepath.Join(home, ".config", "syn4pse")
	}
}

// DefaultModelsPath returns the default path for SYN4PSE models directory.
func DefaultModelsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "syn4pse", "models")
	}

	switch runtime.GOOS {
	case "windows":
		return filepath.Join(home, "AppData", "Local", "syn4pse", "models")
	case "darwin":
		return filepath.Join(home, "Library", "Caches", "syn4pse", "models")
	default: // Linux, BSD, etc.
		if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
			return filepath.Join(xdg, "syn4pse", "models")
		}
		return filepath.Join(home, ".cache", "syn4pse", "models")
	}
}

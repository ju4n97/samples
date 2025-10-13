package main

import (
	"context"
	"flag"
	"log/slog"
	"path"

	"github.com/ekisa-team/syn4pse/internal/config"
	"github.com/ekisa-team/syn4pse/internal/env"
	"github.com/ekisa-team/syn4pse/internal/logger"
	"github.com/ekisa-team/syn4pse/internal/model"
)

func main() {
	var (
		_              = flag.Int("http-port", config.DefaultHTTPPort(), "HTTP port to listen on")
		_              = flag.Int("grpc-port", config.DefaultGRPCPort(), "GRPC port to listen on")
		flagConfigPath = flag.String("config", path.Join(config.DefaultConfigPath(), "config.yaml"), "Path to config file")
		flagSchemaPath = flag.String("schema", path.Join(config.DefaultConfigPath(), "syn4pse.v1.schema.json"), "Path to schema file")
	)
	flag.Parse()

	environment := env.FromEnv()

	slog.SetDefault(
		logger.New(environment,
			logger.WithLogToFile(true),
			logger.WithLogFile("logs/syn4pse.log"),
		),
	)

	manager := model.NewManager()

	watcher, err := config.NewWatcher(*flagConfigPath, *flagSchemaPath, func(cfg *config.Config, err error) {
		if err != nil {
			slog.Error("Failed to reload config", "error", err)
			return
		}

		if err := manager.LoadModelsFromConfig(context.Background(), cfg); err != nil {
			slog.Error("Failed to load models from config", "error", err)
			return
		}
	})
	if err != nil {
		slog.Error("Failed to create config watcher", "error", err)
		return
	}

	cfg := watcher.Snapshot()
	if err := manager.LoadModelsFromConfig(context.Background(), cfg); err != nil {
		slog.Error("Failed to load models from config", "error", err)
		return
	}

	slog.Info("Config loaded successfully", "config", *flagConfigPath, "schema", *flagSchemaPath)

	select {}
}

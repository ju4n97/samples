package main

import (
	"flag"
	"fmt"
	"log/slog"
	"path"

	"github.com/ekisa-team/syn4pse/internal/config"
	"github.com/ekisa-team/syn4pse/internal/env"
	"github.com/ekisa-team/syn4pse/internal/logger"
)

func main() {
	var (
		_              = flag.Int("http-port", 8080, "HTTP port to listen on")
		_              = flag.Int("grpc-port", 50051, "GRPC port to listen on")
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

	watcher, err := config.NewWatcher(*flagConfigPath, *flagSchemaPath, func(cfg *config.Config, err error) {
		if err != nil {
			slog.Error("Failed to reload config", "error", err)
			return
		}

		// if err := manager.LoadModelsFromConfig(context.Background(), cfg); err != nil {
		// 	slog.Error("Failed to load models from config", "error", err)
		// 	return
		// }
	})
	if err != nil {
		slog.Error("Failed to create config watcher", "error", err)
		return
	}

	// cfg := watcher.Snapshot()
	// if err := manager.LoadModelsFromConfig(context.Background(), cfg); err != nil {
	// 	slog.Error("Failed to load models from config", "error", err)
	// 	return
	// }

	fmt.Print(watcher.Snapshot())

	slog.Info("Config loaded successfully", "config", *flagConfigPath, "schema", *flagSchemaPath)

	select {}
}

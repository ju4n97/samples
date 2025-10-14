package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path"
	"strings"

	"github.com/ekisa-team/syn4pse/internal/backend"
	"github.com/ekisa-team/syn4pse/internal/backend/llama"
	"github.com/ekisa-team/syn4pse/internal/backend/piper"
	"github.com/ekisa-team/syn4pse/internal/backend/whisper"
	"github.com/ekisa-team/syn4pse/internal/config"
	"github.com/ekisa-team/syn4pse/internal/env"
	"github.com/ekisa-team/syn4pse/internal/logger"
	"github.com/ekisa-team/syn4pse/internal/model"
)

func main() {
	ctx := context.Background()

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

		if err := manager.LoadModelsFromConfig(ctx, cfg); err != nil {
			slog.Error("Failed to load models from config", "error", err)
			return
		}
	})
	if err != nil {
		slog.Error("Failed to create config watcher", "error", err)
		return
	}

	cfg := watcher.Snapshot()
	if err := manager.LoadModelsFromConfig(ctx, cfg); err != nil {
		slog.Error("Failed to load models from config", "error", err)
		return
	}

	slog.Info("Config loaded successfully", "config", *flagConfigPath, "schema", *flagSchemaPath)

	backendRegistry := backend.NewRegistry()
	defer backendRegistry.Close()

	llamaBackend, err := llama.NewBackend("./bin/llama-cli-cuda")
	if err != nil {
		slog.Error("Failed to create Llama backend", "error", err)
		return
	}
	backendRegistry.Register(llamaBackend)

	whisperBackend, err := whisper.NewBackend("./bin/whisper-cli-cuda")
	if err != nil {
		slog.Error("Failed to create Whisper backend", "error", err)
		return
	}
	backendRegistry.Register(whisperBackend)

	piperBackend, err := piper.NewBackend("./bin/piper-cpu/piper")
	if err != nil {
		slog.Error("Failed to create Piper backend", "error", err)
		return
	}
	backendRegistry.Register(piperBackend)

	fmt.Println("=== LLM Sync ===")
	llamaSync(ctx, manager.Registry(), llamaBackend)

	fmt.Println("\n=== LLM Streaming ===")
	llamaStream(ctx, manager.Registry(), llamaBackend)

	fmt.Println("\n=== Whisper STT ===")
	whisperSTT(ctx, manager.Registry(), whisperBackend)

	fmt.Println("\n=== Piper TTS ===")
	piperTTS(ctx, manager.Registry(), piperBackend)
}

func llamaSync(ctx context.Context, r *model.Registry, b backend.Backend) {
	model, ok := r.Get("llama-cpp-qwen2.5-1.5b-instruct-q4_k_m")
	if !ok {
		log.Fatal("Qwen model not found")
	}

	req := &backend.Request{
		ModelPath: model.Path,
		Input:     strings.NewReader("What is AI? Answer briefly."),
		Parameters: map[string]any{
			"n_ctx":       1024,
			"n_predict":   50,
			"temperature": 0.7,
		},
	}

	resp, err := b.Infer(ctx, req)
	if err != nil {
		log.Fatal(err)
	}

	output, _ := io.ReadAll(resp.Output)
	fmt.Printf("Response: %s\n", output)
}

func llamaStream(ctx context.Context, r *model.Registry, b backend.Backend) {
	sb, ok := b.(backend.StreamingBackend)
	if !ok {
		log.Fatal("Backend doesn't support streaming")
	}

	model, ok := r.Get("llama-cpp-qwen2.5-1.5b-instruct-q4_k_m")
	if !ok {
		log.Fatal("Piper model not found")
	}

	req := &backend.Request{
		ModelPath: os.ExpandEnv(model.Path),
		Input:     strings.NewReader("What is AI? Answer comprehensively."),
		Parameters: map[string]any{
			"n_predict": 100,
		},
	}

	stream, err := sb.InferStream(ctx, req)
	if err != nil {
		log.Fatal(err)
	}

	for chunk := range stream {
		if chunk.Error != nil {
			log.Fatal(chunk.Error)
		}
		if chunk.Done {
			break
		}
		fmt.Print(string(chunk.Data))
	}
	fmt.Println()
}

func whisperSTT(ctx context.Context, r *model.Registry, b backend.Backend) {
	model, ok := r.Get("whisper-cpp-tiny")
	if !ok {
		log.Fatal("Whisper model not found")
	}

	audioData, err := os.ReadFile("./third_party/whisper.cpp/samples/jfk.wav")
	if err != nil {
		log.Fatal(err)
	}

	req := &backend.Request{
		ModelPath: model.Path,
		Input:     bytes.NewReader(audioData),
		Parameters: map[string]any{
			"processors": 4,
			"language":   "en",
		},
	}

	resp, err := b.Infer(ctx, req)
	if err != nil {
		log.Fatal(err)
	}

	transcription, _ := io.ReadAll(resp.Output)
	fmt.Printf("Transcription: %s\n", transcription)
}

func piperTTS(ctx context.Context, r *model.Registry, b backend.Backend) {
	model, ok := r.Get("piper-es-ar-daniela-high")
	if !ok {
		log.Fatal("Piper model not found")
	}

	req := &backend.Request{
		ModelPath: model.Path,
		Input:     strings.NewReader("Buenas tardes. ¿En qué puedo colaborarle?"),
		Parameters: map[string]any{
			"length_scale": 1.0,
		},
	}

	resp, err := b.Infer(ctx, req)
	if err != nil {
		log.Fatal(err)
	}

	audioData, _ := io.ReadAll(resp.Output)
	fmt.Printf("Generated %d bytes of audio\n", len(audioData))

	os.WriteFile("output.wav", audioData, 0644)
}

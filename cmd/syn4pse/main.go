package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/ekisa-team/syn4pse/internal/backend"
	"github.com/ekisa-team/syn4pse/internal/backend/llama"
	"github.com/ekisa-team/syn4pse/internal/backend/piper"
	"github.com/ekisa-team/syn4pse/internal/backend/whisper"
	"github.com/ekisa-team/syn4pse/internal/config"
	"github.com/ekisa-team/syn4pse/internal/env"
	"github.com/ekisa-team/syn4pse/internal/logger"
	"github.com/ekisa-team/syn4pse/internal/model"
	syn4pse_http "github.com/ekisa-team/syn4pse/internal/server/http"
	"github.com/ekisa-team/syn4pse/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog/v3"
	"golang.org/x/sync/errgroup"
)

func main() {
	ctx := context.Background()

	var (
		flagHTTPPort   = flag.Int("http-port", config.DefaultHTTPPort(), "HTTP port to listen on")
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

	backends := backend.NewRegistry()
	defer backends.Close()

	backendLlama, err := llama.NewBackend("./bin/llama-cli-cuda")
	if err != nil {
		slog.Error("Failed to create Llama backend", "error", err)
		return
	}
	backends.Register(backendLlama)

	backendWhisper, err := whisper.NewBackend("./bin/whisper-cli-cuda")
	if err != nil {
		slog.Error("Failed to create Whisper backend", "error", err)
		return
	}
	backends.Register(backendWhisper)

	backendPiper, err := piper.NewBackend("./bin/piper-cpu/piper")
	if err != nil {
		slog.Error("Failed to create Piper backend", "error", err)
		return
	}
	backends.Register(backendPiper)

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)
	httpServer := buildHTTPServer(*flagHTTPPort, backends, manager.Registry())

	g.Go(func() error {
		slog.Info("Starting HTTP server", "port", *flagHTTPPort)
		return runHTTPServer(ctx, httpServer)
	})

	if err := g.Wait(); err != nil {
		slog.Error("Error running HTTP server", "error", err)
	}

	slog.Info("Shutting down...")
}

// runHTTPServer runs the HTTP server.
func runHTTPServer(ctx context.Context, server *http.Server) error {
	go func() {
		<-ctx.Done()
		slog.Info("Shutting down HTTP server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("Error shutting down HTTP server", "error", err)
		}
	}()

	slog.Info("Server starting",
		"protocol", "HTTP",
		"address", fmt.Sprintf("http://localhost%s", server.Addr),
		"docs_v1", fmt.Sprintf("http://localhost%s/v1/docs", server.Addr),
	)

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		if ctx.Err() == nil {
			return fmt.Errorf("HTTP server error: %w", err)
		}
	}

	return nil
}

// buildHTTPServer builds the HTTP server.
func buildHTTPServer(port int, backends *backend.Registry, models *model.Registry) *http.Server {
	router := buildHTTPRouter()

	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("kivox-stt HTTP service is running."))
	})

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	router.Route("/v1", func(r chi.Router) {
		cfg := huma.DefaultConfig("SYN4PSE", "1.0.0")
		cfg.Servers = []*huma.Server{{URL: "/v1"}}
		api := humachi.New(r, cfg)

		llm := service.NewLLM(backends, models)
		stt := service.NewSTT(backends, models)
		tts := service.NewTTS(backends, models)

		syn4pse_http.NewLLMHandler(api, llm)
		syn4pse_http.NewSTTHandler(api, stt)
		syn4pse_http.NewTTSHandler(api, tts)
	})

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}
}

// buildHTTPRouter builds the HTTP router.
func buildHTTPRouter() *chi.Mux {
	router := chi.NewMux()

	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"*"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	router.Use(
		middleware.RequestID,
		middleware.RealIP,
		httplog.RequestLogger(slog.Default(), &httplog.Options{
			Level:         slog.LevelInfo,
			Schema:        httplog.SchemaECS,
			RecoverPanics: true,
			LogExtraAttrs: func(req *http.Request, reqBody string, respStatus int) []slog.Attr {
				reqID := middleware.GetReqID(req.Context())
				realIP := req.RemoteAddr

				return []slog.Attr{
					slog.String("request_id", reqID),
					slog.String("real_ip", realIP),
				}
			},
		}),
		middleware.Recoverer,
		middleware.Compress(5),
		middleware.Timeout(60*time.Second),
	)
	return router
}

package model

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/ekisa-team/syn4pse/internal/config"
	"github.com/ekisa-team/syn4pse/internal/config/source"
	"github.com/ekisa-team/syn4pse/internal/envvar"
	"github.com/ekisa-team/syn4pse/internal/xfs"
)

// Manager orchestrates model lifecycle for any model type.
type Manager struct {
	registry *Registry
	mu       sync.RWMutex // Use RWMutex for better read concurrency
}

// NewManager creates a new Manager instance for a given model type.
func NewManager() *Manager {
	return &Manager{}
}

// Registry returns the model registry.
func (m *Manager) Registry() *Registry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.registry
}

// LoadModelsFromConfig loads models from the config and updates the registry.
func (m *Manager) LoadModelsFromConfig(ctx context.Context, config *config.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.registry = NewRegistry(config)

	assignedModels := make(map[string]bool)
	for _, model := range config.Services.LLM.Models {
		assignedModels[model] = true
	}
	for _, model := range config.Services.STT.Models {
		assignedModels[model] = true
	}
	for _, model := range config.Services.TTS.Models {
		assignedModels[model] = true
	}
	for _, model := range config.Services.NLU.Models {
		assignedModels[model] = true
	}

	modelsPath := resolveModelsPath(config)
	if err := source.EnsureModelsDirectory(modelsPath); err != nil {
		return fmt.Errorf("failed to prepare models directory %s: %w", modelsPath, err)
	}

	loadedKeys := make(map[string]bool)
	for modelID := range assignedModels {
		modelConfig, ok := config.Models[modelID]
		if !ok {
			slog.Warn("Model not found in config", "model_id", modelID)
			continue
		}

		modelSource, err := modelConfig.GetSource()
		if err != nil {
			return fmt.Errorf("failed to get model source for %s: %w", modelID, err)
		}

		downloader, err := source.GetDownloader(ctx, modelSource.Type())
		if err != nil {
			return fmt.Errorf("failed to get downloader for %s: %w", modelID, err)
		}

		downloadPath, _, err := downloader.Download(ctx, &modelConfig, modelsPath)
		if err != nil {
			return fmt.Errorf("failed to download model %s into %s: %w", modelID, modelsPath, err)
		}

		instance := NewModelInstance(&modelConfig, modelID, downloadPath)
		loadedKeys[modelID] = true
		m.registry.Set(instance)
		instance.SetStatus(ModelStatusUnloaded)

		slog.Info("Model loaded into registry", "model_id", modelID, "download_path", downloadPath)
	}

	// Delete unloaded models from the registry (if any)
	current := m.registry.List()
	for _, instance := range current {
		if !loadedKeys[instance.ID] {
			m.registry.Delete(instance.ID)
			slog.Info("Model unloaded successfully", "model_entry", instance.ID)
		}
	}

	return nil
}

// resolveModelsPath returns the path to the models directory.
// Precedence:
// 1. SYN4PSE_MODELS_PATH environment variable.
// 2. ModelsDir field in the config.
// 3. Default models path.
func resolveModelsPath(cfg *config.Config) string {
	if p := os.Getenv(envvar.Syn4pseModelsPath); p != "" {
		return xfs.ExpandTilde(p)
	}
	if cfg.Storage.ModelsDir != "" {
		return xfs.ExpandTilde(cfg.Storage.ModelsDir)
	}
	return xfs.ExpandTilde(config.DefaultModelsPath())
}

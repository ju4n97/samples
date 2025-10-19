package model

import (
	"time"

	"github.com/ekisa-team/syn4pse/config"
)

// ModelType is the type of a model.
type ModelType string

const (
	// ModelTypeLLM is the type of a large language model.
	ModelTypeLLM ModelType = "llm"

	// ModelTypeNLU is the type of a natural language understanding model.
	ModelTypeNLU ModelType = "nlu"

	// ModelTypeSTT is the type of a speech-to-text model.
	ModelTypeSTT ModelType = "stt"

	// ModelTypeTTS is the type of a text-to-speech model.
	ModelTypeTTS ModelType = "tts"

	// ModelTypeEmbedding is the type of an embedding model.
	ModelTypeEmbedding ModelType = "embedding"

	// ModelTypeVision is the type of a vision model.
	ModelTypeVision ModelType = "vision"
)

// ModelStatus is the current loading status of a model.
type ModelStatus string

const (
	// ModelStatusUnloaded indicates that the model is not loaded.
	ModelStatusUnloaded ModelStatus = "unloaded"

	// ModelStatusLoading indicates that the model is being loaded.
	ModelStatusLoading ModelStatus = "loading"

	// ModelStatusLoaded indicates that the model is loaded.
	ModelStatusLoaded ModelStatus = "loaded"

	// ModelStatusFailed indicates that the model failed to load.
	ModelStatusFailed ModelStatus = "failed"

	// ModelStatusUnloading indicates that the model is being unloaded.
	ModelStatusUnloading ModelStatus = "unloading"
)

// ModelInstance represents a loaded model profile.
type ModelInstance struct {
	Config   *config.ModelConfig `json:"config"`
	LoadedAt *time.Time          `json:"loaded_at,omitempty"`
	ID       string              `json:"id"`
	Path     string              `json:"-"`
	Status   ModelStatus         `json:"status"`
	Error    string              `json:"error,omitempty"`
}

// NewModelInstance creates a new model instance.
func NewModelInstance(cfg *config.ModelConfig, id, path string) *ModelInstance {
	return &ModelInstance{
		ID:     id,
		Path:   path,
		Config: cfg,
		Status: ModelStatusUnloaded,
	}
}

// SetStatus sets the status of the model instance.
func (mi *ModelInstance) SetStatus(status ModelStatus) {
	mi.Status = status
	if status == ModelStatusLoaded {
		now := time.Now()
		mi.LoadedAt = &now
	}
}

// SetError sets the error associated with the model instance.
func (mi *ModelInstance) SetError(err error) {
	mi.Error = err.Error()
}

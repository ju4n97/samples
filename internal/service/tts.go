package service

import (
	"context"

	"github.com/ekisa-team/syn4pse/internal/backend"
	"github.com/ekisa-team/syn4pse/internal/model"
)

// TTS is a service abstraction for text-to-speech.
type TTS struct {
	backends *backend.Registry
	models   *model.Registry
}

// NewTTS creates a new TTS service.
func NewTTS(backends *backend.Registry, models *model.Registry) *TTS {
	return &TTS{
		backends: backends,
		models:   models,
	}
}

// Synthesize synthesizes speech using a text-to-speech model.
func (s *TTS) Synthesize(ctx context.Context, provider backend.BackendProvider, modelID string, req *backend.Request) (*backend.Response, error) {
	b, ok := s.backends.Get(provider)
	if !ok {
		return nil, backend.ErrBackendNotFound
	}

	m, ok := s.models.Get(modelID)
	if !ok {
		return nil, model.ErrModelNotFound
	}

	breq := &backend.Request{
		ModelPath:  m.Path,
		Input:      req.Input,
		Parameters: req.Parameters,
	}

	return b.Infer(ctx, breq)
}

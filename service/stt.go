package service

import (
	"context"

	"github.com/ekisa-team/syn4pse/backend"
	"github.com/ekisa-team/syn4pse/model"
)

// STT is a service abstraction for speech-to-text.
type STT struct {
	backends *backend.Registry
	models   *model.Registry
}

// NewSTT creates a new STT service.
func NewSTT(backends *backend.Registry, models *model.Registry) *STT {
	return &STT{
		backends: backends,
		models:   models,
	}
}

// Transcribe transcribes audio using a speech-to-text model.
func (s *STT) Transcribe(ctx context.Context, provider string, modelID string, req *backend.Request) (*backend.Response, error) {
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

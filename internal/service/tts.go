package service

import (
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

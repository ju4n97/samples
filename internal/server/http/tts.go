package http

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/ekisa-team/syn4pse/internal/service"
)

// TTSHandler handles HTTP requests for TTS.
type TTSHandler struct {
	service *service.TTS
}

// NewTTSHandler creates a new TTSHandler instance.
func NewTTSHandler(api huma.API, service *service.TTS) *TTSHandler {
	h := &TTSHandler{service: service}

	return h
}

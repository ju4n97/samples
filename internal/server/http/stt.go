package http

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/ekisa-team/syn4pse/internal/service"
)

// STTHandler handles HTTP requests for STT.
type STTHandler struct {
	service *service.STT
}

// NewSTTHandler creates a new STTHandler instance.
func NewSTTHandler(api huma.API, service *service.STT) *STTHandler {
	h := &STTHandler{service: service}

	return h
}

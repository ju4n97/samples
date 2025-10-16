package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/ekisa-team/syn4pse/backend"
	"github.com/ekisa-team/syn4pse/backend/whisper"
	"github.com/ekisa-team/syn4pse/model"
	"github.com/ekisa-team/syn4pse/service"
)

type (
	TranscribeResponseDTO struct {
		Text     string                    `json:"text"`
		Metadata *backend.ResponseMetadata `json:"metadata,omitempty"`
	}
)

type (
	TranscribeInput struct {
		RawBody huma.MultipartFormFiles[struct {
			AudioFile  huma.FormFile `form:"file" contentType:"audio/*,application/octet-stream" required:"true"`
			ModelID    string        `form:"model_id" minLength:"1" required:"true"`
			Parameters string        `form:"parameters"` // JSON-encoded optional parameters
		}]
	}

	TranscribeOutput struct {
		Body TranscribeResponseDTO
	}
)

// STTHandler handles HTTP requests for STT.
type STTHandler struct {
	service *service.STT
}

// NewSTTHandler creates a new STTHandler instance.
func NewSTTHandler(api huma.API, service *service.STT) *STTHandler {
	h := &STTHandler{service: service}

	huma.Register(api, huma.Operation{
		OperationID:   "transcribe",
		Method:        "POST",
		Path:          "/stt",
		Summary:       "Transcribe speech from an audio file",
		Tags:          []string{"stt"},
		DefaultStatus: http.StatusOK,
	}, h.handleTranscribe)

	return h
}

// handleTranscribe handles the transcribe operation.
func (h *STTHandler) handleTranscribe(ctx context.Context, input *TranscribeInput) (*TranscribeOutput, error) {
	formData := input.RawBody.Data()
	audioFile := formData.AudioFile

	if !audioFile.IsSet {
		return nil, huma.Error400BadRequest("audio file is required", nil)
	}

	audioBytes, err := io.ReadAll(audioFile)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to read audio file", err)
	}

	var parameters map[string]any
	if formData.Parameters != "" {
		if err := json.Unmarshal([]byte(formData.Parameters), &parameters); err != nil {
			return nil, huma.Error400BadRequest("invalid parameters JSON", err)
		}
	}

	provider := whisper.BackendName

	resp, err := h.service.Transcribe(
		ctx,
		provider,
		formData.ModelID,
		&backend.Request{
			Input:      bytes.NewReader(audioBytes),
			Parameters: parameters,
		},
	)
	if err != nil {
		if errors.Is(err, model.ErrModelNotFound) {
			return nil, huma.Error404NotFound("model not found", err)
		}
		return nil, huma.Error500InternalServerError("failed to transcribe", err)
	}

	transcribedBytes, err := io.ReadAll(resp.Output)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to read model output", err)
	}

	return &TranscribeOutput{
		Body: TranscribeResponseDTO{
			Text:     string(transcribedBytes),
			Metadata: resp.Metadata,
		},
	}, nil
}

package http

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/sse"

	"github.com/ju4n97/syn4pse/internal/backend"
	"github.com/ju4n97/syn4pse/internal/backend/llama"
	"github.com/ju4n97/syn4pse/internal/model"
	"github.com/ju4n97/syn4pse/internal/service"
)

type (
	// GenerateRequestDTO is the request body for the Generate operation.
	GenerateRequestDTO struct {
		Parameters map[string]any `json:"parameters,omitempty"`
		ModelID    string         `json:"model_id" minLength:"1"`
		Prompt     string         `json:"prompt" maxLength:"4096" minLength:"1"`
	}

	// GenerateResponseDTO is the response body for the Generate operation.
	GenerateResponseDTO struct {
		Metadata *backend.ResponseMetadata `json:"metadata,omitempty"`
		Text     string                    `json:"text"`
	}
)

type (
	// GenerateInput is the huma input for the Generate operation.
	GenerateInput struct {
		Body GenerateRequestDTO
	}

	// GenerateStreamInput is the huma input for the GenerateStream operation.
	GenerateStreamInput struct {
		Body GenerateRequestDTO
	}

	// GenerateOutput is the huma output for the Generate operation.
	GenerateOutput struct {
		Body GenerateResponseDTO
	}

	// StreamEvent is the huma event for the GenerateStream operation.
	StreamEvent struct {
		Text string `json:"text"`
	}
)

// LLMHandler handles HTTP requests for LLM.
type LLMHandler struct {
	service *service.LLM
}

// NewLLMHandler creates a new LLMHandler instance.
func NewLLMHandler(api huma.API, svc *service.LLM) *LLMHandler {
	h := &LLMHandler{service: svc}

	huma.Register(api, huma.Operation{
		OperationID:   "generate",
		Method:        "POST",
		Path:          "/llm",
		Summary:       "Generate text from a prompt",
		Tags:          []string{"llm"},
		DefaultStatus: http.StatusOK,
	}, h.handleGenerate)

	sse.Register(api, huma.Operation{
		OperationID: "generate-stream",
		Method:      "POST",
		Path:        "/llm/stream",
		Summary:     "Generate stream of text from a prompt (SSE)",
		Tags:        []string{"llm"},
	}, map[string]any{
		"message": StreamEvent{},
	}, h.handleGenerateStream)

	return h
}

// handleGenerate handles the generate operation.
func (h *LLMHandler) handleGenerate(ctx context.Context, input *GenerateInput) (*GenerateOutput, error) {
	provider := llama.BackendName

	resp, err := h.service.Generate(
		ctx,
		provider,
		input.Body.ModelID,
		&backend.Request{
			Input:      strings.NewReader(input.Body.Prompt),
			Parameters: input.Body.Parameters,
		},
	)
	if err != nil {
		if errors.Is(err, model.ErrModelNotFound) {
			return nil, huma.Error404NotFound("model not found", err)
		}
		return nil, huma.Error500InternalServerError("failed to generate", err)
	}

	var sb strings.Builder
	if _, err := io.Copy(&sb, resp.Output); err != nil {
		return nil, huma.Error500InternalServerError("failed to read model output", err)
	}

	return &GenerateOutput{
		Body: GenerateResponseDTO{
			Text:     sb.String(),
			Metadata: resp.Metadata,
		},
	}, nil
}

// handleGenerateStream handles the generate-stream operation.
func (h *LLMHandler) handleGenerateStream(ctx context.Context, input *GenerateStreamInput, send sse.Sender) {
	provider := llama.BackendName

	stream, err := h.service.GenerateStream(
		ctx,
		provider,
		input.Body.ModelID,
		&backend.Request{
			Input:      strings.NewReader(input.Body.Prompt),
			Parameters: input.Body.Parameters,
		},
	)
	if err != nil {
		// send an error event (typed as "error")
		_ = send.Data(struct{ Error string }{Error: err.Error()})
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-stream:
			if !ok {
				// Send an end event to indicate completion (typed as "end")
				_ = send.Data(struct{ Done string }{Done: "[DONE]"})
				return
			}

			// Send the chunk as the "message" event defined in the registration map.
			if err := send.Data(StreamEvent{Text: string(chunk.Data)}); err != nil {
				// If sending fails, abort.
				return
			}
		}
	}
}

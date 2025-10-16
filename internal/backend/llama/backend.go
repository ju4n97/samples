package llama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ekisa-team/syn4pse/internal/backend"
)

type Backend struct {
	executor *backend.Executor
}

type AssistantResponse struct {
	Response string `json:"response"`
}

func NewBackend(binPath string) (*Backend, error) {
	executor, err := backend.NewExecutor(binPath, 1*time.Minute)
	if err != nil {
		return nil, err
	}

	return &Backend{
		executor: executor,
	}, nil
}

func (b *Backend) Provider() backend.BackendProvider {
	return backend.BackendProviderLlamaCPP
}

func (b *Backend) Infer(ctx context.Context, req *backend.Request) (*backend.Response, error) {
	args := b.buildArgs(req)

	prompt, err := io.ReadAll(req.Input)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}

	args = append(args, "--prompt", string(prompt))

	stdout, stderr, err := b.executor.Execute(ctx, args, nil)
	if err != nil {
		return nil, fmt.Errorf("execution failed: %w\nstderr: %s", err, stderr)
	}

	text := b.parseOutput(string(stdout))

	return &backend.Response{
		Output: bytes.NewReader([]byte(text)),
		Metadata: &backend.ResponseMetadata{
			Provider:    b.Provider(),
			Model:       req.ModelPath,
			Timestamp:   time.Now(),
			OutputBytes: int64(len(text)),
			BackendSpecific: map[string]string{
				"stdout": string(stdout),
				"stderr": string(stderr),
				"args":   strings.Join(args, " "),
			},
		},
	}, nil
}

// InferStream executes inference and streams results as they're produced.
func (b *Backend) InferStream(ctx context.Context, req *backend.Request) (<-chan backend.StreamChunk, error) {
	args := b.buildArgs(req)

	prompt, err := io.ReadAll(req.Input)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}

	args = append(args, "--prompt", string(prompt))

	return b.executor.Stream(ctx, args, nil)
}

// buildArgs builds llama.cpp command-line arguments.
func (b *Backend) buildArgs(req *backend.Request) []string {
	args := []string{"--model", req.ModelPath}

	p := req.Parameters
	if p == nil {
		p = make(map[string]any)
	}

	// System prompt
	if v, ok := p["system_prompt"].(string); ok {
		args = append(args, "--system-prompt", v)
	}

	// Context window size (default: 4096)
	if v, ok := p["n_ctx"].(int); ok {
		args = append(args, "--ctx-size", fmt.Sprintf("%d", v))
	} else {
		args = append(args, "--ctx-size", "4096")
	}

	// Token limit (default: 128)
	if v, ok := p["n_predict"].(int); ok {
		args = append(args, "-n", fmt.Sprintf("%d", v))
	} else {
		args = append(args, "-n", "128")
	}

	// GPU layers (default: -1 = full offload)
	if v, ok := p["n_gpu_layers"].(int); ok {
		args = append(args, "-ngl", fmt.Sprintf("%d", v))
	} else {
		args = append(args, "-ngl", "-1")
	}

	// Temperature (default: 0.7)
	if v, ok := p["temperature"].(float64); ok {
		args = append(args, "--temp", fmt.Sprintf("%.2f", v))
	} else {
		args = append(args, "--temp", "0.7")
	}

	// Top-p (default: 0.9)
	if v, ok := p["top_p"].(float64); ok {
		args = append(args, "--top-p", fmt.Sprintf("%.2f", v))
	} else {
		args = append(args, "--top-p", "0.9")
	}

	// Top-k (default: 40)
	if v, ok := p["top_k"].(int); ok {
		args = append(args, "--top-k", fmt.Sprintf("%d", v))
	} else {
		args = append(args, "--top-k", "40")
	}

	// Min-p (default: 0.05)
	if v, ok := p["min_p"].(float64); ok {
		args = append(args, "--min-p", fmt.Sprintf("%.2f", v))
	} else {
		args = append(args, "--min-p", "0.05")
	}

	// Repeat penalty (default: 1.1)
	if v, ok := p["repeat_penalty"].(float64); ok {
		args = append(args, "--repeat-penalty", fmt.Sprintf("%.2f", v))
	} else {
		args = append(args, "--repeat-penalty", "1.1")
	}

	// Presence penalty (no default, model-specific)
	if v, ok := p["presence_penalty"].(float64); ok {
		args = append(args, "--presence-penalty", fmt.Sprintf("%.2f", v))
	}

	// Frequency penalty used for repeat suppression (no default, model-specific)
	if v, ok := p["frequency_penalty"].(float64); ok {
		args = append(args, "--frequency-penalty", fmt.Sprintf("%.2f", v))
	}

	// JSON Schema mode for structured output
	jsonSchema := `{"type":"object","properties":{"response":{"type":"string"}},"required":["response"]}`
	if customSchema, ok := p["json_schema"].(string); ok {
		jsonSchema = customSchema
	}
	args = append(args, "-j", jsonSchema)

	// Conversation and runtime options
	args = append(args, "--no-warmup")         // Skip model warm-up
	args = append(args, "--jinja")             // Use chat template (Jinja format)
	args = append(args, "-cnv")                // Conversation mode
	args = append(args, "-st")                 // Single-turn: exit after response
	args = append(args, "--no-display-prompt") // Don’t echo user prompt in output

	return args
}

// parseOutput extracts the response from JSON output
func (b *Backend) parseOutput(output string) string {
	// Find JSON object in output
	start := strings.Index(output, "{")
	end := strings.LastIndex(output, "}")

	if start == -1 || end == -1 || start >= end {
		return ""
	}

	jsonStr := output[start : end+1]

	fmt.Println(jsonStr)

	var result map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return ""
	}

	if response, ok := result["response"].(string); ok {
		return response
	}

	return ""
}

// Close cleans up resources. Llama does not have any resources to clean up.
func (b *Backend) Close() error {
	return nil
}

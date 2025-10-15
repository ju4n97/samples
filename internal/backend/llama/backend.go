package llama

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ekisa-team/syn4pse/internal/backend"
)

// Backend implements both backend.Backend and backend.StreamingBackend for llama.cpp.
type Backend struct {
	executor *backend.Executor
}

// NewBackend creates a new Llama backend.
func NewBackend(binPath string) (*Backend, error) {
	executor, err := backend.NewExecutor(binPath, 1*time.Minute)
	if err != nil {
		return nil, err
	}

	return &Backend{
		executor: executor,
	}, nil
}

// Provider returns the backend provider.
func (b *Backend) Provider() backend.BackendProvider {
	return backend.BackendProviderLlamaCPP
}

// Infer executes synchronous inference.
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

// InferStream executes streaming inference.
func (b *Backend) InferStream(ctx context.Context, req *backend.Request) (<-chan backend.StreamChunk, error) {
	args := b.buildArgs(req)

	prompt, err := io.ReadAll(req.Input)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}

	args = append(args, "--prompt", string(prompt))

	return b.executor.Stream(ctx, args, nil)
}

// buildArgs builds Llama command-line arguments.
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

	// Context size
	if v, ok := p["n_ctx"].(int); ok {
		args = append(args, "--ctx-size", fmt.Sprintf("%d", v))
	}

	// Token limit
	if v, ok := p["n_predict"].(int); ok {
		args = append(args, "-n", fmt.Sprintf("%d", v))
	} else {
		args = append(args, "-n", "512")
	}

	// GPU layers
	if v, ok := p["n_gpu_layers"].(int); ok {
		args = append(args, "-ngl", fmt.Sprintf("%d", v))
	}

	// Threads
	if v, ok := p["threads"].(int); ok {
		args = append(args, "-t", fmt.Sprintf("%d", v))
	}

	// Temperature
	if v, ok := p["temperature"].(float64); ok {
		args = append(args, "--temp", fmt.Sprintf("%.2f", v))
	}

	// Repeat penalty
	if v, ok := p["repeat_penalty"].(float64); ok {
		args = append(args, "--repeat-penalty", fmt.Sprintf("%.2f", v))
	} else {
		args = append(args, "--repeat-penalty", "1.1")
	}

	// Top-p
	if v, ok := p["top_p"].(float64); ok {
		args = append(args, "--top-p", fmt.Sprintf("%.2f", v))
	}

	// Top-k
	if v, ok := p["top_k"].(int); ok {
		args = append(args, "--top-k", fmt.Sprintf("%d", v))
	}

	args = append(args, "--no-warmup")
	args = append(args, "--no-display-prompt")
	args = append(args, "--simple-io")
	args = append(args, "--no-conversation")

	return args
}

// parseOutput parses Llama output.
func (b *Backend) parseOutput(output string) string {
	// Skip system/debug lines, extract actual generation
	lines := strings.Split(output, "\n")
	var result strings.Builder
	inGeneration := false

	for _, line := range lines {
		// Skip debug/info lines
		if strings.HasPrefix(line, "system_info:") ||
			strings.HasPrefix(line, "llama_") ||
			strings.HasPrefix(line, "ggml_") ||
			strings.HasPrefix(line, "print_info:") ||
			strings.HasPrefix(line, "load:") ||
			strings.HasPrefix(line, "main:") ||
			strings.HasPrefix(line, "sampler") ||
			strings.HasPrefix(line, "generate:") {
			continue
		}

		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 0 {
			inGeneration = true
		}

		if inGeneration {
			result.WriteString(line)
			result.WriteString("\n")
		}
	}

	return strings.TrimSpace(result.String())
}

// Close cleans up resources. Llama does not have any resources to clean up.
func (b *Backend) Close() error {
	return nil
}

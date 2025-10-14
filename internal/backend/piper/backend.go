package piper

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ekisa-team/syn4pse/internal/backend"
)

// Backend implements backend.Backend for Piper TTS.
type Backend struct {
	executor *backend.Executor
	tempDir  string
}

// NewBackend creates a new Piper backend.
func NewBackend(binPath string) (*Backend, error) {
	executor, err := backend.NewExecutor(binPath, 30*time.Second)
	if err != nil {
		return nil, err
	}

	tempDir := os.TempDir()
	return &Backend{
		executor: executor,
		tempDir:  tempDir,
	}, nil
}

// Name returns the backend identifier.
func (b *Backend) Name() string {
	return "piper"
}

// Infer synthesizes speech from text.
// Input: text bytes.
// Output: WAV audio bytes.
func (b *Backend) Infer(ctx context.Context, req *backend.Request) (*backend.Response, error) {
	// Piper outputs to a file, so a temp file must be used, then read it back.
	// this is a limitation of piper's CLI interface.
	outputFile := filepath.Join(b.tempDir, fmt.Sprintf("piper_%d.wav", time.Now().UnixNano()))
	defer os.Remove(outputFile)

	args := b.buildArgs(req, outputFile)

	// Piper reads text from stdin
	stdout, stderr, err := b.executor.Execute(ctx, args, req.Input)
	if err != nil {
		return nil, fmt.Errorf("execution failed: %w\nstderr: %s", err, stderr)
	}

	// Read generated audio file
	audioData, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio file: %w", err)
	}

	return &backend.Response{
		Output: bytes.NewReader(audioData),
		Metadata: &backend.ResponseMetadata{
			Backend:     b.Name(),
			Model:       req.ModelPath,
			Timestamp:   time.Now(),
			OutputBytes: int64(len(audioData)),
			BackendSpecific: map[string]any{
				"stdout": string(stdout),
				"stderr": string(stderr),
				"args":   args,
			},
		},
	}, nil
}

// buildArgs builds Piper command-line arguments.
func (b *Backend) buildArgs(req *backend.Request, outputFile string) []string {
	args := []string{
		"--model", req.ModelPath,
		"--output_file", outputFile,
	}

	p := req.Parameters
	if p == nil {
		return args
	}

	// Speaker ID
	if v, ok := p["speaker_id"].(int); ok {
		args = append(args, "--speaker", fmt.Sprintf("%d", v))
	}

	// Length scale (speed)
	if v, ok := p["length_scale"].(float64); ok {
		args = append(args, "--length_scale", fmt.Sprintf("%.2f", v))
	}

	// Noise scale
	if v, ok := p["noise_scale"].(float64); ok {
		args = append(args, "--noise_scale", fmt.Sprintf("%.2f", v))
	}

	// Noise width
	if v, ok := p["noise_w"].(float64); ok {
		args = append(args, "--noise_w", fmt.Sprintf("%.2f", v))
	}

	// Sentence silence
	if v, ok := p["sentence_silence"].(float64); ok {
		args = append(args, "--sentence_silence", fmt.Sprintf("%.2f", v))
	}

	return args
}

// Close cleans up resources. Piper does not have any resources to clean up.
func (b *Backend) Close() error {
	return nil
}

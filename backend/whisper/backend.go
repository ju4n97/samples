package whisper

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ekisa-team/syn4pse/backend"
)

const (
	BackendName = "whisper.cpp"
)

// Backend implements backend.Backend for whisper.cpp.
type Backend struct {
	executor *backend.Executor
	tempDir  string
}

// NewBackend creates a new Whisper backend.
func NewBackend(binPath string) (*Backend, error) {
	executor, err := backend.NewExecutor(binPath, 1*time.Minute)
	if err != nil {
		return nil, err
	}

	tempDir := os.TempDir()
	return &Backend{
		executor: executor,
		tempDir:  tempDir,
	}, nil
}

// Provider returns the backend provider.
func (b *Backend) Provider() string {
	return BackendName
}

// Infer transcribes audio to text.
// Input: audio bytes (WAV format for now, whisper.cpp handles it)
// Output: transcribed text as bytes
func (b *Backend) Infer(ctx context.Context, req *backend.Request) (*backend.Response, error) {
	// whisper.cpp CLI needs a file path, so we write input to temp file
	// This is a limitation of the whisper-cli interface
	audioData, err := io.ReadAll(req.Input)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio file: %w", err)
	}

	tempFile := filepath.Join(b.tempDir, fmt.Sprintf("whisper_%d.wav", time.Now().UnixNano()))
	if err := os.WriteFile(tempFile, audioData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write audio file: %w", err)
	}
	defer os.Remove(tempFile)

	args := b.buildArgs(req, tempFile)

	stdout, stderr, err := b.executor.Execute(ctx, args, nil)
	if err != nil {
		return nil, fmt.Errorf("execution failed: %w\nstderr: %s", err, stderr)
	}

	text := b.parseTranscription(string(stdout))

	return &backend.Response{
		Output: bytes.NewReader([]byte(text)),
		Metadata: &backend.ResponseMetadata{
			Provider:        b.Provider(),
			Model:           req.ModelPath,
			Timestamp:       time.Now(),
			OutputSizeBytes: int64(len(text)),
			BackendSpecific: map[string]any{
				"stdout": string(stdout),
				"stderr": string(stderr),
				"args":   strings.Join(args, " "),
			},
		},
	}, nil
}

// buildArgs builds Whisper command-line arguments.
func (b *Backend) buildArgs(req *backend.Request, audioFile string) []string {
	args := []string{
		"-m", req.ModelPath,
		"-f", audioFile,
	}

	p := req.Parameters
	if p == nil {
		return args
	}

	// Processors
	if v, ok := p["processors"].(int); ok {
		args = append(args, "--processors", fmt.Sprintf("%d", v))
	}

	// Threads
	if v, ok := p["threads"].(int); ok {
		args = append(args, "-t", fmt.Sprintf("%d", v))
	}

	// Language
	if v, ok := p["language"].(string); ok {
		args = append(args, "-l", v)
	}

	// No timestamps
	if v, ok := p["no_timestamps"].(bool); ok && v {
		args = append(args, "-nt")
	}

	// Translate to English
	if v, ok := p["translate"].(bool); ok && v {
		args = append(args, "-tr")
	}

	return args
}

// parseTranscription parses Whisper's output and returns the transcribed text.
func (b *Backend) parseTranscription(output string) string {
	// Format: [00:00:00.000 --> 00:00:02.000]   Transcribed text
	var result strings.Builder
	lines := strings.Split(output, "\n")

	timestampRegex := regexp.MustCompile(`^\[(\d{2}):(\d{2}):(\d{2})\.(\d{3}) --> (\d{2}):(\d{2}):(\d{2})\.(\d{3})\]`)

	for _, line := range lines {
		if timestampRegex.MatchString(line) {
			parts := strings.SplitN(line, "]", 2)
			if len(parts) == 2 {
				text := strings.TrimSpace(parts[1])
				if text != "" {
					if result.Len() > 0 {
						result.WriteString(" ")
					}
					result.WriteString(text)
				}
			}
		}
	}

	return strings.TrimSpace(result.String())
}

// Close cleans up resources. Whisper does not have any resources to clean up.
func (b *Backend) Close() error {
	return nil
}

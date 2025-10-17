package whisper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/ekisa-team/syn4pse/backend"
	"github.com/ekisa-team/syn4pse/mapsafe"
)

const (
	BackendName = "whisper.cpp"
	BackendPort = 8082
)

// Backend implements backend.Backend for whisper.cpp.
type Backend struct {
	binPath       string
	serverManager *backend.ServerManager
	client        *http.Client
	port          int
}

// TranscriptionRequest represents a request to the whisper-server API.
type TranscriptionRequest struct {
	Language     string  `json:"language,omitempty"`
	Temperature  float64 `json:"temperature,omitempty"`
	BeamSize     int     `json:"beam_size,omitempty"`
	BestOf       int     `json:"best_of,omitempty"`
	Translate    bool    `json:"translate,omitempty"`
	NoTimestamps bool    `json:"no_timestamps,omitempty"`
	Prompt       string  `json:"prompt,omitempty"`
}

// TranscriptionResponse represents a response from the whisper-server API.
type TranscriptionResponse struct {
	Task                        string              `json:"task,omitempty"`
	Language                    string              `json:"language,omitempty"`
	Duration                    float64             `json:"durationm,omitempty"`
	Text                        string              `json:"text,omitempty"`
	Segments                    []TranscriptSegment `json:"segments,omitempty"`
	DetectedLanguage            string              `json:"detected_language,omitempty"`
	DetectedLanguageProbability float64             `json:"detected_language_probability,omitempty"`
	LanguageProbabilities       map[string]float64  `json:"language_probabilities,omitempty"`
}

// TranscriptSegment represents a single segment in the transcription.
type TranscriptSegment struct {
	ID           int                        `json:"id"`
	Text         string                     `json:"text"`
	Start        float64                    `json:"start"`
	End          float64                    `json:"end"`
	Tokens       []int                      `json:"tokens,omitempty"`
	Words        []TranscriptionSegmentWord `json:"words,omitempty"`
	Temperature  float64                    `json:"temperature,omitempty"`
	AvgLogprob   float64                    `json:"avg_logprob,omitempty"`
	NoSpeechProb float64                    `json:"no_speech_prob,omitempty"`
}

// TranscriptionSegmentWord represents a word in the transcription segment.
type TranscriptionSegmentWord struct {
	Word        string  `json:"word"`
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	DTW         float64 `json:"t_dtw"`
	Probability float64 `json:"probability"`
}

// NewBackend creates a new Backend instance.
func NewBackend(binPath string, serverManager *backend.ServerManager) (*Backend, error) {
	return &Backend{
		binPath:       binPath,
		serverManager: serverManager,
		client: &http.Client{
			Timeout: 5 * time.Minute, // Transcription can take longer
		},
		port: BackendPort,
	}, nil
}

// Close implements backend.Backend.
func (b *Backend) Close() error {
	return b.serverManager.StopServer(BackendName, b.port)
}

// Provider implements backend.Backend.
func (b *Backend) Provider() string {
	return BackendName
}

// Infer implements backend.Backend.
func (b *Backend) Infer(ctx context.Context, req *backend.Request) (*backend.Response, error) {
	// Build server arguments
	args := []string{
		"--model", req.ModelPath,
		"--port", fmt.Sprintf("%d", b.port),
		"--host", "127.0.0.1",
	}

	if err := b.serverManager.StartServer(backend.ServerConfig{
		Name:       BackendName,
		BinPath:    b.binPath,
		Args:       args,
		Port:       b.port,
		HealthPath: "/", // Whisper server doesn't have a dedicated health endpoint
	}); err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	audioData, err := io.ReadAll(req.Input)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio input: %w", err)
	}

	// Create multipart form data
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Add audio file
	part, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(audioData); err != nil {
		return nil, fmt.Errorf("failed to write audio data: %w", err)
	}

	// Add parameters to form
	transcriptionReq := b.buildTranscriptionRequest(req)
	if err := b.addTranscriptionParams(writer, transcriptionReq); err != nil {
		return nil, fmt.Errorf("failed to add parameters: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx,
		"POST",
		fmt.Sprintf("http://localhost:%d/inference", b.port),
		&requestBody,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	start := time.Now()

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	elapsed := time.Since(start).Seconds()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return nil, fmt.Errorf("request failed with status code %d: %s", resp.StatusCode, body)
	}

	var transcriptionResp TranscriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&transcriptionResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &backend.Response{
		Output: bytes.NewReader([]byte(transcriptionResp.Text)),
		Metadata: &backend.ResponseMetadata{
			Provider:        b.Provider(),
			Model:           req.ModelPath,
			Timestamp:       time.Now(),
			DurationSeconds: elapsed,
			OutputSizeBytes: int64(len(transcriptionResp.Text)),
			BackendSpecific: map[string]any{
				"response": transcriptionResp,
			},
		},
	}, nil
}

// buildTranscriptionRequest builds a TranscriptionRequest from a backend.Request.
func (b *Backend) buildTranscriptionRequest(req *backend.Request) *TranscriptionRequest {
	p := req.Parameters
	if p == nil {
		p = make(map[string]any)
	}

	return &TranscriptionRequest{
		Language:     mapsafe.Get(p, "language", ""),
		Temperature:  mapsafe.Get(p, "temperature", 0.0),
		Translate:    mapsafe.Get(p, "translate", false),
		NoTimestamps: mapsafe.Get(p, "no_timestamps", false),
		Prompt:       mapsafe.Get(p, "prompt", ""),
		BeamSize:     mapsafe.Get(p, "beam_size", -1),
		BestOf:       mapsafe.Get(p, "best_of", 2),
	}
}

// addTranscriptionParams adds transcription parameters to the multipart writer.
func (b *Backend) addTranscriptionParams(w *multipart.Writer, req *TranscriptionRequest) error {
	params := map[string]string{
		"language":        req.Language,
		"response_format": "verbose_json",
		"temperature":     fmt.Sprintf("%.2f", req.Temperature),
		"translate":       fmt.Sprintf("%t", req.Translate),
		"no_timestamps":   fmt.Sprintf("%t", req.NoTimestamps),
	}

	if req.BeamSize >= 0 {
		params["beam_size"] = fmt.Sprintf("%d", req.BeamSize)
	}

	if req.BestOf > 0 {
		params["best_of"] = fmt.Sprintf("%d", req.BestOf)
	}

	if req.Prompt != "" {
		params["prompt"] = req.Prompt
	}

	for key, value := range params {
		if err := w.WriteField(key, value); err != nil {
			return fmt.Errorf("failed to write field %s: %w", key, err)
		}
	}

	return nil
}

package llama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ekisa-team/syn4pse/backend"
)

const (
	BackendName = "llama.cpp"
	BackendPort = 8081
)

// Backend implements backend.Backend for llama.cpp.
type Backend struct {
	binPath       string
	serverManager *backend.ServerManager
	client        *http.Client
	port          int
}

// ChatMessage represents a single message in a chat conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest is a request to the llama-server API.
type ChatCompletionRequest struct {
	Messages         []ChatMessage `json:"messages"`
	Temperature      float64       `json:"temperature,omitempty"`
	TopK             int           `json:"top_k,omitempty"`
	TopP             float64       `json:"top_p,omitempty"`
	MinP             float64       `json:"min_p,omitempty"`
	NPredict         int           `json:"n_predict,omitempty"`
	RepeatPenalty    float64       `json:"repeat_penalty,omitempty"`
	PresencePenalty  float64       `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64       `json:"frequency_penalty,omitempty"`
}

// ChatCompletionResponse is a response from the llama-server API.
type ChatCompletionResponse struct {
	ID                string         `json:"id,omitempty"`
	Object            string         `json:"object,omitempty"`
	Created           int64          `json:"created,omitempty"`
	Model             string         `json:"model,omitempty"`
	SystemFingerprint string         `json:"system_fingerprint,omitempty"`
	Choices           []Choice       `json:"choices"`
	Usage             Usage          `json:"usage"`
	Timings           map[string]any `json:"timings,omitempty"`
}

// Choice represents a single choice in a response
type Choice struct {
	Index        int     `json:"index"`
	FinishReason string  `json:"finish_reason"`
	Message      Message `json:"message"`
}

// Message represents a single message in a chat conversation
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Usage represents the usage information of a response
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// NewBackend creates a new Backend instance.
func NewBackend(binPath string, serverManager *backend.ServerManager) (*Backend, error) {
	return &Backend{
		binPath:       binPath,
		serverManager: serverManager,
		client: &http.Client{
			Timeout: 2 * time.Minute,
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
	if err := b.serverManager.StartServer(backend.ServerConfig{
		Name:       BackendName,
		BinPath:    b.binPath,
		Args:       []string{"--model", req.ModelPath, "--port", fmt.Sprintf("%d", b.port)},
		Port:       b.port,
		HealthPath: "/health",
	}); err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	prompt, err := io.ReadAll(req.Input)
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}

	completionReq := b.buildChatCompletionRequest(req, string(prompt))

	jsonData, err := json.Marshal(completionReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx,
		"POST",
		fmt.Sprintf("http://localhost:%d/chat/completions", b.port),
		bytes.NewReader(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

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

	var completionResp ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completionResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	content := ""
	if len(completionResp.Choices) > 0 {
		content = completionResp.Choices[0].Message.Content
	}

	return &backend.Response{
		Output: bytes.NewReader([]byte(content)),
		Metadata: &backend.ResponseMetadata{
			Provider:        b.Provider(),
			Model:           req.ModelPath,
			Timestamp:       time.Now(),
			DurationSeconds: elapsed,
			OutputSizeBytes: int64(len(content)),
			BackendSpecific: map[string]any{
				"response": completionResp,
			},
		},
	}, nil
}

// buildChatCompletionRequest builds a ChatCompletionRequest from a backend.Request.
func (s *Backend) buildChatCompletionRequest(req *backend.Request, prompt string) *ChatCompletionRequest {
	p := req.Parameters
	if p == nil {
		p = make(map[string]any)
	}

	messages := []ChatMessage{
		{Role: "user", Content: prompt},
	}

	if sysPrompt, ok := p["system_prompt"].(string); ok && sysPrompt != "" {
		messages = append([]ChatMessage{{Role: "system", Content: sysPrompt}}, messages...)
	}

	return &ChatCompletionRequest{
		Messages:         messages,
		NPredict:         getInt(p, "n_predict", 128),
		Temperature:      getFloat(p, "temperature", 0.7),
		TopK:             getInt(p, "top_k", 40),
		TopP:             getFloat(p, "top_p", 0.9),
		MinP:             getFloat(p, "min_p", 0.05),
		RepeatPenalty:    getFloat(p, "repeat_penalty", 1.1),
		PresencePenalty:  getFloat(p, "presence_penalty", 0.0),
		FrequencyPenalty: getFloat(p, "frequency_penalty", 0.0),
	}
}

// getInt safely retrieves an int from a map[string]any.
func getInt(m map[string]any, key string, defaultValue int) int {
	if val, ok := m[key]; ok {
		// JSON numbers are decoded as float64 sometimes
		if f, isFloat := val.(float64); isFloat {
			return int(f)
		}
		if i, isInt := val.(int); isInt {
			return i
		}
	}
	return defaultValue
}

// getFloat safely retrieves a float64 from a map[string]any.
func getFloat(m map[string]any, key string, defaultValue float64) float64 {
	if val, ok := m[key]; ok {
		if f, isFloat := val.(float64); isFloat {
			return f
		}
	}
	return defaultValue
}

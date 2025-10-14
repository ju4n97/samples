package backend

import (
	"context"
	"io"
	"time"
)

// BackendProvider is a string identifier for a backend provider.
type BackendProvider string

const (
	BackendProviderLlamaCPP   BackendProvider = "llama.cpp"
	BackendProviderPiper      BackendProvider = "piper"
	BackendProviderWhisperCPP BackendProvider = "whisper.cpp"
)

// Backend defines the core interface for all inference backends.
type Backend interface {
	// Name returns the backend identifier.
	Provider() BackendProvider

	// Infer executes inference and returns complete result.
	Infer(ctx context.Context, req *Request) (*Response, error)

	// Close cleans up resources.
	Close() error
}

// StreamingBackend is an optional interface for backends that support streaming.
type StreamingBackend interface {
	Backend

	// InferStream executes inference and streams results as they're produced.
	InferStream(ctx context.Context, req *Request) (<-chan StreamChunk, error)
}

// Request encapsulates all parameters for an inference call.
type Request struct {
	// ModelPath is the path to the model file.
	ModelPath string

	// Input is the raw input data (text, audio bytes, image bytes, etc.).
	Input io.Reader

	// Parameters contains backend-specific inference parameters.
	Parameters map[string]any
}

// Response contains the result of an inference operation.
type Response struct {
	// Output is the raw output data.
	Output io.Reader

	// Metadata contains backend-specific information.
	Metadata *ResponseMetadata
}

// ResponseMetadata contains metadata about the response.
type ResponseMetadata struct {
	Provider        BackendProvider `json:"provider"`
	Model           string          `json:"model"`
	Timestamp       time.Time       `json:"timestamp"`
	OutputBytes     int64           `json:"output_bytes"`
	BackendSpecific map[string]any  `json:"backend_specific"`
}

// StreamChunk represents a single chunk in a streaming response.
type StreamChunk struct {
	// Data is the chunk content.
	Data []byte

	// Done indicates if this is the final chunk.
	Done bool

	// Error if something went wrong.
	Error error
}

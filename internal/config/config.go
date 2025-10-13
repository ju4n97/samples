package config

import (
	"errors"
)

// SourceType represents the type of model source.
type SourceType string

const (
	SourceTypeHuggingFace SourceType = "huggingface"
	// TODO: implement SourceTypeLocal
)

// Config holds the main configuration for the application.
type Config struct {
	Version  string                 `yaml:"version" json:"version"`
	Storage  StorageConfig          `yaml:"storage,omitempty" json:"storage,omitempty"`
	Models   map[string]ModelConfig `yaml:"models" json:"models"`
	Services ServicesConfig         `yaml:"services" json:"services"`
}

// StorageConfig holds configuration for caching and auto-download.
type StorageConfig struct {
	ModelsDir string `yaml:"models_dir,omitempty" json:"models_dir,omitempty"`
}

// ModelConfig holds configuration for a specific model.
type ModelConfig struct {
	Type    string       `yaml:"type" json:"type"`       // "stt", "tts", "llm", "nlu"
	Backend string       `yaml:"backend" json:"backend"` // e.g., "llama.cpp"
	Source  SourceConfig `yaml:"source" json:"source"`   // e.g., {"huggingface": {"repo": "Systran/faster-whisper-tiny"}}
	Order   int          `yaml:"order" json:"order"`     // Lower = higher priority
	Tags    []string     `yaml:"tags" json:"tags"`       // e.g., ["multilingual", "streaming"]
}

// SourceConfig wraps optional sources (only one should be set).
// This struct replaces the incorrectly named "ModelSourceConfig".
type SourceConfig struct {
	HuggingFace *HuggingFaceSource `yaml:"huggingface,omitempty" json:"huggingface,omitempty"`
	// Local       *LocalSource       `yaml:"local,omitempty" json:"local,omitempty"`
	// S3          *S3Source          `yaml:"s3,omitempty" json:"s3,omitempty"`
}

// ServicesConfig holds configuration for all services.
type ServicesConfig struct {
	LLM ServicesConfigAssignment `yaml:"llm" json:"llm"`
	NLU ServicesConfigAssignment `yaml:"nlu" json:"nlu"`
	STT ServicesConfigAssignment `yaml:"stt" json:"stt"`
	TTS ServicesConfigAssignment `yaml:"tts" json:"tts"`
}

// ServicesConfigAssignment holds model assignments for a service.
type ServicesConfigAssignment struct {
	Models []string `yaml:"models" json:"models"` // List of model IDs
}

// -------------------------
// Source definitions
// -------------------------

// ModelSource represents a source for a model.
type ModelSource interface {
	Type() SourceType
}

// HuggingFaceSource represents a Hugging Face model repository source.
type HuggingFaceSource struct {
	Repo          string   `yaml:"repo" json:"repo"`
	Revision      string   `yaml:"revision,omitempty" json:"revision,omitempty"`
	RepoType      string   `yaml:"repo_type,omitempty" json:"repo_type,omitempty"`
	Include       []string `yaml:"include,omitempty" json:"include,omitempty"`
	Exclude       []string `yaml:"exclude,omitempty" json:"exclude,omitempty"`
	ForceDownload bool     `yaml:"force_download,omitempty" json:"force_download,omitempty"`
	Token         string   `yaml:"token,omitempty" json:"token,omitempty"`
	MaxWorkers    int      `yaml:"max_workers,omitempty" json:"max_workers,omitempty"`
}

// Type returns the Hugging Face source type.
func (h HuggingFaceSource) Type() SourceType {
	return SourceTypeHuggingFace
}

// GetSource returns the active source for the model.
func (m *ModelConfig) GetSource() (ModelSource, error) {
	if m.Source.HuggingFace != nil {
		return *m.Source.HuggingFace, nil
	}
	// if m.Source.Local != nil {
	//     return *m.Source.Local, nil
	// }

	return nil, errors.New("no source configured for model")
}

// SetHuggingFaceSource sets the Hugging Face source.
func (m *ModelConfig) SetHuggingFaceSource(source HuggingFaceSource) {
	m.Source.HuggingFace = &source
	// m.Source.Local = nil
}

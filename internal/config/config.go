package config

import (
	"errors"
)

// SourceType represents the type of model source.
type SourceType string

const (
	// SourceTypeHuggingFace represents a Hugging Face model repository source.
	SourceTypeHuggingFace SourceType = "huggingface"
)

// Config holds the main configuration for the application.
type Config struct {
	Version  string                 `json:"version"           yaml:"version"`
	Storage  StorageConfig          `json:"storage,omitempty" yaml:"storage,omitempty"`
	Models   map[string]ModelConfig `json:"models"            yaml:"models"`
	Services ServicesConfig         `json:"services"          yaml:"services"`
}

// StorageConfig holds configuration for caching and auto-download.
type StorageConfig struct {
	ModelsDir string `json:"models_dir,omitempty" yaml:"models_dir,omitempty"`
}

// ModelConfig holds configuration for a specific model.
type ModelConfig struct {
	Source  SourceConfig `json:"source"  yaml:"source"`
	Type    string       `json:"type"    yaml:"type"`
	Backend string       `json:"backend" yaml:"backend"`
	Tags    []string     `json:"tags"    yaml:"tags"`
	Order   int          `json:"order"   yaml:"order"`
}

// SourceConfig wraps optional sources (only one should be set).
// This struct replaces the incorrectly named "ModelSourceConfig".
type SourceConfig struct {
	HuggingFace *HuggingFaceSource `json:"huggingface,omitempty" yaml:"huggingface,omitempty"`
	// Local       *LocalSource       `yaml:"local,omitempty" json:"local,omitempty"`
	// S3          *S3Source          `yaml:"s3,omitempty" json:"s3,omitempty"`
}

// ServicesConfig holds configuration for all services.
type ServicesConfig struct {
	LLM ServicesConfigAssignment `json:"llm" yaml:"llm"`
	NLU ServicesConfigAssignment `json:"nlu" yaml:"nlu"`
	STT ServicesConfigAssignment `json:"stt" yaml:"stt"`
	TTS ServicesConfigAssignment `json:"tts" yaml:"tts"`
}

// ServicesConfigAssignment holds model assignments for a service.
type ServicesConfigAssignment struct {
	Models []string `json:"models" yaml:"models"` // List of model IDs
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
	Repo          string   `json:"repo"                     yaml:"repo"`
	Revision      string   `json:"revision,omitempty"       yaml:"revision,omitempty"`
	RepoType      string   `json:"repo_type,omitempty"      yaml:"repo_type,omitempty"`
	Token         string   `json:"token,omitempty"          yaml:"token,omitempty"`
	Include       []string `json:"include,omitempty"        yaml:"include,omitempty"`
	Exclude       []string `json:"exclude,omitempty"        yaml:"exclude,omitempty"`
	MaxWorkers    int      `json:"max_workers,omitempty"    yaml:"max_workers,omitempty"`
	ForceDownload bool     `json:"force_download,omitempty" yaml:"force_download,omitempty"`
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

	return nil, errors.New("no source configured for model")
}

// SetHuggingFaceSource sets the Hugging Face source.
func (m *ModelConfig) SetHuggingFaceSource(source HuggingFaceSource) {
	m.Source.HuggingFace = &source
}

package config

import (
	"fmt"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"go.yaml.in/yaml/v3"
)

// LoadAndValidate loads and validates the configuration.
func LoadAndValidate(path, schemaPath string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("manager: failed to read config: %w", err)
	}

	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	schema, err := jsonschema.Compile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("manager: failed to compile schema: %w", err)
	}

	if err := schema.Validate(raw); err != nil {
		return nil, fmt.Errorf("manager: config validation failed: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("manager: failed to unmarshal into Config struct: %w", err)
	}

	return &config, nil
}

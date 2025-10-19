package model

import "errors"

// Error definitions for the model package.
var (
	ErrModelNotFound = errors.New("model not found in registry")
)

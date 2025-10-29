package model

import "errors"

// Error definitions for the model package.
var (
	ErrNotFound = errors.New("model not found in registry")
)

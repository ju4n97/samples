package backend

import "errors"

// Error definitions for the backend package.
var (
	ErrNotFound          = errors.New("backend not found in registry")
	ErrAlreadyRegistered = errors.New("backend is already registered in the registry")
	ErrNotStreamable     = errors.New("backend is not streamable")
)

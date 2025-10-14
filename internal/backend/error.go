package backend

import "errors"

var (
	ErrBackendNotFound          = errors.New("backend not found in registry")
	ErrBackendAlreadyRegistered = errors.New("backend is already registered in the registry")
	ErrBackendNotStreamable     = errors.New("backend is not streamable")
)

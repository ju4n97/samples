package backend

// ModelLocator is an optional interface for backends that can locate
// the actual model file to load or execute.
type ModelLocator interface {
	// ResolveModelPath resolves the real model path inside the base downloaded directory.
	ResolveModelPath(basePath string) (string, error)
}

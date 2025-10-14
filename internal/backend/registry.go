package backend

import "sync"

// Registry manages backend instances.
type Registry struct {
	backends map[BackendProvider]Backend
	mu       sync.RWMutex
}

// NewRegistry creates a new backend registry.
func NewRegistry() *Registry {
	return &Registry{
		backends: make(map[BackendProvider]Backend),
	}
}

// Register adds a backend to the registry.
func (r *Registry) Register(b Backend) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.backends[b.Provider()]; ok {
		return ErrBackendAlreadyRegistered
	}

	r.backends[b.Provider()] = b

	return nil
}

// Get retrieves a backend by provider.
func (r *Registry) Get(provider BackendProvider) (Backend, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	b, ok := r.backends[provider]
	return b, ok
}

// GetStreaming retrieves a backend that supports streaming.
func (r *Registry) GetStreaming(provider BackendProvider) (StreamingBackend, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	b, ok := r.backends[provider]
	if !ok {
		return nil, false
	}

	sb, ok := b.(StreamingBackend)
	return sb, ok
}

// Close closes all registered backends.
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, b := range r.backends {
		if err := b.Close(); err != nil {
			return err
		}
	}

	return nil
}

package backend

import "sync"

// Registry manages backend instances.
type Registry struct {
	backends map[string]Backend
	mu       sync.RWMutex
}

// NewRegistry creates a new backend registry.
func NewRegistry() *Registry {
	return &Registry{
		backends: make(map[string]Backend),
	}
}

// Register adds a backend to the registry.
func (r *Registry) Register(b Backend) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.backends[b.Name()] = b
}

// Get retrieves a backend by name.
func (r *Registry) Get(name string) (Backend, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	b, ok := r.backends[name]
	return b, ok
}

// GetStreaming retrieves a backend that supports streaming.
func (r *Registry) GetStreaming(name string) (StreamingBackend, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	b, ok := r.backends[name]
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

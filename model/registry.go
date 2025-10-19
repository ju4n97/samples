package model

import (
	"sync"
)

// Registry stores loaded model instances.
type Registry struct {
	models map[string]*Instance
	mu     sync.RWMutex
}

// NewRegistry creates a new model registry.
func NewRegistry() *Registry {
	return &Registry{
		models: map[string]*Instance{},
	}
}

// Set adds a model instance to the registry.
func (r *Registry) Set(instance *Instance) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.models[instance.ID] = instance
}

// Get returns the model instance with the given ID.
func (r *Registry) Get(id string) (*Instance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instance, ok := r.models[id]
	return instance, ok
}

// List returns all model instances.
func (r *Registry) List() []*Instance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances := make([]*Instance, 0, len(r.models))
	for _, instance := range r.models {
		instances = append(instances, instance)
	}

	return instances
}

// Delete deletes the model instance with the given ID.
func (r *Registry) Delete(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.models, id)
}

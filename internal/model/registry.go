package model

import (
	"sync"

	"github.com/ekisa-team/syn4pse/internal/config"
)

// Registry stores loaded model instances.
type Registry struct {
	models map[string]*ModelInstance
	config *config.Config
	mu     sync.RWMutex
}

// NewRegistry creates a new model registry.
func NewRegistry(config *config.Config) *Registry {
	return &Registry{
		models: make(map[string]*ModelInstance),
		config: config,
	}
}

// Set adds a model instance to the registry.
func (r *Registry) Set(instance *ModelInstance) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.models[instance.ID] = instance
}

// Get returns the model instance with the given ID.
func (r *Registry) Get(id string) (*ModelInstance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instance, ok := r.models[id]
	return instance, ok
}

// List returns all model instances.
func (r *Registry) List() []*ModelInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances := make([]*ModelInstance, 0, len(r.models))
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

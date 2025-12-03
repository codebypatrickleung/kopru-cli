// Package workflow provides workflow registration and management.
package workflow

import (
	"fmt"
	"sync"
)

// Registry manages workflow handlers for different migration paths.
type Registry struct {
	handlers map[string]Handler
	mu       sync.RWMutex
}

// NewRegistry creates a new workflow registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]Handler),
	}
}

// Register registers a workflow handler.
// The handler is registered using a key format: "source-to-target" (e.g., "azure-to-oci").
func (r *Registry) Register(handler Handler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s-to-%s", handler.SourcePlatform(), handler.TargetPlatform())
	if _, exists := r.handlers[key]; exists {
		return fmt.Errorf("workflow handler for %s already registered", key)
	}

	r.handlers[key] = handler
	return nil
}

// Get retrieves a workflow handler for the given source and target platforms.
func (r *Registry) Get(sourcePlatform, targetPlatform string) (Handler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := fmt.Sprintf("%s-to-%s", sourcePlatform, targetPlatform)
	handler, exists := r.handlers[key]
	if !exists {
		return nil, fmt.Errorf("no workflow handler registered for %s", key)
	}

	return handler, nil
}

// List returns all registered workflow handlers.
func (r *Registry) List() []Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handlers := make([]Handler, 0, len(r.handlers))
	for _, handler := range r.handlers {
		handlers = append(handlers, handler)
	}
	return handlers
}

// DefaultRegistry is the global workflow registry.
var DefaultRegistry = NewRegistry()

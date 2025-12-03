// Package os provides registry for OS adjustment modules.
package os

import (
	"fmt"
	"sync"
)

const (
	// CustomOSType is the special OS type identifier for custom script-based adjustments
	CustomOSType = "CUSTOM"
)

// ConfiguratorRegistry manages OS configurators for different platform and OS combinations.
type ConfiguratorRegistry struct {
	configurators map[string]Configurator
	mu            sync.RWMutex
}

// NewConfiguratorRegistry creates a new configurator registry.
func NewConfiguratorRegistry() *ConfiguratorRegistry {
	return &ConfiguratorRegistry{
		configurators: make(map[string]Configurator),
	}
}

// Register registers an OS configurator.
// The configurator is registered using a key format: "source-to-target-os" (e.g., "azure-to-oci-ubuntu").
func (r *ConfiguratorRegistry) Register(configurator Configurator) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := makeConfiguratorKey(configurator.SourcePlatform(), configurator.TargetPlatform(), configurator.OSType())
	if _, exists := r.configurators[key]; exists {
		return fmt.Errorf("configurator for %s already registered", key)
	}

	r.configurators[key] = configurator
	return nil
}

// Get retrieves an OS configurator for the given source platform, target platform, and OS type.
func (r *ConfiguratorRegistry) Get(sourcePlatform, targetPlatform, osType string) (Configurator, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := makeConfiguratorKey(sourcePlatform, targetPlatform, osType)
	configurator, exists := r.configurators[key]
	if !exists {
		return nil, fmt.Errorf("no configurator registered for %s", key)
	}

	return configurator, nil
}

// List returns all registered configurators.
func (r *ConfiguratorRegistry) List() []Configurator {
	r.mu.RLock()
	defer r.mu.RUnlock()

	configurators := make([]Configurator, 0, len(r.configurators))
	for _, configurator := range r.configurators {
		configurators = append(configurators, configurator)
	}
	return configurators
}

// makeConfiguratorKey creates a registry key from platform and OS type.
func makeConfiguratorKey(sourcePlatform, targetPlatform, osType string) string {
	return fmt.Sprintf("%s-to-%s-%s", sourcePlatform, targetPlatform, osType)
}

// DefaultConfiguratorRegistry is the global OS configurator registry.
var DefaultConfiguratorRegistry = NewConfiguratorRegistry()

// GetConfigurator is a convenience function to get an configurator from the default registry.
// If osType is CUSTOM, it returns a custom configurator instead of looking in the registry.
func GetConfigurator(sourcePlatform, targetPlatform, osType string) (Configurator, error) {
	// Handle CUSTOM OS type specially
	if osType == CustomOSType {
		return DefaultCustomConfiguratorFactory.GetCustomConfigurator(sourcePlatform, targetPlatform), nil
	}

	return DefaultConfiguratorRegistry.Get(sourcePlatform, targetPlatform, osType)
}

// RegisterConfigurator is a convenience function to register an configurator to the default registry.
func RegisterConfigurator(configurator Configurator) error {
	return DefaultConfiguratorRegistry.Register(configurator)
}

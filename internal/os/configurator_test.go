// Package os provides tests for OS configurator registry and interfaces.
package os

import (
	"context"
	"testing"

	"github.com/codebypatrickleung/kopru-cli/internal/config"
	"github.com/codebypatrickleung/kopru-cli/internal/logger"
)

// MockConfigurator is a mock configurator for testing.
type MockConfigurator struct {
	name            string
	source          string
	target          string
	osType          string
	configureCalled bool
	shouldFail      bool
}

func (m *MockConfigurator) Name() string           { return m.name }
func (m *MockConfigurator) SourcePlatform() string { return m.source }
func (m *MockConfigurator) TargetPlatform() string { return m.target }
func (m *MockConfigurator) OSType() string         { return m.osType }

func (m *MockConfigurator) ConfigureImage(ctx context.Context, qcow2File string, log *logger.Logger, cfg *config.Config) error {
	m.configureCalled = true
	if m.shouldFail {
		return &mockError{"mock configure error"}
	}
	return nil
}

type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}

func TestConfiguratorRegistry(t *testing.T) {
	t.Run("Register and Get", func(t *testing.T) {
		registry := NewConfiguratorRegistry()
		configurator := &MockConfigurator{
			name:   "Test Configurator",
			source: "test-source",
			target: "test-target",
			osType: "TestOS",
		}

		// Register configurator
		err := registry.Register(configurator)
		if err != nil {
			t.Fatalf("Failed to register configurator: %v", err)
		}

		// Get configurator
		retrieved, err := registry.Get("test-source", "test-target", "TestOS")
		if err != nil {
			t.Fatalf("Failed to get configurator: %v", err)
		}

		if retrieved != configurator {
			t.Error("Retrieved configurator is not the same as registered configurator")
		}
	})

	t.Run("Register Duplicate", func(t *testing.T) {
		registry := NewConfiguratorRegistry()
		configurator1 := &MockConfigurator{source: "source1", target: "target1", osType: "OS1"}
		configurator2 := &MockConfigurator{source: "source1", target: "target1", osType: "OS1"}

		registry.Register(configurator1)
		err := registry.Register(configurator2)

		if err == nil {
			t.Error("Expected error when registering duplicate configurator")
		}
	})

	t.Run("Get Nonexistent", func(t *testing.T) {
		registry := NewConfiguratorRegistry()

		_, err := registry.Get("nonexistent", "configurator", "OS")
		if err == nil {
			t.Error("Expected error when getting nonexistent configurator")
		}
	})

	t.Run("List Configurators", func(t *testing.T) {
		registry := NewConfiguratorRegistry()
		configurator1 := &MockConfigurator{source: "source1", target: "target1", osType: "OS1"}
		configurator2 := &MockConfigurator{source: "source2", target: "target2", osType: "OS2"}

		registry.Register(configurator1)
		registry.Register(configurator2)

		configurators := registry.List()
		if len(configurators) != 2 {
			t.Errorf("Expected 2 configurators, got %d", len(configurators))
		}
	})
}

func TestGetConfigurator(t *testing.T) {
	t.Run("Get Ubuntu Configurator", func(t *testing.T) {
		// Ubuntu configurator is registered in init()
		configurator, err := GetConfigurator("azure", "oci", "Ubuntu")
		if err != nil {
			t.Fatalf("Failed to get Ubuntu configurator: %v", err)
		}

		if configurator == nil {
			t.Error("Ubuntu configurator is nil")
		}

		if configurator.OSType() != "Ubuntu" {
			t.Errorf("Expected OS type 'Ubuntu', got '%s'", configurator.OSType())
		}
	})

	t.Run("Get Custom Configurator", func(t *testing.T) {
		// CUSTOM configurator should be created on demand
		configurator, err := GetConfigurator("azure", "oci", "CUSTOM")
		if err != nil {
			t.Fatalf("Failed to get CUSTOM configurator: %v", err)
		}

		if configurator == nil {
			t.Error("CUSTOM configurator is nil")
		}

		if configurator.OSType() != "CUSTOM" {
			t.Errorf("Expected OS type 'CUSTOM', got '%s'", configurator.OSType())
		}
	})

	t.Run("Get Nonexistent Configurator", func(t *testing.T) {
		_, err := GetConfigurator("nonexistent", "platform", "OS")
		if err == nil {
			t.Error("Expected error when getting nonexistent configurator")
		}
	})
}

func TestUbuntuAzureToOCIConfigurator(t *testing.T) {
	configurator := NewUbuntuAzureToOCIConfigurator()

	t.Run("Name", func(t *testing.T) {
		if configurator.Name() != "Ubuntu Azure to OCI Configurator" {
			t.Errorf("Expected 'Ubuntu Azure to OCI Configurator', got '%s'", configurator.Name())
		}
	})

	t.Run("SourcePlatform", func(t *testing.T) {
		if configurator.SourcePlatform() != "azure" {
			t.Errorf("Expected 'azure', got '%s'", configurator.SourcePlatform())
		}
	})

	t.Run("TargetPlatform", func(t *testing.T) {
		if configurator.TargetPlatform() != "oci" {
			t.Errorf("Expected 'oci', got '%s'", configurator.TargetPlatform())
		}
	})

	t.Run("OSType", func(t *testing.T) {
		if configurator.OSType() != "Ubuntu" {
			t.Errorf("Expected 'Ubuntu', got '%s'", configurator.OSType())
		}
	})
}

func TestCustomConfigurator(t *testing.T) {
	configurator := NewCustomConfigurator("gcp", "oci")

	t.Run("SourcePlatform", func(t *testing.T) {
		if configurator.SourcePlatform() != "gcp" {
			t.Errorf("Expected 'gcp', got '%s'", configurator.SourcePlatform())
		}
	})

	t.Run("TargetPlatform", func(t *testing.T) {
		if configurator.TargetPlatform() != "oci" {
			t.Errorf("Expected 'oci', got '%s'", configurator.TargetPlatform())
		}
	})

	t.Run("OSType", func(t *testing.T) {
		if configurator.OSType() != "CUSTOM" {
			t.Errorf("Expected 'CUSTOM', got '%s'", configurator.OSType())
		}
	})

	t.Run("Name", func(t *testing.T) {
		expected := "Custom Script Configurator (gcp to oci)"
		if configurator.Name() != expected {
			t.Errorf("Expected '%s', got '%s'", expected, configurator.Name())
		}
	})
}

func TestCustomConfiguratorFactory(t *testing.T) {
	factory := DefaultCustomConfiguratorFactory

	t.Run("Create Custom Configurator", func(t *testing.T) {
		configurator := factory.GetCustomConfigurator("test-source", "test-target")

		if configurator == nil {
			t.Error("Factory returned nil configurator")
		}

		if configurator.SourcePlatform() != "test-source" {
			t.Errorf("Expected source 'test-source', got '%s'", configurator.SourcePlatform())
		}

		if configurator.TargetPlatform() != "test-target" {
			t.Errorf("Expected target 'test-target', got '%s'", configurator.TargetPlatform())
		}

		if configurator.OSType() != "CUSTOM" {
			t.Errorf("Expected OS type 'CUSTOM', got '%s'", configurator.OSType())
		}
	})
}

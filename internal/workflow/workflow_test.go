// Package workflow provides tests for workflow registry and interfaces.
package workflow

import (
	"context"
	"testing"

	"github.com/codebypatrickleung/kopru-cli/internal/config"
	"github.com/codebypatrickleung/kopru-cli/internal/logger"
)

// MockHandler is a mock workflow handler for testing.
type MockHandler struct {
	name           string
	source         string
	target         string
	initCalled     bool
	executeCalled  bool
	shouldFailInit bool
	shouldFailExec bool
}

func (m *MockHandler) Name() string           { return m.name }
func (m *MockHandler) SourcePlatform() string { return m.source }
func (m *MockHandler) TargetPlatform() string { return m.target }

func (m *MockHandler) Initialize(cfg *config.Config, log *logger.Logger) error {
	m.initCalled = true
	if m.shouldFailInit {
		return &testError{"mock init error"}
	}
	return nil
}

func (m *MockHandler) Execute(ctx context.Context) error {
	m.executeCalled = true
	if m.shouldFailExec {
		return &testError{"mock execute error"}
	}
	return nil
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestWorkflowRegistry(t *testing.T) {
	t.Run("Register and Get", func(t *testing.T) {
		registry := NewRegistry()
		handler := &MockHandler{
			name:   "Test Handler",
			source: "test-source",
			target: "test-target",
		}

		// Register handler
		err := registry.Register(handler)
		if err != nil {
			t.Fatalf("Failed to register handler: %v", err)
		}

		// Get handler
		retrieved, err := registry.Get("test-source", "test-target")
		if err != nil {
			t.Fatalf("Failed to get handler: %v", err)
		}

		if retrieved != handler {
			t.Error("Retrieved handler is not the same as registered handler")
		}
	})

	t.Run("Register Duplicate", func(t *testing.T) {
		registry := NewRegistry()
		handler1 := &MockHandler{source: "source1", target: "target1"}
		handler2 := &MockHandler{source: "source1", target: "target1"}

		registry.Register(handler1)
		err := registry.Register(handler2)

		if err == nil {
			t.Error("Expected error when registering duplicate handler")
		}
	})

	t.Run("Get Nonexistent", func(t *testing.T) {
		registry := NewRegistry()

		_, err := registry.Get("nonexistent", "handler")
		if err == nil {
			t.Error("Expected error when getting nonexistent handler")
		}
	})

	t.Run("List Handlers", func(t *testing.T) {
		registry := NewRegistry()
		handler1 := &MockHandler{source: "source1", target: "target1"}
		handler2 := &MockHandler{source: "source2", target: "target2"}

		registry.Register(handler1)
		registry.Register(handler2)

		handlers := registry.List()
		if len(handlers) != 2 {
			t.Errorf("Expected 2 handlers, got %d", len(handlers))
		}
	})
}

func TestWorkflowManager(t *testing.T) {
	t.Run("Create Manager with Azure to OCI", func(t *testing.T) {
		cfg := &config.Config{
			SourcePlatform:      "azure",
			TargetPlatform:      "oci",
			AzureComputeName:    "test-vm",
			AzureResourceGroup:  "test-rg",
			AzureSubscriptionID: "test-sub",
			OCICompartmentID:    "test-compartment",
			OCISubnetID:         "test-subnet",
			OCIRegion:           "us-ashburn-1",
		}

		log := logger.New(false)

		// This should succeed because Azure to OCI handler is registered
		manager, err := NewManager(cfg, log, "2.0.0")
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		if manager == nil {
			t.Error("Manager is nil")
		}

		if manager.handler == nil {
			t.Error("Handler is nil")
		}
	})

	t.Run("Create Manager with Unsupported Workflow", func(t *testing.T) {
		cfg := &config.Config{
			SourcePlatform: "unsupported",
			TargetPlatform: "platform",
		}

		log := logger.New(false)

		// This should fail because unsupported workflow
		_, err := NewManager(cfg, log, "2.0.0")
		if err == nil {
			t.Error("Expected error for unsupported workflow")
		}
	})
}

func TestAzureToOCIHandler(t *testing.T) {
	handler := NewAzureToOCIHandler()

	t.Run("Name", func(t *testing.T) {
		if handler.Name() != "Azure to OCI Migration" {
			t.Errorf("Expected 'Azure to OCI Migration', got '%s'", handler.Name())
		}
	})

	t.Run("SourcePlatform", func(t *testing.T) {
		if handler.SourcePlatform() != "azure" {
			t.Errorf("Expected 'azure', got '%s'", handler.SourcePlatform())
		}
	})

	t.Run("TargetPlatform", func(t *testing.T) {
		if handler.TargetPlatform() != "oci" {
			t.Errorf("Expected 'oci', got '%s'", handler.TargetPlatform())
		}
	})
}

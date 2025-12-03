// Package workflow orchestrates the Compute migration workflow.
package workflow

import (
	"context"
	"fmt"

	"github.com/codebypatrickleung/kopru-cli/internal/config"
	"github.com/codebypatrickleung/kopru-cli/internal/logger"
)

// Manager orchestrates the migration workflow by delegating to registered workflow handlers.
type Manager struct {
	config  *config.Config
	logger  *logger.Logger
	handler Handler
	version string
}

// NewManager creates a new workflow manager.
func NewManager(cfg *config.Config, log *logger.Logger, version string) (*Manager, error) {
	// Create registry and register all workflow handlers
	registry := NewRegistry()

	// Register the Azure to OCI workflow handler
	if err := registry.Register(NewAzureToOCIHandler()); err != nil {
		return nil, fmt.Errorf("failed to register Azure to OCI handler: %w", err)
	}

	// Get the appropriate workflow handler for the source and target platforms
	handler, err := registry.Get(cfg.SourcePlatform, cfg.TargetPlatform)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow handler: %w", err)
	}

	// Initialize the handler
	if err := handler.Initialize(cfg, log); err != nil {
		return nil, fmt.Errorf("failed to initialize workflow handler: %w", err)
	}

	return &Manager{
		config:  cfg,
		logger:  log,
		handler: handler,
		version: version,
	}, nil
}

// Run executes the complete migration workflow by delegating to the registered handler.
func (m *Manager) Run(ctx context.Context) error {
	m.logger.Info("=========================================")
	m.logger.Infof("Kopru - Compute Migration Tool v%s", m.version)
	m.logger.Info("=========================================")
	m.logger.Infof("Source Platform: %s", m.config.SourcePlatform)
	m.logger.Infof("Target Platform: %s", m.config.TargetPlatform)
	m.logger.Info("=========================================")

	// Execute the workflow handler
	if err := m.handler.Execute(ctx); err != nil {
		m.logger.Error(fmt.Sprintf("Workflow failed: %v", err))
		return err
	}

	return nil
}

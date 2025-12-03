// Package workflow defines interfaces for workflow abstraction.
package workflow

import (
	"context"

	"github.com/codebypatrickleung/kopru-cli/internal/config"
	"github.com/codebypatrickleung/kopru-cli/internal/logger"
)

// Handler defines the interface for a workflow handler that orchestrates migration.
// Each workflow handler implements a specific source-to-target migration path.
type Handler interface {
	// Name returns the name of the workflow (e.g., "azure-to-oci", "aws-to-oci")
	Name() string

	// SourcePlatform returns the source cloud platform identifier
	SourcePlatform() string

	// TargetPlatform returns the target cloud platform identifier
	TargetPlatform() string

	// Initialize prepares the workflow handler with configuration and logger
	Initialize(cfg *config.Config, log *logger.Logger) error

	// Execute runs the complete migration workflow
	Execute(ctx context.Context) error
}

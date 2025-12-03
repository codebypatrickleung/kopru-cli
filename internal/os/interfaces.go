// Package os handles OS configuration operations for different platforms and OS types.
package os

import (
	"context"

	"github.com/codebypatrickleung/kopru-cli/internal/config"
	"github.com/codebypatrickleung/kopru-cli/internal/logger"
)

// Configurator defines the interface for OS-specific image configuration.
// Each configurator implements configuration changes for a specific source platform, target platform,
// and operating system combination.
type Configurator interface {
	// Name returns the name of this configurator (e.g., "Ubuntu Azure to OCI")
	Name() string

	// SourcePlatform returns the source platform identifier (e.g., "azure", "gcp")
	SourcePlatform() string

	// TargetPlatform returns the target platform identifier (e.g., "oci", "azure")
	TargetPlatform() string

	// OSType returns the operating system type this configurator supports (e.g., "Ubuntu", "RHEL")
	OSType() string

	// ConfigureImage performs the image configuration operations
	// qcow2File: path to the QCOW2 image file
	// log: logger instance for output
	// cfg: configuration containing custom scripts and other settings
	ConfigureImage(ctx context.Context, qcow2File string, log *logger.Logger, cfg *config.Config) error
}

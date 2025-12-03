// Package os provides Ubuntu-specific configurations for Azure to OCI migration.
package os

import (
	"context"
	"fmt"

	"github.com/codebypatrickleung/kopru-cli/internal/config"
	"github.com/codebypatrickleung/kopru-cli/internal/logger"
)

// UbuntuAzureToOCIConfigurator implements OS configurations for Ubuntu from Azure to OCI.
type UbuntuAzureToOCIConfigurator struct{}

// NewUbuntuAzureToOCIConfigurator creates a new Ubuntu Azure to OCI configurator.
func NewUbuntuAzureToOCIConfigurator() *UbuntuAzureToOCIConfigurator {
	return &UbuntuAzureToOCIConfigurator{}
}

// Name returns the name of this configurator.
func (a *UbuntuAzureToOCIConfigurator) Name() string {
	return "Ubuntu Azure to OCI Configurator"
}

// SourcePlatform returns the source platform identifier.
func (a *UbuntuAzureToOCIConfigurator) SourcePlatform() string {
	return "azure"
}

// TargetPlatform returns the target platform identifier.
func (a *UbuntuAzureToOCIConfigurator) TargetPlatform() string {
	return "oci"
}

// OSType returns the operating system type.
func (a *UbuntuAzureToOCIConfigurator) OSType() string {
	return "Ubuntu"
}

// ConfigureImage performs Ubuntu-specific configurations for Azure to OCI migration.
func (a *UbuntuAzureToOCIConfigurator) ConfigureImage(ctx context.Context, qcow2File string, log *logger.Logger, cfg *config.Config) error {
	log.Info("Applying Ubuntu configurations for Azure to OCI migration...")

	// Create image manager for mounting/unmounting
	manager := NewManager(log, a.SourcePlatform())

	// Mount the QCOW2 image
	if err := manager.MountQCOW2(qcow2File); err != nil {
		return fmt.Errorf("failed to mount QCOW2 image: %w", err)
	}

	// Ensure cleanup happens
	defer func() {
		if cfg.CleanupMount {
			if err := manager.UnmountQCOW2(); err != nil {
				log.Warning(fmt.Sprintf("Failed to unmount QCOW2: %v", err))
			}
		} else {
			log.Info("Skipping cleanup of mount and NBD (CLEANUP_MOUNT=false)")
		}
	}()

	// Apply Ubuntu-specific configurations
	if err := manager.ApplyUbuntuConfigurations(); err != nil {
		return fmt.Errorf("failed to apply Ubuntu configurations: %w", err)
	}

	log.Success("Ubuntu configurations completed successfully")
	return nil
}

// init registers this configurator with the default registry.
func init() {
	RegisterConfigurator(NewUbuntuAzureToOCIConfigurator())
}

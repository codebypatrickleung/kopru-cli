// Package os provides Ubuntu-specific configurations for Azure to OCI migration.
package os

import (
	"context"
	"fmt"

	"github.com/codebypatrickleung/kopru-cli/internal/common"
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

	// Create image manager for OS configurations
	manager := NewManager(log, a.SourcePlatform())

	// Mount the QCOW2 image
	log.Info("Mounting QCOW2 image using NBD...")
	mountDir, _, err := common.MountQCOW2Image(qcow2File, "/dev/nbd0")
	if err != nil {
		return fmt.Errorf("failed to mount QCOW2 image: %w", err)
	}
	log.Successf("Successfully mounted QCOW2 image at %s", mountDir)
	
	// Set mount directory in manager for configuration functions to use
	manager.MountDir = mountDir

	// Ensure cleanup happens
	defer func() {
		log.Info("Unmounting QCOW2 image...")
		if err := common.CleanupNBDMount("/dev/nbd0", mountDir); err != nil {
			log.Warning(fmt.Sprintf("Failed to unmount QCOW2: %v", err))
		} else {
			log.Success("QCOW2 image unmounted")
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

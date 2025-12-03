// Package os provides custom script-based configurations.
package os

import (
	"context"
	"fmt"

	"github.com/codebypatrickleung/kopru-cli/internal/config"
	"github.com/codebypatrickleung/kopru-cli/internal/logger"
)

// CustomConfigurator implements custom script-based OS configurations.
// This configurator can be used for any source/target platform combination
// when OCI_IMAGE_OS is set to "CUSTOM".
type CustomConfigurator struct {
	sourcePlatform string
	targetPlatform string
}

// NewCustomConfigurator creates a new custom configurator.
func NewCustomConfigurator(sourcePlatform, targetPlatform string) *CustomConfigurator {
	return &CustomConfigurator{
		sourcePlatform: sourcePlatform,
		targetPlatform: targetPlatform,
	}
}

// Name returns the name of this configurator.
func (a *CustomConfigurator) Name() string {
	return fmt.Sprintf("Custom Script Configurator (%s to %s)", a.sourcePlatform, a.targetPlatform)
}

// SourcePlatform returns the source platform identifier.
func (a *CustomConfigurator) SourcePlatform() string {
	return a.sourcePlatform
}

// TargetPlatform returns the target platform identifier.
func (a *CustomConfigurator) TargetPlatform() string {
	return a.targetPlatform
}

// OSType returns the operating system type.
func (a *CustomConfigurator) OSType() string {
	return CustomOSType
}

// ConfigureImage performs custom script-based configurations.
func (a *CustomConfigurator) ConfigureImage(ctx context.Context, qcow2File string, log *logger.Logger, cfg *config.Config) error {
	if cfg.CustomOSConfigurationScript == "" {
		return fmt.Errorf("custom OS configuration script required when OCI_IMAGE_OS=CUSTOM")
	}

	log.Infof("Applying custom configuration script: %s", cfg.CustomOSConfigurationScript)

	// Create image manager for mounting/unmounting
	manager := NewManager(log, a.sourcePlatform)

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

	// Apply custom script
	if err := manager.ApplyCustomScript(cfg.CustomOSConfigurationScript); err != nil {
		return fmt.Errorf("custom configuration script failed: %w", err)
	}

	log.Success("Custom configurations completed successfully")
	return nil
}

// CustomConfiguratorFactory creates custom configurators on demand.
type CustomConfiguratorFactory struct{}

// GetCustomConfigurator returns a custom configurator for the given platforms.
// This is used internally when OCI_IMAGE_OS is set to "CUSTOM".
func (f *CustomConfiguratorFactory) GetCustomConfigurator(sourcePlatform, targetPlatform string) Configurator {
	return NewCustomConfigurator(sourcePlatform, targetPlatform)
}

// DefaultCustomConfiguratorFactory is the global custom configurator factory.
var DefaultCustomConfiguratorFactory = &CustomConfiguratorFactory{}

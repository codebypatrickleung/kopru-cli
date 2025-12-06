// Package common provides utility functions for OS configuration.
package common

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/codebypatrickleung/kopru-cli/internal/logger"
)

const (
	// CustomOSType is the identifier for custom script-based OS configurations
	CustomOSType = "CUSTOM"
)

// ExecuteOSConfigScript executes an OS configuration script from the scripts/os-config directory.
// It determines the appropriate script based on OS type and source platform.
func ExecuteOSConfigScript(mountDir, osType, sourcePlatform string, log *logger.Logger) error {
	var scriptName string

	// Determine which script to run based on OS type and source platform
	if osType == CustomOSType {
		return fmt.Errorf("CUSTOM OS type should use ExecuteCustomOSConfigScript instead")
	}

	// For Ubuntu from Azure to OCI
	if osType == "Ubuntu" && sourcePlatform == "azure" {
		scriptName = "ubuntu_azure_to_oci.sh"
	} else {
		return fmt.Errorf("unsupported OS type '%s' for source platform '%s'", osType, sourcePlatform)
	}

	return executeScript(mountDir, scriptName, log, true)
}

// ExecuteCustomOSConfigScript executes a custom OS configuration script provided by the user.
func ExecuteCustomOSConfigScript(mountDir, scriptPath string, log *logger.Logger) error {
	log.Infof("Applying custom configuration script: %s", scriptPath)

	// Verify script exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("custom configuration script not found: %s", scriptPath)
	}

	return executeScript(mountDir, scriptPath, log, false)
}

// executeScript executes a bash script with the mount directory as argument.
// If isBuiltIn is true, the script is expected to be in the scripts/os-config directory relative to the executable.
// If isBuiltIn is false, scriptPath is treated as an absolute or relative path to the script.
func executeScript(mountDir, scriptPath string, log *logger.Logger, isBuiltIn bool) error {
	var fullScriptPath string

	if isBuiltIn {
		// Get the directory where the binary is located
		execPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}
		execDir := filepath.Dir(execPath)

		// Construct path to the script relative to the executable
		fullScriptPath = filepath.Join(execDir, "scripts", "os-config", scriptPath)

		// Verify script exists
		if _, err := os.Stat(fullScriptPath); os.IsNotExist(err) {
			return fmt.Errorf("OS configuration script not found: %s", fullScriptPath)
		}

		log.Infof("Executing OS configuration script: %s", fullScriptPath)
	} else {
		fullScriptPath = scriptPath
	}

	// Make script executable if it isn't
	if err := os.Chmod(fullScriptPath, 0755); err != nil {
		log.Warning(fmt.Sprintf("Could not make script executable: %v", err))
	}

	// Set environment variable for the script
	env := append(os.Environ(), fmt.Sprintf("KOPRU_MOUNT_DIR=%s", mountDir))

	// Execute the script with mount directory as argument
	var cmd *exec.Cmd
	if isBuiltIn {
		// Built-in scripts need sudo
		cmd = exec.Command("sudo", fullScriptPath, mountDir)
	} else {
		// Custom scripts run as provided
		cmd = exec.Command(fullScriptPath, mountDir)
	}
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("OS configuration script failed: %w", err)
	}

	return nil
}

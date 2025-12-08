// Package common provides utility functions for OS configuration.
package common

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/codebypatrickleung/kopru-cli/internal/logger"
)

const CustomOSType = "CUSTOM"

// ExecuteOSConfigScript executes an OS configuration script from the scripts/os-config directory.
func ExecuteOSConfigScript(mountDir, osType, sourcePlatform string, log *logger.Logger) error {
	if osType == CustomOSType {
		return fmt.Errorf("CUSTOM OS type should use ExecuteCustomOSConfigScript instead")
	}
	if osType == "Ubuntu" && sourcePlatform == "azure" {
		return executeScript(mountDir, "ubuntu_azure_to_oci.sh", log, true)
	}
	return fmt.Errorf("unsupported OS type '%s' for source platform '%s'", osType, sourcePlatform)
}

// ExecuteCustomOSConfigScript executes a custom OS configuration script provided by the user.
func ExecuteCustomOSConfigScript(mountDir, scriptPath string, log *logger.Logger) error {
	log.Infof("Applying custom configuration script: %s", scriptPath)
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("custom configuration script not found: %s", scriptPath)
	}
	return executeScript(mountDir, scriptPath, log, false)
}

// executeScript executes a bash script with the mount directory as argument.
func executeScript(mountDir, scriptPath string, log *logger.Logger, isBuiltIn bool) error {
	var fullScriptPath string
	if isBuiltIn {
		execPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}
		fullScriptPath = filepath.Join(filepath.Dir(execPath), "scripts", "os-config", scriptPath)
		if _, err := os.Stat(fullScriptPath); os.IsNotExist(err) {
			return fmt.Errorf("OS configuration script not found: %s", fullScriptPath)
		}
		log.Infof("Executing OS configuration script: %s", fullScriptPath)
	} else {
		fullScriptPath = scriptPath
	}

	if err := os.Chmod(fullScriptPath, 0755); err != nil {
		log.Warning(fmt.Sprintf("Could not make script executable: %v", err))
	}

	env := append(os.Environ(), fmt.Sprintf("KOPRU_MOUNT_DIR=%s", mountDir))
	var cmd *exec.Cmd
	if isBuiltIn {
		cmd = exec.Command("sudo", fullScriptPath, mountDir)
	} else {
		cmd = exec.Command(fullScriptPath, mountDir)
	}
	cmd.Env = env

	log.Infof("Starting script execution: %s", filepath.Base(fullScriptPath))

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start OS configuration script: %w", err)
	}

	var wg sync.WaitGroup
	readAndLog := func(pipe io.ReadCloser) {
		defer wg.Done()
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			log.Info(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.Warningf("Error reading script output: %v", err)
		}
	}
	wg.Add(2)
	go readAndLog(stdoutPipe)
	go readAndLog(stderrPipe)
	wg.Wait()

	if err := cmd.Wait(); err != nil {
		log.Errorf("Script execution failed: %s", filepath.Base(fullScriptPath))
		return fmt.Errorf("OS configuration script failed: %w", err)
	}

	log.Successf("Script execution completed: %s", filepath.Base(fullScriptPath))
	return nil
}

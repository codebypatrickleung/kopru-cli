// Package common provides utility functions used across the Kopru CLI.
package common

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// EnsureDir creates a directory if it doesn't exist.
func EnsureDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}
	return nil
}

// SanitizeName sanitizes a string for use in file/directory names.
func SanitizeName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	// Keep only alphanumeric characters, hyphens, and underscores
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// CheckCommand checks if a command is available in the system PATH.
func CheckCommand(cmd string) error {
	_, err := exec.LookPath(cmd)
	if err != nil {
		return fmt.Errorf("command '%s' not found in PATH", cmd)
	}
	return nil
}

// GetAvailableDiskSpace returns the available disk space in bytes for the given path.
func GetAvailableDiskSpace(path string) (int64, error) {
	// Get the absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return 0, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Use df command to get disk space
	cmd := exec.Command("df", "-B1", absPath)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get disk space: %w", err)
	}

	// Parse df output
	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return 0, fmt.Errorf("unexpected df output format")
	}

	// The second line contains the disk space information
	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		return 0, fmt.Errorf("unexpected df output format")
	}

	// The fourth field is the available space
	var available int64
	_, err = fmt.Sscanf(fields[3], "%d", &available)
	if err != nil {
		return 0, fmt.Errorf("failed to parse available disk space: %w", err)
	}

	return available, nil
}

// RunCommand executes a command and returns the output.
func RunCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}
	return string(output), nil
}

// ConvertVHDToQCOW2 converts a VHD file to QCOW2 format.
// It finds the VHD file in the specified directory, converts it using qemu-img,
// and optionally removes the VHD file after conversion.
func ConvertVHDToQCOW2(vhdFile, qcow2File string, removeVHD bool) error {
	// Convert using qemu-img
	output, err := RunCommand("qemu-img", "convert", "-f", "vpc", "-O", "qcow2", vhdFile, qcow2File)
	if err != nil {
		return fmt.Errorf("qemu-img convert failed: %w\nOutput: %s", err, output)
	}

	// Workaround: Resize QCOW2 file by +5MB to workaround the disk size issue after conversion
	output, err = RunCommand("qemu-img", "resize", qcow2File, "+5M")
	if err != nil {
		return fmt.Errorf("qemu-img resize failed: %w\nOutput: %s", err, output)
	}

	// Optionally remove VHD file
	if removeVHD {
		if err := os.Remove(vhdFile); err != nil {
			return fmt.Errorf("failed to remove VHD file: %w", err)
		}
	}

	return nil
}

// CheckDiskSpace checks if the path has sufficient disk space.
// It returns an error if available space is less than minDiskSpaceGB.
func CheckDiskSpace(path string, minDiskSpaceGB int64) error {
	available, err := GetAvailableDiskSpace(path)
	if err != nil {
		return err
	}

	availableGB := available / (1024 * 1024 * 1024)
	if availableGB < minDiskSpaceGB {
		return fmt.Errorf("insufficient disk space: %d GB available, %d GB recommended", availableGB, minDiskSpaceGB)
	}

	return nil
}

// FindVHDFile finds the first VHD file in the specified directory.
func FindVHDFile(dir string) (string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.vhd"))
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no VHD file found in %s", dir)
	}
	return files[0], nil
}

// FindQCOW2File finds the first QCOW2 file in the specified directory.
func FindQCOW2File(dir string) (string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.qcow2"))
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no QCOW2 file found in %s", dir)
	}
	return files[0], nil
}

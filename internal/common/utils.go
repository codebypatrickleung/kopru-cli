// Package common provides utility functions used across the Kopru CLI.
package common

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const (
	// OCIMinVolumeSizeGB is the minimum volume size in GB for OCI block volumes
	OCIMinVolumeSizeGB = 50
	// MinDiskSpaceGB is the recommended minimum disk space in GB for migration operations
	MinDiskSpaceGB = 100
)

// CheckCommand returns an error if the command is not found in PATH.
func CheckCommand(cmd string) error {
	_, err := exec.LookPath(cmd)
	if err != nil {
		return fmt.Errorf("command '%s' not found in PATH", cmd)
	}
	return nil
}

// RunCommand executes a command and returns its output and error.
func RunCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}
	return string(output), nil
}

// SanitizeName returns a lowercase, safe string for file/directory names.
func SanitizeName(name string) string {
	name = strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, name)
}

// EnsureDir creates a directory if it doesn't exist.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// FindDiskFile finds the first file with the specified extension in the directory.
// The extension parameter should include the dot (e.g., ".vhd", ".qcow2").
func FindDiskFile(dir string, extension string) (string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*"+extension))
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no %s file found in %s", extension, dir)
	}
	return files[0], nil
}

// GetAvailableDiskSpace returns the available disk space in bytes for the given path using unix.Statfs.
func GetAvailableDiskSpace(path string, minDiskSpaceGB int64) (int64, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return 0, fmt.Errorf("failed to get absolute path: %w", err)
	}
	var stat unix.Statfs_t
	if err := unix.Statfs(absPath, &stat); err != nil {
		return 0, fmt.Errorf("failed to get disk space: %w", err)
	}
	available := int64(stat.Bavail) * int64(stat.Bsize)
	if minDiskSpaceGB > 0 {
		availableGB := available / (1024 * 1024 * 1024)
		if availableGB < minDiskSpaceGB {
			return available, fmt.Errorf("insufficient disk space: %d GB available, %d GB recommended", availableGB, minDiskSpaceGB)
		}
	}
	return available, nil
}

// GetFileSizeGB returns the size of a file in gigabytes.
func GetFileSizeGB(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}

	// Get size in bytes and convert to GB, rounding up
	sizeBytes := info.Size()
	sizeGB := (sizeBytes + (1024*1024*1024 - 1)) / (1024 * 1024 * 1024)

	// Enforce OCI minimum volume size
	if sizeGB < OCIMinVolumeSizeGB {
		sizeGB = OCIMinVolumeSizeGB
	}

	return sizeGB, nil
}

// WriteFile writes content to a file using a temporary file and sudo.
// It creates a temporary file, writes the content, sets permissions, and moves it to the final location.
func WriteFile(path, content string) error {
	// Create a temporary file securely
	tmpFile, err := os.CreateTemp(filepath.Dir(path), ".kopru-tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Write content and close
	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Set appropriate permissions
	if err := os.Chmod(tmpPath, 0644); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Move the file using sudo
	if _, err := RunCommand("sudo", "mv", tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to move temp file: %w", err)
	}

	return nil
}

// CopyDataWithDD copies data from source to destination using dd.
func CopyDataWithDD(source, destination string) error {
	cmd := exec.Command("sudo", "dd",
		"if="+source,
		"of="+destination,
		"bs=4M",
		"status=progress",
		"conv=fsync")

	// Redirect output to /dev/null to avoid cluttering logs
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open /dev/null: %w", err)
	}
	defer devNull.Close()

	cmd.Stdout = devNull
	cmd.Stderr = devNull

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy data with dd: %w", err)
	}

	return nil
}

// SliceDifference returns elements in slice a that are not in slice b.
func SliceDifference(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

// HasFilesystem returns true if blkid detects a filesystem on the device.
func HasFilesystem(device string) bool {
	cmd := exec.Command("sudo", "blkid", device)
	output, err := cmd.Output()
	return err == nil && len(output) > 0
}

// ListBlockDevices returns a list of block device names (without /dev/ prefix).
func ListBlockDevices() ([]string, error) {
	cmd := exec.Command("lsblk", "-dn", "-o", "NAME")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list block devices: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var devices []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			devices = append(devices, line)
		}
	}
	return devices, nil
}

// DetectNewBlockDevice detects a newly attached block device by comparing before and after device lists.
func DetectNewBlockDevice(beforeDevices []string) (string, error) {
	time.Sleep(3 * time.Second)
	afterDevices, err := ListBlockDevices()
	if err != nil {
		return "", err
	}
	newDevices := SliceDifference(afterDevices, beforeDevices)
	if len(newDevices) == 0 {
		return "", fmt.Errorf("no new block device detected")
	}
	return "/dev/" + newDevices[0], nil
}

// ConvertVHDToQCOW2 converts a VHD file to QCOW2 format and optionally removes the VHD file.
func ConvertVHDToQCOW2(vhdFile, qcow2File string, removeVHD bool) error {
	output, err := RunCommand("qemu-img", "convert", "-f", "vpc", "-O", "qcow2", vhdFile, qcow2File)
	if err != nil {
		return fmt.Errorf("qemu-img convert failed: %w\nOutput: %s", err, output)
	}
	output, err = RunCommand("qemu-img", "resize", qcow2File, "+5M")
	if err != nil {
		return fmt.Errorf("qemu-img resize failed: %w\nOutput: %s", err, output)
	}
	if removeVHD {
		if err := os.Remove(vhdFile); err != nil {
			return fmt.Errorf("failed to remove VHD file: %w", err)
		}
	}
	return nil
}

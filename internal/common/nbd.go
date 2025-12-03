// Package common provides utility functions for NBD (Network Block Device) operations.
package common

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	// OCIMinVolumeSizeGB is the minimum volume size in GB for OCI block volumes
	OCIMinVolumeSizeGB = 50
	// MinDiskSpaceGB is the recommended minimum disk space in GB for migration operations
	MinDiskSpaceGB = 100
)

// LoadNBDModule loads the NBD kernel module with specified parameters.
func LoadNBDModule() error {
	// Check if module is already loaded
	if isNBDModuleLoaded() {
		return nil
	}

	// Load the module with parameters
	cmd := exec.Command("sudo", "modprobe", "nbd", "max_part=8", "nbds_max=16")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to load NBD module: %w", err)
	}

	// Wait a moment for the module to initialize
	time.Sleep(1 * time.Second)

	return nil
}

// isNBDModuleLoaded checks if the NBD module is loaded.
func isNBDModuleLoaded() bool {
	// Check if /dev/nbd0 exists
	_, err := os.Stat("/dev/nbd0")
	return err == nil
}

// ConnectVHDToNBD connects a VHD file to an NBD device.
// Note: Azure exports VHDs as "fixed" format which are raw disk images with a VHD footer.
// We use -f raw because qemu-nbd can handle fixed VHDs as raw images.
func ConnectVHDToNBD(vhdFile, nbdDevice string) error {
	// Use qemu-nbd to connect the VHD to the NBD device
	cmd := exec.Command("sudo", "qemu-nbd", "--connect="+nbdDevice, vhdFile, "-f", "raw")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to connect VHD to NBD: %w", err)
	}

	// Wait for device to be ready
	time.Sleep(3 * time.Second)

	return nil
}

// DisconnectNBD disconnects an NBD device.
func DisconnectNBD(nbdDevice string) error {
	cmd := exec.Command("sudo", "qemu-nbd", "--disconnect", nbdDevice)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to disconnect NBD: %w", err)
	}

	return nil
}

// FindMountablePartition finds a mountable partition on an NBD device.
// It tries to find partitions in order: nbd0p1, nbd0p2, etc., or the device itself.
func FindMountablePartition(nbdDevice string) (string, error) {
	// First, try to find partitions
	partitions := []string{
		nbdDevice + "p1",
		nbdDevice + "p2",
		nbdDevice + "p3",
		nbdDevice + "1",
		nbdDevice + "2",
		nbdDevice + "3",
		nbdDevice, // Try the device itself if no partitions
	}

	for _, partition := range partitions {
		if _, err := os.Stat(partition); err == nil {
			// Check if partition has a filesystem
			if hasFilesystem(partition) {
				return partition, nil
			}
		}
	}

	return "", fmt.Errorf("no mountable partition found on %s", nbdDevice)
}

// hasFilesystem checks if a device has a recognizable filesystem.
func hasFilesystem(device string) bool {
	cmd := exec.Command("sudo", "blkid", device)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// If blkid returns output, it has a filesystem
	return len(output) > 0
}

// MountPartition mounts a partition to a mount point.
func MountPartition(partition, mountPoint string) error {
	cmd := exec.Command("sudo", "mount", partition, mountPoint)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to mount partition: %w", err)
	}

	return nil
}

// UnmountPartition unmounts a mount point.
func UnmountPartition(mountPoint string) error {
	cmd := exec.Command("sudo", "umount", mountPoint)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to unmount partition: %w", err)
	}

	return nil
}

// CleanupNBDMount cleans up NBD and mount operations.
func CleanupNBDMount(nbdDevice, mountPoint string) error {
	var lastErr error

	// Unmount if mount point is provided and exists
	if mountPoint != "" {
		if _, err := os.Stat(mountPoint); err == nil {
			if err := UnmountPartition(mountPoint); err != nil {
				lastErr = err
			}
		}
		// Remove mount point directory
		os.RemoveAll(mountPoint)
	}

	// Disconnect NBD if device is provided
	if nbdDevice != "" {
		if err := DisconnectNBD(nbdDevice); err != nil {
			lastErr = err
		}
	}

	return lastErr
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

// DetectNewBlockDevice detects a newly attached block device by comparing before and after device lists.
func DetectNewBlockDevice(beforeDevices []string) (string, error) {
	// Wait a moment for the new device to appear
	time.Sleep(3 * time.Second)

	// Get current devices
	afterDevices, err := listBlockDevices()
	if err != nil {
		return "", err
	}

	// Find the difference
	newDevices := difference(afterDevices, beforeDevices)
	if len(newDevices) == 0 {
		return "", fmt.Errorf("no new block device detected")
	}

	// Return the first new device
	return "/dev/" + newDevices[0], nil
}

// ListBlockDevices returns a list of current block devices.
func ListBlockDevices() ([]string, error) {
	return listBlockDevices()
}

// listBlockDevices returns a list of block device names (without /dev/ prefix).
func listBlockDevices() ([]string, error) {
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

// difference returns elements in slice a that are not in slice b.
func difference(a, b []string) []string {
	mb := make(map[string]bool, len(b))
	for _, x := range b {
		mb[x] = true
	}

	var diff []string
	for _, x := range a {
		if !mb[x] {
			diff = append(diff, x)
		}
	}

	return diff
}

// Package common provides utility functions for NBD (Network Block Device) operations.

package common

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// LoadNBDModule loads the NBD kernel module with specified parameters.
func LoadNBDModule() error {
	// Check if module is already loaded
	if IsNBDModuleLoaded() {
		return nil
	}

	// Load the module with parameters
	cmd := exec.Command("sudo", "modprobe", "nbd", "max_part=16", "nbds_max=16")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to load NBD module: %w", err)
	}

	// Wait a moment for the module to initialize
	time.Sleep(1 * time.Second)

	return nil
}

// IsNBDModuleLoaded checks if the NBD module is loaded.
func IsNBDModuleLoaded() bool {
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

// ConnectQCOW2ToNBD connects a QCOW2 file to an NBD device.
func ConnectQCOW2ToNBD(qcow2File, nbdDevice string) error {
	// Use qemu-nbd to connect the QCOW2 to the NBD device
	cmd := exec.Command("sudo", "qemu-nbd", "--connect="+nbdDevice, qcow2File, "-f", "qcow2")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to connect QCOW2 to NBD: %w", err)
	}

	// Wait for device to be ready (NBD device nodes need time to appear)
	// This is necessary for the kernel to create partition devices
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
			if HasFilesystem(partition) {
				return partition, nil
			}
		}
	}

	return "", fmt.Errorf("no mountable partition found on %s", nbdDevice)
}

// MountQCOW2Image mounts a QCOW2 or VHD image using NBD and returns the mount directory and partition.
// It handles loading the NBD module, connecting the image, finding a mountable partition, and mounting it.
// The caller is responsible for calling CleanupNBDMount when done.
func MountQCOW2Image(imageFile, nbdDevice string) (mountDir string, partition string, err error) {
	// Verify file exists
	if _, err := os.Stat(imageFile); os.IsNotExist(err) {
		return "", "", fmt.Errorf("image file not found: %s", imageFile)
	}

	// Load NBD kernel module
	if err := LoadNBDModule(); err != nil {
		return "", "", fmt.Errorf("failed to load NBD module: %w", err)
	}

	// Disconnect any existing NBD connection
	RunCommand("sudo", "qemu-nbd", "--disconnect", nbdDevice) // Ignore errors

	// Determine file type and connect to NBD device (case-insensitive extension check)
	lowerImageFile := strings.ToLower(imageFile)
	if strings.HasSuffix(lowerImageFile, ".qcow2") {
		// Connect QCOW2 to NBD device
		if err := ConnectQCOW2ToNBD(imageFile, nbdDevice); err != nil {
			return "", "", err
		}
	} else if strings.HasSuffix(lowerImageFile, ".vhd") {
		// Connect VHD to NBD device
		if err := ConnectVHDToNBD(imageFile, nbdDevice); err != nil {
			return "", "", err
		}
	} else {
		return "", "", fmt.Errorf("unsupported file type: %s (supported types: .qcow2, .vhd)", imageFile)
	}

	// Find mountable partition
	targetPartition, err := FindMountablePartition(nbdDevice)
	if err != nil {
		CleanupNBDMount(nbdDevice, "") // mountDir not yet created, pass empty string
		return "", "", fmt.Errorf("failed to find partition: %w", err)
	}

	// Create temporary mount directory
	mountDir, err = os.MkdirTemp("", "kopru-mount-*")
	if err != nil {
		CleanupNBDMount(nbdDevice, "") // mountDir creation failed, pass empty string
		return "", "", fmt.Errorf("failed to create mount directory: %w", err)
	}

	// Mount the partition
	if err := MountPartition(targetPartition, mountDir); err != nil {
		CleanupNBDMount(nbdDevice, mountDir)
		return "", "", fmt.Errorf("failed to mount partition: %w", err)
	}

	return mountDir, targetPartition, nil
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


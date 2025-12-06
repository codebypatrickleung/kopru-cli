// Package common provides utility functions for NBD (Network Block Device) operations.
package common

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// MountQCOW2Image loads, connects, finds, and mounts a QCOW2 or VHD image.
func MountQCOW2Image(imageFile, nbdDevice string) (mountDir, partition string, err error) {
	if _, err := os.Stat(imageFile); os.IsNotExist(err) {
		return "", "", fmt.Errorf("image file not found: %s", imageFile)
	}
	if err := LoadNBDModule(); err != nil {
		return "", "", fmt.Errorf("failed to load NBD module: %w", err)
	}
	RunCommand("sudo", "qemu-nbd", "--disconnect", nbdDevice)
	lowerImageFile := strings.ToLower(imageFile)
	if strings.HasSuffix(lowerImageFile, ".qcow2") {
		if err := ConnectQCOW2ToNBD(imageFile, nbdDevice); err != nil {
			return "", "", err
		}
	} else if strings.HasSuffix(lowerImageFile, ".vhd") {
		if err := ConnectVHDToNBD(imageFile, nbdDevice); err != nil {
			return "", "", err
		}
	} else {
		return "", "", fmt.Errorf("unsupported file type: %s (supported types: .qcow2, .vhd)", imageFile)
	}
	targetPartition, err := FindMountablePartition(nbdDevice)
	if err != nil {
		CleanupNBDMount(nbdDevice, "")
		return "", "", fmt.Errorf("failed to find partition: %w", err)
	}
	mountDir, err = os.MkdirTemp("", "kopru-mount-*")
	if err != nil {
		CleanupNBDMount(nbdDevice, "")
		return "", "", fmt.Errorf("failed to create mount directory: %w", err)
	}
	if err := MountPartition(targetPartition, mountDir); err != nil {
		CleanupNBDMount(nbdDevice, mountDir)
		return "", "", fmt.Errorf("failed to mount partition: %w", err)
	}
	return mountDir, targetPartition, nil
}

// FindMountablePartition returns the first mountable partition or device.
func FindMountablePartition(nbdDevice string) (string, error) {
	partitions := []string{
		nbdDevice + "p1",
		nbdDevice + "p2",
		nbdDevice + "p3",
		nbdDevice + "1",
		nbdDevice + "2",
		nbdDevice + "3",
		nbdDevice, 
	}
	for _, partition := range partitions {
		if _, err := os.Stat(partition); err == nil {
			if HasFilesystem(partition) {
				return partition, nil
			}
		}
	}
	return "", fmt.Errorf("no mountable partition found on %s", nbdDevice)
}

// CleanupNBDMount unmounts and disconnects NBD devices and removes the mount directory.
func CleanupNBDMount(nbdDevice, mountPoint string) error {
	var lastErr error
	if mountPoint != "" {
		if _, err := os.Stat(mountPoint); err == nil {
			if err := UnmountPartition(mountPoint); err != nil {
				lastErr = err
			}
		}
		os.RemoveAll(mountPoint)
	}
	if nbdDevice != "" {
		if err := DisconnectNBD(nbdDevice); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// LoadNBDModule loads the NBD kernel module with specified parameters.
func LoadNBDModule() error {
	if IsNBDModuleLoaded() {
		return nil
	}
	cmd := exec.Command("sudo", "modprobe", "nbd", "max_part=8", "nbds_max=8")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to load NBD module: %w", err)
	}
	time.Sleep(1 * time.Second)
	return nil
}

// IsNBDModuleLoaded returns true if the NBD module is loaded (i.e., /dev/nbd0 exists).
func IsNBDModuleLoaded() bool {
	_, err := os.Stat("/dev/nbd0")
	return err == nil
}

// ConnectVHDToNBD connects a VHD file to an NBD device.
func ConnectVHDToNBD(vhdFile, nbdDevice string) error {
	cmd := exec.Command("sudo", "qemu-nbd", "--connect="+nbdDevice, vhdFile, "-f", "raw")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to connect VHD to NBD: %w", err)
	}
	time.Sleep(3 * time.Second)
	return nil
}

// ConnectQCOW2ToNBD connects a QCOW2 file to an NBD device.
func ConnectQCOW2ToNBD(qcow2File, nbdDevice string) error {
	cmd := exec.Command("sudo", "qemu-nbd", "--connect="+nbdDevice, qcow2File, "-f", "qcow2")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to connect QCOW2 to NBD: %w", err)
	}
	// Wait for device nodes to appear
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

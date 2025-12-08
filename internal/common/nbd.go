// Package common provides utility functions for NBD (Network Block Device) operations.
package common

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	MaxNBDDevices        = 8
	MaxPartitionsPerNBD  = 4
)

// nbdMutex protects NBD device allocation to prevent race conditions
// when multiple goroutines try to get and connect NBD devices concurrently.
var nbdMutex sync.Mutex

// MountQCOW2Image loads, connects, finds, and mounts a QCOW2 or VHD image.
func MountQCOW2Image(imageFile string) (mountDir, partition string, err error) {
	if _, err := os.Stat(imageFile); os.IsNotExist(err) {
		return "", "", fmt.Errorf("image file not found: %s", imageFile)
	}
	if err := LoadNBDModule(); err != nil {
		return "", "", fmt.Errorf("failed to load NBD module: %w", err)
	}
	
	// Lock to prevent race condition when multiple goroutines try to
	// get and connect NBD devices concurrently
	nbdMutex.Lock()
	nbdDevice, err := GetFreeNBDDevice()
	if err != nil {
		nbdMutex.Unlock()
		return "", "", fmt.Errorf("failed to get free NBD device: %w", err)
	}
	if err := ConnectImageToNBD(imageFile, nbdDevice); err != nil {
		nbdMutex.Unlock()
		return "", "", err
	}
	nbdMutex.Unlock()
	
	targetPartition, err := FindMountablePartition(nbdDevice)
	if err != nil {
		DisconnectNBD(nbdDevice)
		return "", "", fmt.Errorf("failed to find partition: %w", err)
	}
	mountDir, err = os.MkdirTemp("", "kopru-mount-*")
	if err != nil {
		DisconnectNBD(nbdDevice)
		return "", "", fmt.Errorf("failed to create mount directory: %w", err)
	}
	if err := MountPartition(targetPartition, mountDir); err != nil {
		os.RemoveAll(mountDir)
		DisconnectNBD(nbdDevice)
		return "", "", fmt.Errorf("failed to mount partition: %w", err)
	}
	return mountDir, targetPartition, nil
}

// FindMountablePartition returns the first mountable partition or device.
func FindMountablePartition(nbdDevice string) (string, error) {
	var partitions []string
	for i := 1; i <= MaxPartitionsPerNBD; i++ {
		partitions = append(partitions, fmt.Sprintf("%sp%d", nbdDevice, i))
		partitions = append(partitions, fmt.Sprintf("%s%d", nbdDevice, i))
	}
	partitions = append(partitions, nbdDevice)
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
func CleanupNBDMount(mountPoint string) error {
	var lastErr error
	var nbdDevice string
	if mountPoint != "" {
		if _, err := os.Stat(mountPoint); err == nil {
			nbdDevice = getNBDDeviceFromMountPoint(mountPoint)
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
	} else {
		for i := 0; i < MaxNBDDevices; i++ {
			dev := fmt.Sprintf("/dev/nbd%d", i)
			if isNBDDeviceConnected(dev) {
				if err := DisconnectNBD(dev); err != nil {
					lastErr = err
				}
			}
		}
	}
	return lastErr
}

// GetFreeNBDDevice finds and returns the first available NBD device.
func GetFreeNBDDevice() (string, error) {
	for i := 0; i < MaxNBDDevices; i++ {
		nbdDevice := fmt.Sprintf("/dev/nbd%d", i)
		if _, err := os.Stat(nbdDevice); os.IsNotExist(err) {
			continue
		}
		if !isNBDDeviceConnected(nbdDevice) {
			return nbdDevice, nil
		}
	}
	return "", fmt.Errorf("no free NBD device found (checked %d devices)", MaxNBDDevices)
}

// getNBDDeviceFromMountPoint returns the NBD device that is mounted at the given mount point.
func getNBDDeviceFromMountPoint(mountPoint string) string {
	cmd := exec.Command("findmnt", "-n", "-o", "SOURCE", mountPoint)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	source := strings.TrimSpace(string(output))
	if strings.HasPrefix(source, "/dev/nbd") {
		for i := 0; i < MaxNBDDevices; i++ {
			dev := fmt.Sprintf("/dev/nbd%d", i)
			if strings.HasPrefix(source, dev) {
				return dev
			}
		}
	}
	return ""
}

// isNBDDeviceConnected checks if an NBD device is currently connected.
func isNBDDeviceConnected(nbdDevice string) bool {
	cmd := exec.Command("sudo", "blockdev", "--getsize64", nbdDevice)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) != "0"
}

// LoadNBDModule loads the NBD kernel module with specified parameters.
func LoadNBDModule() error {
	if IsNBDModuleLoaded() {
		return nil
	}
	cmd := exec.Command("sudo", "modprobe", "nbd",
		fmt.Sprintf("max_part=%d", MaxPartitionsPerNBD),
		fmt.Sprintf("nbds_max=%d", MaxNBDDevices))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to load NBD module: %w", err)
	}
	time.Sleep(1 * time.Second)
	return nil
}

// IsNBDModuleLoaded returns true if the NBD module is loaded by checking lsmod output.
func IsNBDModuleLoaded() bool {
	cmd := exec.Command("lsmod")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "nbd" {
			return true
		}
	}
	return false
}

// ConnectImageToNBD connects a QCOW2 or VHD file to an NBD device.
func ConnectImageToNBD(imageFile, nbdDevice string) error {
	lowerImageFile := strings.ToLower(imageFile)
	var format string
	switch {
	case strings.HasSuffix(lowerImageFile, ".qcow2"):
		format = "qcow2"
	case strings.HasSuffix(lowerImageFile, ".vhd"):
		format = "raw"
	default:
		return fmt.Errorf("unsupported image format: %s (expected .qcow2 or .vhd)", imageFile)
	}
	cmd := exec.Command("sudo", "qemu-nbd", "--connect="+nbdDevice, imageFile, "-f", format)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to connect image to NBD: %w", err)
	}
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

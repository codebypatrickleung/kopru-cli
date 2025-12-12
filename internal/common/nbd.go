// Package common provides utility functions for NBD (Network Block Device) operations.
package common

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	MaxNBDDevices       = 8
	MaxPartitionsPerNBD = 4
	MaxMountRetries     = 3
	RetryDelaySeconds   = 2
)

var nbdMutex sync.Mutex

func MountQCOW2Image(imageFile string) (mountDir, partition string, err error) {
	if _, err := os.Stat(imageFile); os.IsNotExist(err) {
		return "", "", fmt.Errorf("image file not found: %s", imageFile)
	}
	if err := LoadNBDModule(); err != nil {
		return "", "", fmt.Errorf("failed to load NBD module: %w", err)
	}

	attemptedDevices := make(map[string]bool)
	for deviceIndex := 0; deviceIndex < MaxNBDDevices; deviceIndex++ {
		nbdMutex.Lock()
		nbdDevice, err := GetFreeNBDDevice()
		if err != nil {
			nbdMutex.Unlock()
			return "", "", fmt.Errorf("failed to get free NBD device: %w", err)
		}
		if attemptedDevices[nbdDevice] {
			nbdMutex.Unlock()
			continue
		}
		attemptedDevices[nbdDevice] = true

		if err := ConnectImageToNBD(imageFile, nbdDevice); err != nil {
			nbdMutex.Unlock()
			continue
		}
		nbdMutex.Unlock()

		var mountSucceeded bool
		var targetPartition, mountDir string

		for retry := 0; retry < MaxMountRetries; retry++ {
			if retry > 0 {
				time.Sleep(time.Duration(RetryDelaySeconds) * time.Second)
			}
			targetPartition, err = FindMountablePartition(nbdDevice)
			if err != nil {
				continue
			}
			mountDir, err = os.MkdirTemp("", "kopru-mount-*")
			if err != nil {
				continue
			}
			if err := MountPartition(targetPartition, mountDir); err != nil {
				os.RemoveAll(mountDir)
				mountDir = ""
				continue
			}
			mountSucceeded = true
			break
		}

		if mountSucceeded {
			return mountDir, targetPartition, nil
		}
		DisconnectNBD(nbdDevice)
	}
	return "", "", fmt.Errorf("failed to mount image after trying multiple NBD devices and retries")
}

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

func isNBDDeviceConnected(nbdDevice string) bool {
	cmd := exec.Command("sudo", "blockdev", "--getsize64", nbdDevice)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) != "0"
}

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
	for i := 0; i < 20; i++ {
		if IsNBDModuleLoaded() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("NBD module did not load after modprobe")
}

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
	return pollNBDDeviceReady(nbdDevice)
}

func pollNBDDeviceReady(nbdDevice string) error {
	for i := 0; i < 30; i++ {
		if isNBDDeviceConnected(nbdDevice) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("NBD device %s did not become ready after connect", nbdDevice)
}

func DisconnectNBD(nbdDevice string) error {
	cmd := exec.Command("sudo", "qemu-nbd", "--disconnect", nbdDevice)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to disconnect NBD: %w", err)
	}
	return nil
}

func MountPartition(partition, mountPoint string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sudo", "mount", partition, mountPoint)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to mount partition: %w", err)
	}
	return nil
}

func UnmountPartition(mountPoint string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sudo", "umount", mountPoint)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to unmount partition: %w", err)
	}
	return nil
}

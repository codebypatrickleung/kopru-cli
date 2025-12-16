// Package common provides utility functions for NBD (Network Block Device) operations.
package common

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	MaxNBDDevices       = 8
	MaxPartitionsPerNBD = 4
	MaxMountRetries     = 3
	RetryDelaySeconds   = 2
)

func MountQCOW2Image(imageFile string) (mountDir, partition string, err error) {
	if _, err := os.Stat(imageFile); os.IsNotExist(err) {
		return "", "", fmt.Errorf("image file not found: %s", imageFile)
	}
	if err := LoadNBDModule(); err != nil {
		return "", "", fmt.Errorf("failed to load NBD module: %w", err)
	}

	attemptedDevices := make(map[string]bool)
	for deviceIndex := 0; deviceIndex < MaxNBDDevices; deviceIndex++ {
		nbdDevice, err := GetFreeNBDDevice()
		if err != nil {
			return "", "", fmt.Errorf("failed to get free NBD device: %w", err)
		}
		if attemptedDevices[nbdDevice] {
			continue
		}
		attemptedDevices[nbdDevice] = true

		if err := ConnectImageToNBD(imageFile, nbdDevice); err != nil {
			continue
		}

		_ = ScanAndActivateLVM(nbdDevice)

		var (
			mountSucceeded  bool
			targetPartition string
			mountDir        string
		)

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
			_ = MountAdditionalLVMVolumes(nbdDevice, mountDir)
			return mountDir, targetPartition, nil
		}
		_ = DeactivateLVM(nbdDevice)
		_ = DisconnectNBD(nbdDevice)
	}
	return "", "", fmt.Errorf("failed to mount image after trying multiple NBD devices and retries")
}

func FindMountablePartition(nbdDevice string) (string, error) {
	lvmVolumes := GetLVMVolumesForNBD(nbdDevice)
	if len(lvmVolumes) > 0 {
		rootLikeNames := []string{"root", "rootlv", "lv_root"}
		for _, rootName := range rootLikeNames {
			for _, lv := range lvmVolumes {
				lvLower := strings.ToLower(filepath.Base(lv))
				if strings.Contains(lvLower, rootName) && HasFilesystem(lv) {
					return lv, nil
				}
			}
		}
		for _, lv := range lvmVolumes {
			if HasFilesystem(lv) {
				return lv, nil
			}
		}
	}

	var partitions []string
	for i := 1; i <= MaxPartitionsPerNBD; i++ {
		partitions = append(partitions, fmt.Sprintf("%sp%d", nbdDevice, i))
		partitions = append(partitions, fmt.Sprintf("%s%d", nbdDevice, i))
	}
	partitions = append(partitions, nbdDevice)
	for _, partition := range partitions {
		if _, err := os.Stat(partition); err == nil && HasFilesystem(partition) {
			return partition, nil
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
			if err := UnmountAdditionalLVMVolumes(mountPoint); err != nil {
				lastErr = err
			}
			if err := UnmountPartition(mountPoint); err != nil {
				lastErr = err
			}
		}
		_ = os.RemoveAll(mountPoint)
	}
	if nbdDevice != "" {
		if err := DeactivateLVM(nbdDevice); err != nil {
			lastErr = err
		}
		if err := DisconnectNBD(nbdDevice); err != nil {
			lastErr = err
		}
	} else {
		for i := 0; i < MaxNBDDevices; i++ {
			dev := fmt.Sprintf("/dev/nbd%d", i)
			if isNBDDeviceConnected(dev) {
				if err := DeactivateLVM(dev); err != nil {
					lastErr = err
				}
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
	cmd := exec.Command("sudo", "mount", partition, mountPoint)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to mount partition: %w", err)
	}
	return nil
}

func UnmountPartition(mountPoint string) error {
	cmd := exec.Command("sudo", "umount", mountPoint)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to unmount partition: %w", err)
	}
	return nil
}

// MountAdditionalLVMVolumes mounts additional LVM logical volumes for RHEL-based systems.
func MountAdditionalLVMVolumes(nbdDevice, mountDir string) error {
	lvmVolumes := GetLVMVolumesForNBD(nbdDevice)
	if len(lvmVolumes) == 0 {
		return nil
	}

	lvMappings := map[string][]string{
		"usr":  {"usrlv", "usr-lv", "usr_lv", "lv-usr", "lv_usr"},
		"home": {"homelv", "home-lv", "home_lv", "lv-home", "lv_home"},
		"var":  {"varlv", "var-lv", "var_lv", "lv-var", "lv_var"},
		"tmp":  {"tmplv", "tmp-lv", "tmp_lv", "lv-tmp", "lv_tmp"},
	}

	for _, lv := range lvmVolumes {
		lvName := strings.ToLower(filepath.Base(lv))
		for targetPath, patterns := range lvMappings {
			for _, pattern := range patterns {
				if lvName == pattern {
					targetDir := filepath.Join(mountDir, targetPath)
					if err := os.MkdirAll(targetDir, 0755); err != nil {
						break
					}
					if err := MountPartition(lv, targetDir); err != nil {
						break
					}
					break
				}
			}
		}
	}
	return nil
}

// UnmountAdditionalLVMVolumes unmounts additional LVM logical volumes that were mounted.
func UnmountAdditionalLVMVolumes(mountDir string) error {
	subMounts := []string{"tmp", "var", "home", "usr"}
	for _, subMount := range subMounts {
		subMountPath := filepath.Join(mountDir, subMount)
		if _, err := os.Stat(subMountPath); err == nil {
			cmd := exec.Command("mountpoint", "-q", subMountPath)
			if cmd.Run() == nil {
				_ = UnmountPartition(subMountPath)
			}
		}
	}
	return nil
}

// ScanAndActivateLVM scans for and activates LVM logical volumes for the given NBD device.
func ScanAndActivateLVM(nbdDevice string) error {
	time.Sleep(500 * time.Millisecond)
	
	_ = exec.Command("sudo", "pvscan").Run()
	_ = exec.Command("sudo", "vgscan", "--mknodes").Run()
	_ = exec.Command("sudo", "vgchange", "-ay").Run()
	
	time.Sleep(500 * time.Millisecond)
	return nil
}

// DeactivateLVM deactivates LVM logical volumes associated with an NBD device.
func DeactivateLVM(nbdDevice string) error {
	cmd := exec.Command("sudo", "pvs", "--noheadings", "-o", "pv_name,vg_name")
	pvOutput, err := cmd.Output()
	if err != nil {
		return nil
	}
	vgsToDeactivate := make(map[string]bool)
	lines := strings.Split(strings.TrimSpace(string(pvOutput)), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			pvName := fields[0]
			if strings.HasPrefix(pvName, nbdDevice) {
				if pvName == nbdDevice || strings.HasPrefix(pvName, nbdDevice+"p") {
					vgsToDeactivate[fields[1]] = true
				}
			}
		}
	}
	for vgName := range vgsToDeactivate {
		_ = exec.Command("sudo", "vgchange", "-an", vgName).Run()
	}
	return nil
}

// GetLVMVolumesForNBD returns a list of LVM logical volume device paths for a given NBD device.
func GetLVMVolumesForNBD(nbdDevice string) []string {
	if nbdDevice == "" {
		cmd := exec.Command("sudo", "lvs", "--noheadings", "-o", "lv_path")
		output, err := cmd.Output()
		if err != nil {
			return nil
		}
		var volumes []string
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			lvPath := strings.TrimSpace(line)
			if lvPath != "" && strings.HasPrefix(lvPath, "/dev/") {
				volumes = append(volumes, lvPath)
			}
		}
		return volumes
	}

	cmd := exec.Command("sudo", "pvs", "--noheadings", "-o", "pv_name,vg_name")
	pvOutput, err := cmd.Output()
	if err != nil {
		return nil
	}

	vgsOnDevice := make(map[string]bool)
	lines := strings.Split(strings.TrimSpace(string(pvOutput)), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			pvName := fields[0]
			if strings.HasPrefix(pvName, nbdDevice) {
				if pvName == nbdDevice || strings.HasPrefix(pvName, nbdDevice+"p") {
					vgsOnDevice[fields[1]] = true
				}
			}
		}
	}

	if len(vgsOnDevice) == 0 {
		return nil
	}

	cmd = exec.Command("sudo", "lvs", "--noheadings", "-o", "lv_path,vg_name")
	lvOutput, err := cmd.Output()
	if err != nil {
		return nil
	}

	var volumes []string
	lines = strings.Split(strings.TrimSpace(string(lvOutput)), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			lvPath := fields[0]
			vgName := fields[1]
			if vgsOnDevice[vgName] && lvPath != "" && strings.HasPrefix(lvPath, "/dev/") {
				volumes = append(volumes, lvPath)
			}
		}
	}
	return volumes
}

// HasFilesystem returns true if blkid detects a filesystem on the device.
func HasFilesystem(device string) bool {
	output, err := exec.Command("sudo", "blkid", device).Output()
	return err == nil && len(output) > 0
}

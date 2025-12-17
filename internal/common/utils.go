// Package common provides utility functions used across the Kopru CLI.
package common

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/codebypatrickleung/kopru-cli/internal/logger"
	"golang.org/x/sys/unix"
)

const (
	OCIMinVolumeSizeGB = 50  // Minimum volume size in GB for OCI block volumes
	MinDiskSpaceGB     = 500 // Recommended minimum disk space in GB for migration operations
)

// IsWindowsOS checks if the given operating system string is exactly "Windows" (case-insensitive).
func IsWindowsOS(operatingSystem string) bool {
	return strings.EqualFold(strings.TrimSpace(operatingSystem), "windows")
}

// CheckCommand returns an error if the command is not found in PATH.
func CheckCommand(cmd string) error {
	if _, err := exec.LookPath(cmd); err != nil {
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
func FindDiskFile(dir, extension string) (string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*"+extension))
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no %s file found in %s", extension, dir)
	}
	return files[0], nil
}

// GetAvailableDiskSpace returns the available disk space in bytes for the given path.
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

// GetFileSizeGB returns the size of a file in gigabytes, rounded up and enforcing OCI minimum.
func GetFileSizeGB(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}
	sizeGB := (info.Size() + (1024*1024*1024 - 1)) / (1024 * 1024 * 1024)
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

// ListBlockDevices returns a list of block device names (without /dev/ prefix).
func ListBlockDevices() ([]string, error) {
	output, err := exec.Command("lsblk", "-dn", "-o", "NAME").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list block devices: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var devices []string
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
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

// ConvertVHDToQCOW2 converts a VHD file to QCOW2 format. The VHD file is always kept for auditing purposes.
func ConvertVHDToQCOW2(vhdFile, qcow2File string) error {
	if output, err := RunCommand("qemu-img", "convert", "-f", "vpc", "-O", "qcow2", vhdFile, qcow2File); err != nil {
		return fmt.Errorf("qemu-img convert failed: %w\nOutput: %s", err, output)
	}
	if output, err := RunCommand("qemu-img", "resize", qcow2File, "+5M"); err != nil {
		return fmt.Errorf("qemu-img resize failed: %w\nOutput: %s", err, output)
	}
	return nil
}

// ConvertVHDToRAW converts a VHD file to RAW format. The VHD file is always kept for auditing purposes.
func ConvertVHDToRAW(vhdFile, rawFile string) error {
	if vhdFile == "" {
		return fmt.Errorf("VHD file path cannot be empty")
	}
	if rawFile == "" {
		return fmt.Errorf("RAW file path cannot be empty")
	}
	if _, err := os.Stat(vhdFile); os.IsNotExist(err) {
		return fmt.Errorf("VHD file not found: %s", vhdFile)
	}
	if output, err := RunCommand("qemu-img", "convert", "-f", "vpc", "-O", "raw", vhdFile, rawFile); err != nil {
		return fmt.Errorf("qemu-img convert to RAW failed: %w\nOutput: %s", err, output)
	}
	return nil
}

// GetComputeOSDiskSizeGB reads the virtual size of a QCOW2 file and returns the size in GB.
func GetComputeOSDiskSizeGB(qcow2File string) (int64, error) {
	output, err := RunCommand("qemu-img", "info", qcow2File)
	if err != nil {
		return 0, fmt.Errorf("failed to get QCOW2 info: %w", err)
	}
	const bytesPerGB = 1024 * 1024 * 1024
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "virtual size:") {
			start := strings.Index(line, "(")
			end := strings.Index(line, "bytes)")
			if start != -1 && end != -1 && end > start {
				bytesStr := strings.TrimSpace(line[start+1 : end])
				bytesStr = strings.ReplaceAll(bytesStr, ",", "")
				var bytes int64
				if _, err := fmt.Sscanf(bytesStr, "%d", &bytes); err != nil {
					return 0, fmt.Errorf("failed to parse virtual size bytes: %w", err)
				}
				sizeGB := (bytes + bytesPerGB - 1) / bytesPerGB
				return sizeGB, nil
			}
		}
	}
	return 0, fmt.Errorf("virtual size not found in qemu-img output")
}

// ExecuteOSConfigScript executes an OS configuration script from the scripts/os-config directory.
func ExecuteOSConfigScript(mountDir, osType, sourcePlatform string, log *logger.Logger) error {
	if sourcePlatform == "azure" && IsLinuxOS(osType) {
		return executeScript(mountDir, "generic_linux_azure_to_oci.sh", log, true)
	}
	log.Infof("Skipping OS configuration for OS type '%s'", osType)
	return nil
}

// IsLinuxOS checks if the given operating system string is a Linux-based OS.
func IsLinuxOS(operatingSystem string) bool {
	osLower := strings.ToLower(strings.TrimSpace(operatingSystem))
	linuxOSTypes := []string{
		"ubuntu", "rhel", "centos", "almalinux", "rocky linux",
		"oracle linux", "debian", "suse", "generic linux",
	}
	for _, linuxOS := range linuxOSTypes {
		if osLower == linuxOS {
			return true
		}
	}
	return false
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

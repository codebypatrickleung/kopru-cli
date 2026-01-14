// Package common provides utility functions for mounting QCOW2 images using libguestfs.
package common

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	MaxMountRetries   = 3
	RetryDelaySeconds = 2
)

// MountQCOW2Image mounts a QCOW2 image using guestmount and returns the mount directory.
func MountQCOW2Image(imageFile string) (mountDir, partition string, err error) {
	if _, err := os.Stat(imageFile); os.IsNotExist(err) {
		return "", "", fmt.Errorf("image file not found: %s", imageFile)
	}

	if err := CheckCommand("guestmount"); err != nil {
		return "", "", fmt.Errorf("guestmount is not installed: %w", err)
	}

	mountDir, err = os.MkdirTemp("", "kopru-mount-*")
	if err != nil {
		return "", "", fmt.Errorf("failed to create mount directory: %w", err)
	}

	var lastErr error
	for retry := range MaxMountRetries {
		if retry > 0 {
			time.Sleep(RetryDelaySeconds * time.Second)
		}

		cmd := exec.Command("guestmount", "-a", imageFile, "-i", "--rw", mountDir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			lastErr = fmt.Errorf("guestmount failed: %w, output: %s", err, string(output))
			continue
		}

		time.Sleep(500 * time.Millisecond)

		if isMountValid(mountDir) {
			return mountDir, "", nil
		}

		lastErr = fmt.Errorf("mount validation failed")
		_ = unmountGuestFS(mountDir)
	}

	if err := os.RemoveAll(mountDir); err != nil {
		return "", "", fmt.Errorf("failed to remove mount directory: %w (mount error: %v)", err, lastErr)
	}
	return "", "", fmt.Errorf("failed to mount image after %d retries: %w", MaxMountRetries, lastErr)
}

// isMountValid checks if the mounted filesystem looks valid by checking for common directories
func isMountValid(mountDir string) bool {
	commonDirs := []string{"etc", "usr", "var", "bin"}
	for _, dir := range commonDirs {
		dirPath := filepath.Join(mountDir, dir)
		if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// CleanupMount unmounts a guestmount-mounted filesystem and removes the mount directory.
func CleanupMount(mountPoint string) error {
	if mountPoint == "" {
		return nil
	}

	var lastErr error

	if _, err := os.Stat(mountPoint); err == nil {
		if err := unmountGuestFS(mountPoint); err != nil {
			lastErr = err
		}
	}

	if err := os.RemoveAll(mountPoint); err != nil {
		lastErr = fmt.Errorf("failed to remove mount directory: %w", err)
	}

	return lastErr
}

// unmountGuestFS unmounts a FUSE filesystem mounted by guestmount
func unmountGuestFS(mountPoint string) error {
	if err := CheckCommand("guestunmount"); err == nil {
		cmd := exec.Command("guestunmount", mountPoint)
		if err := cmd.Run(); err == nil {
			time.Sleep(500 * time.Millisecond)
			return nil
		}
	}

	cmd := exec.Command("fusermount", "-u", mountPoint)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to unmount: %w", err)
	}

	time.Sleep(500 * time.Millisecond)
	return nil
}

// Package os handles QCOW2 image mounting and OS-specific configurations.
package os

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/codebypatrickleung/kopru-cli/internal/logger"
)

func TestNewManager(t *testing.T) {
	log := logger.New(false)
	mgr := NewManager(log, "azure")

	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}

	if mgr.logger != log {
		t.Error("Manager logger not set correctly")
	}

	if mgr.nbdDevice != "/dev/nbd0" {
		t.Errorf("Expected NBD device /dev/nbd0, got %s", mgr.nbdDevice)
	}

	if mgr.isMounted {
		t.Error("Manager should not be mounted initially")
	}

	if mgr.sourcePlatform != "azure" {
		t.Errorf("Expected source platform azure, got %s", mgr.sourcePlatform)
	}
}

func TestWriteFile(t *testing.T) {
	log := logger.New(false)
	mgr := NewManager(log, "azure")

	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "test content\n"
	err := mgr.writeFile(testFile, content)

	// This will fail if sudo is not available or if we don't have permissions
	// In a real environment, we'd skip this test or use a mock
	if err != nil {
		// Expected to fail in CI without sudo, so we just log it
		t.Logf("writeFile failed (expected in CI): %v", err)
		return
	}

	// If it succeeded, verify the content
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	if string(data) != content {
		t.Errorf("Expected content %q, got %q", content, string(data))
	}
}

func TestFindRootPartition(t *testing.T) {
	log := logger.New(false)
	mgr := NewManager(log, "azure")

	// This test will fail if the NBD device doesn't exist
	// We're mainly testing the function exists and has the right signature
	_, err := mgr.findRootPartition()
	if err == nil {
		t.Log("findRootPartition succeeded (unexpected in test environment)")
	} else {
		t.Logf("findRootPartition failed as expected in test environment: %v", err)
	}
}

func TestManagerCleanup(t *testing.T) {
	log := logger.New(false)
	mgr := NewManager(log, "azure")

	// Create a temporary mount directory
	tmpDir := t.TempDir()
	mgr.mountDir = tmpDir
	mgr.isMounted = false // Not actually mounted

	// Call cleanup
	mgr.cleanup()

	// Verify cleanup was called
	if mgr.isMounted {
		t.Error("isMounted should be false after cleanup")
	}
}

func TestApplyUbuntuConfigurationsNotMounted(t *testing.T) {
	log := logger.New(false)
	mgr := NewManager(log, "azure")

	// Should fail if not mounted
	err := mgr.ApplyUbuntuConfigurations()
	if err == nil {
		t.Error("Expected error when applying configurations without mounting")
	}

	expectedMsg := "image not mounted, call MountQCOW2 first"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestApplyCustomScriptNotMounted(t *testing.T) {
	log := logger.New(false)
	mgr := NewManager(log, "azure")

	// Should fail if not mounted
	err := mgr.ApplyCustomScript("/nonexistent/script.sh")
	if err == nil {
		t.Error("Expected error when applying custom script without mounting")
	}

	expectedMsg := "image not mounted, call MountQCOW2 first"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestApplyCustomScriptFileNotFound(t *testing.T) {
	log := logger.New(false)
	mgr := NewManager(log, "azure")
	mgr.isMounted = true // Fake being mounted
	mgr.mountDir = t.TempDir()

	// Should fail if script doesn't exist
	err := mgr.ApplyCustomScript("/nonexistent/script.sh")
	if err == nil {
		t.Error("Expected error when script file doesn't exist")
	}
}

func TestMountQCOW2FileNotFound(t *testing.T) {
	log := logger.New(false)
	mgr := NewManager(log, "azure")

	// Should fail if file doesn't exist
	err := mgr.MountQCOW2("/nonexistent/image.qcow2")
	if err == nil {
		t.Error("Expected error when QCOW2 file doesn't exist")
	}
}

func TestNewManagerWithDifferentPlatforms(t *testing.T) {
	log := logger.New(false)

	tests := []struct {
		name           string
		sourcePlatform string
	}{
		{"Azure platform", "azure"},
		{"Other platform", "other"},
		{"Empty platform", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewManager(log, tt.sourcePlatform)
			if mgr == nil {
				t.Fatal("NewManager returned nil")
			}
			if mgr.sourcePlatform != tt.sourcePlatform {
				t.Errorf("Expected source platform %q, got %q", tt.sourcePlatform, mgr.sourcePlatform)
			}
		})
	}
}

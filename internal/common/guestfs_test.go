package common

import (
	"os"
	"testing"
)

func TestMaxMountRetriesConstant(t *testing.T) {
	const want = 3
	if MaxMountRetries != want {
		t.Errorf("MaxMountRetries = %d; want %d", MaxMountRetries, want)
	}
}

func TestRetryDelaySecondsConstant(t *testing.T) {
	const want = 2
	if RetryDelaySeconds != want {
		t.Errorf("RetryDelaySeconds = %d; want %d", RetryDelaySeconds, want)
	}
}

func TestMountQCOW2ImageNonExistentFile(t *testing.T) {
	_, _, err := MountQCOW2Image("/non/existent/image.qcow2")
	if err == nil {
		t.Error("Expected error for non-existent image file, got nil")
	}
	wantErr := "image file not found: /non/existent/image.qcow2"
	if err != nil && err.Error() != wantErr {
		t.Errorf("Expected error %q, got: %q", wantErr, err.Error())
	}
}

func TestCleanupMountEmptyPath(t *testing.T) {
	if err := CleanupMount(""); err != nil {
		t.Errorf("CleanupMount(\"\") returned error: %v", err)
	}
}

func TestCleanupMountNonExistentPath(t *testing.T) {
	err := CleanupMount("/tmp/kopru-test-nonexistent-mount-12345")
	if err != nil {
		t.Logf("CleanupMount returned error for non-existent path (acceptable): %v", err)
	}
}

func TestIsMountValidWithInvalidPath(t *testing.T) {
	if isMountValid("/tmp/kopru-test-invalid-mount-12345") {
		t.Error("Expected false for invalid mount path")
	}
}

func TestIsMountValidWithTempDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "kopru-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if isMountValid(tmpDir) {
		t.Error("Expected false for empty temp directory")
	}
}

func TestIsMountValidWithEtcDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "kopru-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	etcDir := tmpDir + "/etc"
	if err := os.MkdirAll(etcDir, 0755); err != nil {
		t.Fatalf("Failed to create etc directory: %v", err)
	}

	if !isMountValid(tmpDir) {
		t.Error("Expected true when etc directory exists")
	}
}

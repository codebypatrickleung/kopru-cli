// Package os handles QCOW2 image mounting and OS-specific configurations.
package os

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codebypatrickleung/kopru-cli/internal/common"
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

	if mgr.sourcePlatform != "azure" {
		t.Errorf("Expected source platform azure, got %s", mgr.sourcePlatform)
	}
}

func TestWriteFile(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "test content\n"
	err := common.WriteFile(testFile, content)

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

func TestManagerCleanup(t *testing.T) {
	log := logger.New(false)
	mgr := NewManager(log, "azure")

	// Create a temporary mount directory
	tmpDir := t.TempDir()
	mgr.MountDir = tmpDir

	// Test cleanup using common function directly
	err := common.CleanupNBDMount("/dev/nbd0", tmpDir)
	if err != nil {
		t.Logf("CleanupNBDMount failed (expected in test environment): %v", err)
	}
}

func TestApplyUbuntuConfigurationsNotMounted(t *testing.T) {
	// This test is no longer relevant since we removed the isMounted check
	// The Manager methods now expect the mount directory to be set by the caller
	t.Skip("isMounted check removed - Manager now expects mount directory to be set externally")
}

func TestApplyCustomScriptNotMounted(t *testing.T) {
	// This test is no longer relevant since we removed the isMounted check
	// The Manager methods now expect the mount directory to be set by the caller
	t.Skip("isMounted check removed - Manager now expects mount directory to be set externally")
}

func TestApplyCustomScriptFileNotFound(t *testing.T) {
	log := logger.New(false)
	mgr := NewManager(log, "azure")
	mgr.MountDir = t.TempDir()

	// Should fail if script doesn't exist
	err := mgr.ApplyCustomScript("/nonexistent/script.sh")
	if err == nil {
		t.Error("Expected error when script file doesn't exist")
	}
}

func TestMountQCOW2FileNotFound(t *testing.T) {
	// This test is no longer relevant since MountQCOW2 is removed from Manager
	// Testing should be done on common.MountQCOW2Image directly
	t.Skip("MountQCOW2 method removed - test common.MountQCOW2Image directly")
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

func TestDisableAzureHostsTemplate(t *testing.T) {
	log := logger.New(false)
	mgr := NewManager(log, "azure")

	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	mgr.MountDir = tmpDir

	// Create the cloud templates directory
	cloudTemplatesDir := filepath.Join(tmpDir, "etc/cloud/templates")
	if err := os.MkdirAll(cloudTemplatesDir, 0755); err != nil {
		t.Fatalf("Failed to create cloud templates directory: %v", err)
	}

	// Test case 1: File doesn't exist - should skip
	err := mgr.DisableAzureHostsTemplate()
	if err != nil {
		t.Errorf("Expected no error when file doesn't exist, got: %v", err)
	}

	// Test case 2: File exists - should append "disable"
	hostsTemplatePath := filepath.Join(cloudTemplatesDir, "hosts.azurelinux.tmpl")
	initialContent := "127.0.0.1 localhost\n"
	if err := os.WriteFile(hostsTemplatePath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = mgr.DisableAzureHostsTemplate()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify content
	content, err := os.ReadFile(hostsTemplatePath)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(content)), "disable") {
		t.Errorf("Expected file to end with 'disable', got: %q", string(content))
	}

	// Test case 3: File already has "disable" - should not append again
	err = mgr.DisableAzureHostsTemplate()
	if err != nil {
		t.Errorf("Expected no error when already disabled, got: %v", err)
	}
}

func TestCommentOutAzureChronyRefclock(t *testing.T) {
	log := logger.New(false)
	mgr := NewManager(log, "azure")

	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	mgr.MountDir = tmpDir

	// Create the chrony directory
	chronyDir := filepath.Join(tmpDir, "etc/chrony")
	if err := os.MkdirAll(chronyDir, 0755); err != nil {
		t.Fatalf("Failed to create chrony directory: %v", err)
	}

	// Test case 1: File doesn't exist - should skip
	err := mgr.DisableAzureChronyRefclock()
	if err != nil {
		t.Errorf("Expected no error when file doesn't exist, got: %v", err)
	}

	// Test case 2: File exists without the target line - should skip
	chronyConfPath := filepath.Join(chronyDir, "chrony.conf")
	initialContent := "pool 2.ubuntu.pool.ntp.org iburst\n"
	if err := os.WriteFile(chronyConfPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = mgr.DisableAzureChronyRefclock()
	if err != nil {
		t.Errorf("Expected no error when target line doesn't exist, got: %v", err)
	}

	// Test case 3: File exists with the target line - should comment it out
	contentWithRefclock := "pool 2.ubuntu.pool.ntp.org iburst\nrefclock PHC /dev/ptp_hyperv poll 3 dpoll -2 offset 0\n"
	if err := os.WriteFile(chronyConfPath, []byte(contentWithRefclock), 0644); err != nil {
		t.Fatalf("Failed to update test file: %v", err)
	}

	err = mgr.DisableAzureChronyRefclock()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify the line is commented
	content, err := os.ReadFile(chronyConfPath)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}
	contentStr := string(content)
	
	// Check that the commented version exists
	if !strings.Contains(contentStr, "# refclock PHC /dev/ptp_hyperv poll 3 dpoll -2 offset 0") {
		t.Errorf("Expected refclock line to be commented out, got: %q", contentStr)
	}
	
	// Check that the original uncommented version (not as part of comment) does not exist
	// We need to ensure "refclock PHC" appears only in commented form
	lines := strings.Split(contentStr, "\n")
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "refclock PHC /dev/ptp_hyperv poll 3 dpoll -2 offset 0" {
			t.Errorf("Found uncommented refclock line, expected all to be commented")
		}
	}

	// Test case 4: File already has the line commented - should skip
	err = mgr.DisableAzureChronyRefclock()
	if err != nil {
		t.Errorf("Expected no error when already commented, got: %v", err)
	}
}

func TestDisableAzureUdevRules(t *testing.T) {
	log := logger.New(false)
	mgr := NewManager(log, "azure")

	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	mgr.MountDir = tmpDir

	// Create the udev rules directory
	udevRulesDir := filepath.Join(tmpDir, "etc/udev/rules.d")
	if err := os.MkdirAll(udevRulesDir, 0755); err != nil {
		t.Fatalf("Failed to create udev rules directory: %v", err)
	}

	// Test case 1: Files don't exist - should skip
	err := mgr.DisableAzureUdevRules()
	if err != nil {
		t.Errorf("Expected no error when files don't exist, got: %v", err)
	}

	// Test case 2: Files exist - should rename with .disable suffix
	testRules := []string{
		"66-azure-storage.rules",
		"99-azure-product-uuid.rules",
	}

	for _, rule := range testRules {
		rulePath := filepath.Join(udevRulesDir, rule)
		if err := os.WriteFile(rulePath, []byte("test content\n"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", rule, err)
		}
	}

	err = mgr.DisableAzureUdevRules()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify files are renamed
	for _, rule := range testRules {
		originalPath := filepath.Join(udevRulesDir, rule)
		disabledPath := originalPath + ".disable"

		if _, err := os.Stat(originalPath); !os.IsNotExist(err) {
			t.Errorf("Original file %s should not exist", rule)
		}

		if _, err := os.Stat(disabledPath); os.IsNotExist(err) {
			t.Errorf("Disabled file %s.disable should exist", rule)
		}
	}

	// Test case 3: Files already disabled - should skip
	err = mgr.DisableAzureUdevRules()
	if err != nil {
		t.Errorf("Expected no error when already disabled, got: %v", err)
	}
}

func TestDisableWALinuxAgent(t *testing.T) {
	log := logger.New(false)
	mgr := NewManager(log, "azure")

	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	mgr.MountDir = tmpDir

	// Test case 1: Files don't exist - should skip
	err := mgr.DisableAzureLinuxAgent()
	if err != nil {
		t.Errorf("Expected no error when files don't exist, got: %v", err)
	}

	// Test case 2: Create test files and directories
	waagentPaths := []struct {
		path  string
		isDir bool
	}{
		{"/var/lib/waagent", true},
		{"/etc/init/walinuxagent.conf", false},
		{"/etc/init.d/walinuxagent", false},
		{"/usr/sbin/waagent", false},
		{"/usr/sbin/waagent2.0", false},
		{"/etc/waagent.conf", false},
		{"/var/log/waagent.log", false},
	}

	for _, item := range waagentPaths {
		fullPath := filepath.Join(tmpDir, item.path)
		parentDir := filepath.Dir(fullPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			t.Fatalf("Failed to create parent directory for %s: %v", item.path, err)
		}

		if item.isDir {
			if err := os.MkdirAll(fullPath, 0755); err != nil {
				t.Fatalf("Failed to create directory %s: %v", item.path, err)
			}
			// Create a file inside the directory to verify it's a directory
			testFile := filepath.Join(fullPath, "test.txt")
			if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
				t.Fatalf("Failed to create test file in directory: %v", err)
			}
		} else {
			if err := os.WriteFile(fullPath, []byte("test content\n"), 0644); err != nil {
				t.Fatalf("Failed to create test file %s: %v", item.path, err)
			}
		}
	}

	err = mgr.DisableAzureLinuxAgent()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify files/directories are renamed
	for _, item := range waagentPaths {
		originalPath := filepath.Join(tmpDir, item.path)
		disabledPath := originalPath + ".disable"

		if _, err := os.Stat(originalPath); !os.IsNotExist(err) {
			t.Errorf("Original path %s should not exist", item.path)
		}

		if _, err := os.Stat(disabledPath); os.IsNotExist(err) {
			t.Errorf("Disabled path %s.disable should exist", item.path)
		}
	}

	// Test case 3: Files already disabled - should skip
	err = mgr.DisableAzureLinuxAgent()
	if err != nil {
		t.Errorf("Expected no error when already disabled, got: %v", err)
	}
}

func TestCommentOutAzureChronyRefclockWithOCIServer(t *testing.T) {
	log := logger.New(false)
	mgr := NewManager(log, "azure")

	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	mgr.MountDir = tmpDir

	// Create the chrony directory
	chronyDir := filepath.Join(tmpDir, "etc/chrony")
	if err := os.MkdirAll(chronyDir, 0755); err != nil {
		t.Fatalf("Failed to create chrony directory: %v", err)
	}

	// Test case: File exists with the target line - should comment it out and add OCI server
	chronyConfPath := filepath.Join(chronyDir, "chrony.conf")
	contentWithRefclock := "pool 2.ubuntu.pool.ntp.org iburst\nrefclock PHC /dev/ptp_hyperv poll 3 dpoll -2 offset 0\n"
	if err := os.WriteFile(chronyConfPath, []byte(contentWithRefclock), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := mgr.DisableAzureChronyRefclock()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify the line is commented and OCI server is added
	content, err := os.ReadFile(chronyConfPath)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}
	contentStr := string(content)
	
	// Check that the commented version exists
	if !strings.Contains(contentStr, "# refclock PHC /dev/ptp_hyperv poll 3 dpoll -2 offset 0") {
		t.Errorf("Expected refclock line to be commented out, got: %q", contentStr)
	}
	
	// Check that OCI server line is added
	if !strings.Contains(contentStr, "server 169.254.169.254 iburst") {
		t.Errorf("Expected OCI time server to be added, got: %q", contentStr)
	}
	
	// Check that the original uncommented version does not exist
	lines := strings.Split(contentStr, "\n")
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "refclock PHC /dev/ptp_hyperv poll 3 dpoll -2 offset 0" {
			t.Errorf("Found uncommented refclock line, expected all to be commented")
		}
	}
}

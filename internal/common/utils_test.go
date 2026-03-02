// Package common provides utility functions used across the Kopru CLI.
package common

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestIsWindowsOS(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Windows exact", "Windows", true},
		{"Windows lowercase", "windows", true},
		{"Windows uppercase", "WINDOWS", true},
		{"Windows with spaces", "  Windows  ", true},
		{"Ubuntu", "Ubuntu", false},
		{"RHEL", "RHEL", false},
		{"Empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsWindowsOS(tt.input)
			if result != tt.expected {
				t.Errorf("IsWindowsOS(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsLinuxOS(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Ubuntu exact", "Ubuntu", true},
		{"Ubuntu lowercase", "ubuntu", true},
		{"Ubuntu uppercase", "UBUNTU", true},
		{"Ubuntu with spaces", "  Ubuntu  ", true},
		{"RHEL exact", "RHEL", true},
		{"RHEL lowercase", "rhel", true},
		{"CentOS exact", "CentOS", true},
		{"CentOS lowercase", "centos", true},
		{"AlmaLinux exact", "AlmaLinux", true},
		{"AlmaLinux lowercase", "almalinux", true},
		{"Rocky Linux exact", "Rocky Linux", true},
		{"Rocky Linux lowercase", "rocky linux", true},
		{"Oracle Linux exact", "Oracle Linux", true},
		{"Oracle Linux lowercase", "oracle linux", true},
		{"Debian exact", "Debian", true},
		{"Debian lowercase", "debian", true},
		{"SUSE exact", "SUSE", true},
		{"SUSE lowercase", "suse", true},
		{"Generic Linux exact", "Generic Linux", true},
		{"Generic Linux lowercase", "generic linux", true},
		{"Windows", "Windows", false},
		{"Empty string", "", false},
		{"Unknown OS", "FreeBSD", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsLinuxOS(tt.input)
			if result != tt.expected {
				t.Errorf("IsLinuxOS(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Simple name", "test-vm", "test-vm"},
		{"With spaces", "test vm", "test-vm"},
		{"With uppercase", "Test-VM", "test-vm"},
		{"With special chars", "test@vm#123", "testvm123"},
		{"With underscores", "test_vm_123", "test_vm_123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeName(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSliceDifference(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected []string
	}{
		{"New device added", []string{"sda", "sdb", "sdc"}, []string{"sda", "sdb"}, []string{"sdc"}},
		{"No new devices", []string{"sda", "sdb"}, []string{"sda", "sdb"}, nil},
		{"All new devices", []string{"sda", "sdb"}, []string{}, []string{"sda", "sdb"}},
		{"Empty a", []string{}, []string{"sda"}, nil},
		{"Both empty", []string{}, []string{}, nil},
		{"Device removed from a but not in b", []string{"sda", "sdc"}, []string{"sda", "sdb"}, []string{"sdc"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SliceDifference(tt.a, tt.b)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("SliceDifference(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestDataDiskDevicePath(t *testing.T) {
	tests := []struct {
		name     string
		index    int
		expected string
	}{
		{"Index 0 → vdb", 0, "/dev/oracleoci/oraclevdb"},
		{"Index 1 → vdc", 1, "/dev/oracleoci/oraclevdc"},
		{"Index 2 → vdd", 2, "/dev/oracleoci/oraclevdd"},
		{"Index 24 → vdz", 24, "/dev/oracleoci/oraclevdz"},
		{"Index 25 → vdaa", 25, "/dev/oracleoci/oraclevdaa"},
		{"Index 26 → vdab", 26, "/dev/oracleoci/oraclevdab"},
		{"Index 31 → vdag", 31, "/dev/oracleoci/oraclevdag"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DataDiskDevicePath(tt.index)
			if result != tt.expected {
				t.Errorf("DataDiskDevicePath(%d) = %q, want %q", tt.index, result, tt.expected)
			}
		})
	}
}

func TestWaitForDevice(t *testing.T) {
	t.Run("Device exists immediately", func(t *testing.T) {
		dir := t.TempDir()
		devicePath := filepath.Join(dir, "vdb")
		if err := os.WriteFile(devicePath, []byte{}, 0600); err != nil {
			t.Fatal(err)
		}
		got, err := WaitForDevice(devicePath)
		if err != nil {
			t.Fatalf("WaitForDevice(%q) returned unexpected error: %v", devicePath, err)
		}
		if got != devicePath {
			t.Errorf("WaitForDevice(%q) = %q, want %q", devicePath, got, devicePath)
		}
	})

	t.Run("Device appears after delay", func(t *testing.T) {
		dir := t.TempDir()
		devicePath := filepath.Join(dir, "vdc")
		go func() {
			time.Sleep(100 * time.Millisecond)
			_ = os.WriteFile(devicePath, []byte{}, 0600)
		}()
		got, err := WaitForDevice(devicePath)
		if err != nil {
			t.Fatalf("WaitForDevice(%q) returned unexpected error: %v", devicePath, err)
		}
		if got != devicePath {
			t.Errorf("WaitForDevice(%q) = %q, want %q", devicePath, got, devicePath)
		}
	})
}

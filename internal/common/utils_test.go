// Package common provides utility functions used across the Kopru CLI.
package common

import (
	"testing"
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

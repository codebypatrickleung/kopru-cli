// Package os handles QCOW2 image mounting and OS-specific configurations.
package os

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/codebypatrickleung/kopru-cli/internal/common"
	"github.com/codebypatrickleung/kopru-cli/internal/logger"
)

// Manager handles image configuration operations.
type Manager struct {
	logger         *logger.Logger
	sourcePlatform string
	nbdDevice      string
	MountDir       string
}

// NewManager creates a new configuration manager.
func NewManager(log *logger.Logger, sourcePlatform string) *Manager {
	return &Manager{
		logger:         log,
		sourcePlatform: sourcePlatform,
		nbdDevice:      "/dev/nbd0",
		MountDir:       "",
	}
}

// ApplyUbuntuConfigurations applies Ubuntu-specific configurations for OCI compatibility.
func (m *Manager) ApplyUbuntuConfigurations() error {
	m.logger.Info("Applying Ubuntu configurations for OCI compatibility...")

	// Apply source platform-specific cleanup
	if m.sourcePlatform == "azure" {
		m.logger.Info("Applying Azure source platform cleanup...")

		// Disable Azure-specific udev rules
		if err := m.DisableAzureUdevRules(); err != nil {
			m.logger.Warning(fmt.Sprintf("Failed to disable Azure udev rules: %v", err))
		}

		// Disable WALinux Agent
		if err := m.DisableAzureLinuxAgent(); err != nil {
			m.logger.Warning(fmt.Sprintf("Failed to disable WALinux Agent: %v", err))
		}

		// Disable Azure Linux hosts template
		if err := m.DisableAzureHostsTemplate(); err != nil {
			m.logger.Warning(fmt.Sprintf("Failed to disable Azure hosts template: %v", err))
		}

		// Comment out Azure PTP hyperv refclock in chrony
		if err := m.DisableAzureChronyRefclock(); err != nil {
			m.logger.Warning(fmt.Sprintf("Failed to comment out Azure chrony refclock: %v", err))
		}
	}

	// Configure cloud-init for OCI
	if err := m.ConfigureCloudInit(); err != nil {
		m.logger.Warning(fmt.Sprintf("Failed to configure cloud-init: %v", err))
	}

	// Update GRUB for OCI console
	if err := m.UpdateGRUB(); err != nil {
		m.logger.Warning(fmt.Sprintf("Failed to update GRUB: %v", err))
	}

	m.logger.Success("Ubuntu configurations complete")
	return nil
}

// ApplyCustomScript executes a custom configuration script.
func (m *Manager) ApplyCustomScript(scriptPath string) error {
	m.logger.Infof("Applying custom configuration script: %s", scriptPath)

	// Verify script exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("custom configuration script not found: %s", scriptPath)
	}

	// Make script executable if it isn't
	if err := os.Chmod(scriptPath, 0755); err != nil {
		m.logger.Warning(fmt.Sprintf("Could not make script executable: %v", err))
	}

	// Set environment variable for the script
	env := append(os.Environ(), fmt.Sprintf("KOPRU_MOUNT_DIR=%s", m.MountDir))

	// Execute the script with mount directory as argument
	cmd := exec.Command(scriptPath, m.MountDir)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("custom configuration script failed: %w", err)
	}

	m.logger.Success("Custom configurations applied successfully")
	return nil
}

// DisableAzureUdevRules disables Azure-specific udev rules.
func (m *Manager) DisableAzureUdevRules() error {
	m.logger.Info("Disabling Azure-specific udev rules...")

	azureRules := []string{
		"66-azure-storage.rules",
		"99-azure-product-uuid.rules",
	}

	for _, rule := range azureRules {
		rulePath := filepath.Join(m.MountDir, "etc/udev/rules.d", rule)
		disabledPath := rulePath + ".disable"
		if _, err := os.Stat(rulePath); err == nil {
			if err := os.Rename(rulePath, disabledPath); err != nil {
				m.logger.Warning(fmt.Sprintf("Failed to disable %s: %v", rule, err))
			} else {
				m.logger.Successf("✓ Disabled %s", rule)
			}
		}
	}

	return nil
}

// DisableAzureHostsTemplate disables the Azure hosts template.
func (m *Manager) DisableAzureHostsTemplate() error {
	m.logger.Info("Disabling Azure hosts template...")

	hostsTemplatePath := filepath.Join(m.MountDir, "etc/cloud/templates/hosts.azurelinux.tmpl")
	
	// Check if the file exists
	if _, err := os.Stat(hostsTemplatePath); os.IsNotExist(err) {
		m.logger.Info("Azure hosts template not found, skipping...")
		return nil
	}

	// Read the current content
	content, err := os.ReadFile(hostsTemplatePath)
	if err != nil {
		return fmt.Errorf("failed to read hosts template: %w", err)
	}

	// Check if "disable" is already appended
	contentStr := string(content)
	if strings.HasSuffix(strings.TrimSpace(contentStr), "disable") {
		m.logger.Info("✓ Azure hosts template already disabled")
		return nil
	}

	// Append "disable" to the file
	updated := contentStr
	if !strings.HasSuffix(contentStr, "\n") {
		updated += "\n"
	}
	updated += "disable\n"

	if err := common.WriteFile(hostsTemplatePath, updated); err != nil {
		return fmt.Errorf("failed to update hosts template: %w", err)
	}

	m.logger.Success("✓ Disabled Azure hosts template")
	return nil
}

// DisableAzureChronyRefclock comments out Azure PTP hyperv refclock in chrony config.
func (m *Manager) DisableAzureChronyRefclock() error {
	m.logger.Info("Commenting out Azure PTP hyperv refclock in chrony config...")

	chronyConfPath := filepath.Join(m.MountDir, "etc/chrony/chrony.conf")
	
	// Check if the file exists
	if _, err := os.Stat(chronyConfPath); os.IsNotExist(err) {
		m.logger.Info("Chrony config not found, skipping...")
		return nil
	}

	// Read the current content
	content, err := os.ReadFile(chronyConfPath)
	if err != nil {
		return fmt.Errorf("failed to read chrony config: %w", err)
	}

	contentStr := string(content)
	targetLine := "refclock PHC /dev/ptp_hyperv poll 3 dpoll -2 offset 0"
	commentedLine := "# " + targetLine
	ociServerLine := "server 169.254.169.254 iburst"

	// Check if the line exists and needs to be commented
	if !strings.Contains(contentStr, targetLine) {
		m.logger.Info("Azure PTP hyperv refclock not found in chrony config, skipping...")
		return nil
	}

	// Check if all occurrences are already commented by checking if uncommented version exists
	hasUncommented := false
	lines := strings.Split(contentStr, "\n")
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == targetLine {
			hasUncommented = true
			break
		}
	}

	if !hasUncommented {
		m.logger.Info("✓ Azure PTP hyperv refclock already commented out")
		return nil
	}

	// Comment out all occurrences of the line (using ReplaceAll to handle potential duplicates)
	updated := strings.ReplaceAll(contentStr, targetLine, commentedLine)

	// Add OCI time server if not already present
	if !strings.Contains(updated, ociServerLine) {
		// Add the OCI server line after the commented refclock line
		updated = strings.ReplaceAll(updated, commentedLine, commentedLine+"\n"+ociServerLine)
		m.logger.Info("✓ Added OCI time server to chrony config")
	}

	if err := common.WriteFile(chronyConfPath, updated); err != nil {
		return fmt.Errorf("failed to update chrony config: %w", err)
	}

	m.logger.Success("✓ Commented out Azure PTP hyperv refclock")
	return nil
}

// DisableAzureLinuxAgent disables the WALinux Agent.
func (m *Manager) DisableAzureLinuxAgent() error {
	m.logger.Info("Disabling WALinux Agent files...")

	waagentPaths := []string{
		"/var/lib/waagent",
		"/etc/init/walinuxagent.conf",
		"/etc/init.d/walinuxagent",
		"/usr/sbin/waagent",
		"/usr/sbin/waagent2.0",
		"/etc/waagent.conf",
		"/var/log/waagent.log",
	}

	for _, path := range waagentPaths {
		fullPath := filepath.Join(m.MountDir, path)
		disabledPath := fullPath + ".disable"
		if _, err := os.Stat(fullPath); err == nil {
			if err := os.Rename(fullPath, disabledPath); err != nil {
				m.logger.Warning(fmt.Sprintf("Failed to disable %s: %v", path, err))
			} else {
				m.logger.Successf("✓ Disabled %s", path)
			}
		}
	}

	return nil
}

// ConfigureCloudInit configures cloud-init for OCI.
func (m *Manager) ConfigureCloudInit() error {
	m.logger.Info("Configuring cloud-init for OCI...")

	ociCfgPath := filepath.Join(m.MountDir, "etc/cloud/cloud.cfg.d/99_oci.cfg")

	// Create cloud.cfg.d directory if it doesn't exist
	cloudCfgDir := filepath.Join(m.MountDir, "etc/cloud/cloud.cfg.d")
	if err := os.MkdirAll(cloudCfgDir, 0755); err != nil {
		return err
	}

	content := "datasource_list: [Oracle, None]\n"
	if err := common.WriteFile(ociCfgPath, content); err != nil {
		return err
	}

	// Disable Azure specific cloud-init files if present
	azureFiles := []string{
		"10-azure-kvp.cfg",
		"90-azure.cfg",
		"90_dpkg.cfg",
	}

	for _, file := range azureFiles {
		filePath := filepath.Join(m.MountDir, "etc/cloud/cloud.cfg.d", file)
		disabledPath := filePath + ".disable"
		if _, err := os.Stat(filePath); err == nil {
			if err := os.Rename(filePath, disabledPath); err != nil {
				m.logger.Warning(fmt.Sprintf("Failed to disable %s: %v", file, err))
			} else {
				m.logger.Successf("✓ Disabled %s", file)
			}
		}
	}

	m.logger.Success("✓ Configured cloud-init datasource")
	return nil
}

// UpdateGRUB updates GRUB configuration for OCI serial console.
func (m *Manager) UpdateGRUB() error {
	m.logger.Info("Updating GRUB for OCI serial console...")

	grubPath := filepath.Join(m.MountDir, "etc/default/grub")
	if _, err := os.Stat(grubPath); os.IsNotExist(err) {
		m.logger.Info("GRUB config not found, skipping...")
		return nil
	}

	content, err := os.ReadFile(grubPath)
	if err != nil {
		return err
	}

	contentStr := string(content)

	// Check if console parameter already present
	if strings.Contains(contentStr, "console=ttyS0") {
		m.logger.Info("✓ GRUB console already configured")
		return nil
	}

	// Check for any existing console parameter
	if strings.Contains(contentStr, "console=") {
		m.logger.Info("GRUB already has a console parameter, skipping to avoid conflicts")
		return nil
	}

	// Add console parameter to GRUB_CMDLINE_LINUX
	lines := strings.Split(contentStr, "\n")
	modified := false
	for i, line := range lines {
		if strings.HasPrefix(line, "GRUB_CMDLINE_LINUX=") {
			// Validate the line format: should start with GRUB_CMDLINE_LINUX=" and end with "
			if !strings.HasPrefix(line, "GRUB_CMDLINE_LINUX=\"") || !strings.HasSuffix(line, "\"") {
				m.logger.Warning("GRUB_CMDLINE_LINUX line has unexpected format, skipping GRUB update")
				return nil
			}
			// Remove trailing quote, trim spaces, add console parameter, add closing quote
			withoutQuote := strings.TrimSuffix(line, "\"")
			withoutQuote = strings.TrimRight(withoutQuote, " ")
			lines[i] = withoutQuote + " console=ttyS0,115200\""
			modified = true
			break
		}
	}

	if !modified {
		m.logger.Warning("GRUB_CMDLINE_LINUX not found in GRUB config, skipping")
		return nil
	}

	updated := strings.Join(lines, "\n")
	if err := common.WriteFile(grubPath, updated); err != nil {
		return err
	}

	m.logger.Success("✓ Updated GRUB console configuration")
	return nil
}

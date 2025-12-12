package config

import (
	"os"
	"testing"
)

func setEnvVars(vars map[string]string) {
	for k, v := range vars {
		os.Setenv(k, v)
	}
}

func unsetEnvVars(vars []string) {
	for _, k := range vars {
		os.Unsetenv(k)
	}
}

func TestConfigLoad(t *testing.T) {
	envVars := map[string]string{
		"AZURE_COMPUTE_NAME":   "test-vm",
		"AZURE_RESOURCE_GROUP": "test-rg",
		"OCI_COMPARTMENT_ID":   "ocid1.compartment.test",
		"OCI_SUBNET_ID":        "ocid1.subnet.test",
	}
	setEnvVars(envVars)
	defer unsetEnvVars([]string{
		"AZURE_COMPUTE_NAME",
		"AZURE_RESOURCE_GROUP",
		"OCI_COMPARTMENT_ID",
		"OCI_SUBNET_ID",
	})

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.AzureComputeName != envVars["AZURE_COMPUTE_NAME"] {
		t.Errorf("Expected AzureComputeName to be '%s', got '%s'", envVars["AZURE_COMPUTE_NAME"], cfg.AzureComputeName)
	}
	if cfg.AzureResourceGroup != envVars["AZURE_RESOURCE_GROUP"] {
		t.Errorf("Expected AzureResourceGroup to be '%s', got '%s'", envVars["AZURE_RESOURCE_GROUP"], cfg.AzureResourceGroup)
	}
	if cfg.OCICompartmentID != envVars["OCI_COMPARTMENT_ID"] {
		t.Errorf("Expected OCICompartmentID to be '%s', got '%s'", envVars["OCI_COMPARTMENT_ID"], cfg.OCICompartmentID)
	}
	if cfg.OCISubnetID != envVars["OCI_SUBNET_ID"] {
		t.Errorf("Expected OCISubnetID to be '%s', got '%s'", envVars["OCI_SUBNET_ID"], cfg.OCISubnetID)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid Azure to OCI config",
			config: &Config{
				SourcePlatform:     "azure",
				TargetPlatform:     "oci",
				AzureComputeName:   "test-vm",
				AzureResourceGroup: "test-rg",
				OCICompartmentID:   "ocid1.compartment.test",
				OCISubnetID:        "ocid1.subnet.test",
			},
			expectError: false,
		},
		{
			name: "missing Azure Compute name",
			config: &Config{
				SourcePlatform:     "azure",
				TargetPlatform:     "oci",
				AzureResourceGroup: "test-rg",
				OCICompartmentID:   "ocid1.compartment.test",
				OCISubnetID:        "ocid1.subnet.test",
			},
			expectError: true,
		},
		{
			name: "missing OCI compartment ID",
			config: &Config{
				SourcePlatform:     "azure",
				TargetPlatform:     "oci",
				AzureComputeName:   "test-vm",
				AzureResourceGroup: "test-rg",
				OCISubnetID:        "ocid1.subnet.test",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	os.Clearenv()
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.SourcePlatform != "azure" {
		t.Errorf("Expected default SourcePlatform to be 'azure', got '%s'", cfg.SourcePlatform)
	}
	if cfg.TargetPlatform != "oci" {
		t.Errorf("Expected default TargetPlatform to be 'oci', got '%s'", cfg.TargetPlatform)
	}
	if cfg.OCIBucketName != "kopru-bucket" {
		t.Errorf("Expected default OCIBucketName to be 'kopru-bucket', got '%s'", cfg.OCIBucketName)
	}
	if cfg.OCIImageName != "kopru-image" {
		t.Errorf("Expected default OCIImageName to be 'kopru-image', got '%s'", cfg.OCIImageName)
	}
	if cfg.OCIRegion != "eu-frankfurt-1" {
		t.Errorf("Expected default OCIRegion to be 'eu-frankfurt-1', got '%s'", cfg.OCIRegion)
	}
}

func TestTemplateOutputDirNaming(t *testing.T) {
	tests := []struct {
		name             string
		azureComputeName string
		explicitDir      string
		expectedDir      string
	}{
		{"Default dir with Azure Compute name", "test-vm", "", "./test-vm-template-output"},
		{"Azure Compute name with special characters", "Test_VM-123", "", "./test_vm-123-template-output"},
		{"Explicit dir overrides default", "test-vm", "./custom-dir", "./custom-dir"},
		{"No Azure Compute name uses default", "", "", "./template-output"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			if tt.azureComputeName != "" {
				os.Setenv("AZURE_COMPUTE_NAME", tt.azureComputeName)
			}
			if tt.explicitDir != "" {
				os.Setenv("TEMPLATE_OUTPUT_DIR", tt.explicitDir)
			}
			cfg, err := Load("")
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}
			if cfg.TemplateOutputDir != tt.expectedDir {
				t.Errorf("Expected TemplateOutputDir to be '%s', got '%s'", tt.expectedDir, cfg.TemplateOutputDir)
			}
		})
	}
}

func TestOCIInstanceNameNaming(t *testing.T) {
	tests := []struct {
		name             string
		azureComputeName string
		explicitName     string
		expectedName     string
	}{
		{"Default name with Azure Compute name", "test-vm", "", "test-vm"},
		{"Azure Compute name with special characters", "Test_VM-123", "", "test_vm-123"},
		{"Explicit name overrides default", "test-vm", "custom-instance", "custom-instance"},
		{"No Azure Compute name uses default", "", "", "kopru-instance"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			if tt.azureComputeName != "" {
				os.Setenv("AZURE_COMPUTE_NAME", tt.azureComputeName)
			}
			if tt.explicitName != "" {
				os.Setenv("OCI_INSTANCE_NAME", tt.explicitName)
			}
			cfg, err := Load("")
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}
			if cfg.OCIInstanceName != tt.expectedName {
				t.Errorf("Expected OCIInstanceName to be '%s', got '%s'", tt.expectedName, cfg.OCIInstanceName)
			}
		})
	}
}

func TestOCIImageNameNaming(t *testing.T) {
	tests := []struct {
		name             string
		azureComputeName string
		explicitName     string
		expectedName     string
	}{
		{"Default name with Azure Compute name", "test-vm", "", "test-vm-image"},
		{"Azure Compute name with special characters", "Test_VM-123", "", "test_vm-123-image"},
		{"Explicit name overrides default", "test-vm", "custom-image", "custom-image"},
		{"No Azure Compute name uses default", "", "", "kopru-image"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			if tt.azureComputeName != "" {
				os.Setenv("AZURE_COMPUTE_NAME", tt.azureComputeName)
			}
			if tt.explicitName != "" {
				os.Setenv("OCI_IMAGE_NAME", tt.explicitName)
			}
			cfg, err := Load("")
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}
			if cfg.OCIImageName != tt.expectedName {
				t.Errorf("Expected OCIImageName to be '%s', got '%s'", tt.expectedName, cfg.OCIImageName)
			}
		})
	}
}

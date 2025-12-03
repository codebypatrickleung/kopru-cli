package config

import (
	"os"
	"testing"
)

func TestConfigLoad(t *testing.T) {
	// Set test environment variables
	os.Setenv("AZURE_COMPUTE_NAME", "test-vm")
	os.Setenv("AZURE_RESOURCE_GROUP", "test-rg")
	os.Setenv("OCI_COMPARTMENT_ID", "ocid1.compartment.test")
	os.Setenv("OCI_SUBNET_ID", "ocid1.subnet.test")
	defer func() {
		os.Unsetenv("AZURE_COMPUTE_NAME")
		os.Unsetenv("AZURE_RESOURCE_GROUP")
		os.Unsetenv("OCI_COMPARTMENT_ID")
		os.Unsetenv("OCI_SUBNET_ID")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.AzureComputeName != "test-vm" {
		t.Errorf("Expected AzureComputeName to be 'test-vm', got '%s'", cfg.AzureComputeName)
	}

	if cfg.AzureResourceGroup != "test-rg" {
		t.Errorf("Expected AzureResourceGroup to be 'test-rg', got '%s'", cfg.AzureResourceGroup)
	}

	if cfg.OCICompartmentID != "ocid1.compartment.test" {
		t.Errorf("Expected OCICompartmentID to be 'ocid1.compartment.test', got '%s'", cfg.OCICompartmentID)
	}

	if cfg.OCISubnetID != "ocid1.subnet.test" {
		t.Errorf("Expected OCISubnetID to be 'ocid1.subnet.test', got '%s'", cfg.OCISubnetID)
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
	// Clear environment variables
	os.Clearenv()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test default values
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

	if !cfg.KeepVHD {
		t.Error("Expected default KeepVHD to be true")
	}
}

func TestTemplateOutputDirNaming(t *testing.T) {
	tests := []struct {
		name             string
		azureComputeName string
		explicitDir      string
		expectedDir      string
	}{
		{
			name:             "Default dir with Azure Compute name",
			azureComputeName: "test-vm",
			explicitDir:      "",
			expectedDir:      "./test-vm-template-output",
		},
		{
			name:             "Azure Compute name with special characters",
			azureComputeName: "Test_VM-123",
			explicitDir:      "",
			expectedDir:      "./test_vm-123-template-output",
		},
		{
			name:             "Explicit dir overrides default",
			azureComputeName: "test-vm",
			explicitDir:      "./custom-dir",
			expectedDir:      "./custom-dir",
		},
		{
			name:             "No Azure Compute name uses default",
			azureComputeName: "",
			explicitDir:      "",
			expectedDir:      "./template-output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set Azure Compute name if provided
			if tt.azureComputeName != "" {
				os.Setenv("AZURE_COMPUTE_NAME", tt.azureComputeName)
			}

			// Set explicit dir if provided
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
		name              string
		azureComputeName  string
		explicitName      string
		expectedName      string
	}{
		{
			name:             "Default name with Azure Compute name",
			azureComputeName: "test-vm",
			explicitName:     "",
			expectedName:     "test-vm",
		},
		{
			name:             "Azure Compute name with special characters",
			azureComputeName: "Test_VM-123",
			explicitName:     "",
			expectedName:     "test_vm-123",
		},
		{
			name:             "Explicit name overrides default",
			azureComputeName: "test-vm",
			explicitName:     "custom-instance",
			expectedName:     "custom-instance",
		},
		{
			name:             "No Azure Compute name uses default",
			azureComputeName: "",
			explicitName:     "",
			expectedName:     "kopru-instance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set Azure Compute name if provided
			if tt.azureComputeName != "" {
				os.Setenv("AZURE_COMPUTE_NAME", tt.azureComputeName)
			}

			// Set explicit name if provided
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
		name              string
		azureComputeName  string
		explicitName      string
		expectedName      string
	}{
		{
			name:             "Default name with Azure Compute name",
			azureComputeName: "test-vm",
			explicitName:     "",
			expectedName:     "test-vm-image",
		},
		{
			name:             "Azure Compute name with special characters",
			azureComputeName: "Test_VM-123",
			explicitName:     "",
			expectedName:     "test_vm-123-image",
		},
		{
			name:             "Explicit name overrides default",
			azureComputeName: "test-vm",
			explicitName:     "custom-image",
			expectedName:     "custom-image",
		},
		{
			name:             "No Azure Compute name uses default",
			azureComputeName: "",
			explicitName:     "",
			expectedName:     "kopru-image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set Azure Compute name if provided
			if tt.azureComputeName != "" {
				os.Setenv("AZURE_COMPUTE_NAME", tt.azureComputeName)
			}

			// Set explicit name if provided
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


package template

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/codebypatrickleung/kopru-cli/internal/config"
	"github.com/codebypatrickleung/kopru-cli/internal/logger"
)

func TestBootVolumeSizeCalculation(t *testing.T) {
	tests := []struct {
		name              string
		azureDiskSizeGB   int64
		expectedMinSizeGB int64
	}{
		{"Azure disk 30GB should use minimum 50GB", 30, 50},
		{"Azure disk 50GB should use 50GB", 50, 50},
		{"Azure disk 100GB should use 100GB", 100, 100},
		{"Azure disk 127GB should use 127GB", 127, 127},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfg := &config.Config{
				OCICompartmentID:  "test-compartment",
				OCISubnetID:       "test-subnet",
				OCIRegion:         "us-ashburn-1",
				OCIInstanceName:   "test-instance",
				OCIImageName:      "test-image",
				TemplateOutputDir: tmpDir,
			}
			log := logger.New(false)
			gen := NewOCIGenerator(cfg, log, "test-namespace", "test-object.qcow2", nil, nil, tt.azureDiskSizeGB, 0, 0, "x86_64")
			if err := gen.GenerateTemplate(); err != nil {
				t.Fatalf("GenerateTemplate failed: %v", err)
			}
			tfvarsPath := filepath.Join(tmpDir, "terraform.tfvars")
			content, err := os.ReadFile(tfvarsPath)
			if err != nil {
				t.Fatalf("Failed to read terraform.tfvars: %v", err)
			}
			re := regexp.MustCompile(`boot_volume_size_in_gbs\s*=\s*(\d+)`)
			matches := re.FindStringSubmatch(string(content))
			if len(matches) < 2 {
				t.Fatal("boot_volume_size_in_gbs not found in terraform.tfvars")
			}
			actualSize, err := strconv.ParseInt(matches[1], 10, 64)
			if err != nil {
				t.Fatalf("Failed to parse boot volume size: %v", err)
			}
			if actualSize != tt.expectedMinSizeGB {
				t.Errorf("Expected boot_volume_size_in_gbs to be %d, got %d", tt.expectedMinSizeGB, actualSize)
			}
			t.Logf("✓ Boot volume size correctly set to %d GB (Azure disk: %d GB)", actualSize, tt.azureDiskSizeGB)
		})
	}
}

func TestUEFICapabilitySchemaGeneration(t *testing.T) {
	tests := []struct {
		name                string
		uefiEnabled         bool
		shouldContainSchema bool
	}{
		{"UEFI enabled should include capability schema", true, true},
		{"UEFI disabled should not include capability schema", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfg := &config.Config{
				OCICompartmentID:   "test-compartment",
				OCISubnetID:        "test-subnet",
				OCIRegion:          "us-ashburn-1",
				OCIInstanceName:    "test-instance",
				OCIImageName:       "test-image",
				OCIImageEnableUEFI: tt.uefiEnabled,
				TemplateOutputDir:  tmpDir,
			}
			log := logger.New(false)
			gen := NewOCIGenerator(cfg, log, "test-namespace", "test-object.qcow2", nil, nil, 50, 0, 0, "x86_64")
			if err := gen.GenerateTemplate(); err != nil {
				t.Fatalf("GenerateTemplate failed: %v", err)
			}
			mainTfPath := filepath.Join(tmpDir, "main.tf")
			content, err := os.ReadFile(mainTfPath)
			if err != nil {
				t.Fatalf("Failed to read main.tf: %v", err)
			}
			mainTfContent := string(content)
			hasGlobalSchemaData := regexp.MustCompile(`data\s+"oci_core_compute_global_image_capability_schemas"`).MatchString(mainTfContent)
			hasImageSchemaData := regexp.MustCompile(`image_schema_data\s*=`).MatchString(mainTfContent)
			hasCapabilitySchemaResource := regexp.MustCompile(`resource\s+"oci_core_compute_image_capability_schema"`).MatchString(mainTfContent)
			hasComputeFirmware := regexp.MustCompile(`Compute\.Firmware`).MatchString(mainTfContent)
			hasUEFI64 := regexp.MustCompile(`UEFI_64`).MatchString(mainTfContent)

			if tt.shouldContainSchema {
				if !hasGlobalSchemaData {
					t.Error("Expected main.tf to contain oci_core_compute_global_image_capability_schemas data source")
				}
				if !hasImageSchemaData {
					t.Error("Expected main.tf to contain image_schema_data local")
				}
				if !hasCapabilitySchemaResource {
					t.Error("Expected main.tf to contain oci_core_compute_image_capability_schema resource")
				}
				if !hasComputeFirmware {
					t.Error("Expected main.tf to contain Compute.Firmware configuration")
				}
				if !hasUEFI64 {
					t.Error("Expected main.tf to contain UEFI_64 value")
				}
				t.Log("✓ UEFI capability schema resources correctly included in main.tf")
			} else {
				if hasGlobalSchemaData {
					t.Error("Expected main.tf to NOT contain oci_core_compute_global_image_capability_schemas data source")
				}
				if hasCapabilitySchemaResource {
					t.Error("Expected main.tf to NOT contain oci_core_compute_image_capability_schema resource")
				}
				if hasComputeFirmware {
					t.Error("Expected main.tf to NOT contain Compute.Firmware configuration")
				}
				if hasUEFI64 {
					t.Error("Expected main.tf to NOT contain UEFI_64 value")
				}
				t.Log("✓ UEFI capability schema resources correctly excluded from main.tf")
			}
		})
	}
}

func TestCPUAndMemoryConfiguration(t *testing.T) {
	tests := []struct {
		name             string
		vmCPUs           int32
		vmMemoryGB       int32
		vmArchitecture   string
		expectedShape    string
		expectedOCPUs    int32
		expectedMemoryGB int32
	}{
		{"x86_64 with 2 vCPUs and 8GB memory", 2, 8, "x86_64", "VM.Standard.E5.Flex", 1, 8},
		{"x86_64 with 3 vCPUs and 8GB memory (odd, rounds up)", 3, 8, "x86_64", "VM.Standard.E5.Flex", 2, 8},
		{"ARM64 with 4 vCPUs and 16GB memory", 4, 16, "ARM64", "VM.Standard.A1.Flex", 2, 16},
		{"x86_64 with default values (0 CPUs)", 0, 0, "x86_64", "VM.Standard.E5.Flex", 1, 12},
		{"x86_64 with 8 vCPUs and 64GB memory", 8, 64, "x86_64", "VM.Standard.E5.Flex", 4, 64},
		{"x86_64 with 1 vCPU and 4GB memory (minimum)", 1, 4, "x86_64", "VM.Standard.E5.Flex", 1, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfg := &config.Config{
				OCICompartmentID:  "test-compartment",
				OCISubnetID:       "test-subnet",
				OCIRegion:         "us-ashburn-1",
				OCIInstanceName:   "test-instance",
				OCIImageName:      "test-image",
				TemplateOutputDir: tmpDir,
			}
			log := logger.New(false)
			gen := NewOCIGenerator(cfg, log, "test-namespace", "test-object.qcow2", nil, nil, 50, tt.vmCPUs, tt.vmMemoryGB, tt.vmArchitecture)
			if err := gen.GenerateTemplate(); err != nil {
				t.Fatalf("GenerateTemplate failed: %v", err)
			}
			tfvarsPath := filepath.Join(tmpDir, "terraform.tfvars")
			content, err := os.ReadFile(tfvarsPath)
			if err != nil {
				t.Fatalf("Failed to read terraform.tfvars: %v", err)
			}
			tfvarsContent := string(content)

			shapeRe := regexp.MustCompile(`instance_shape\s*=\s*"([^"]+)"`)
			shapeMatches := shapeRe.FindStringSubmatch(tfvarsContent)
			if len(shapeMatches) < 2 {
				t.Fatal("instance_shape not found in terraform.tfvars")
			}
			actualShape := shapeMatches[1]
			if actualShape != tt.expectedShape {
				t.Errorf("Expected instance_shape to be %s, got %s", tt.expectedShape, actualShape)
			}

			ocpusRe := regexp.MustCompile(`instance_ocpus\s*=\s*(\d+)`)
			ocpusMatches := ocpusRe.FindStringSubmatch(tfvarsContent)
			if len(ocpusMatches) < 2 {
				t.Fatal("instance_ocpus not found in terraform.tfvars")
			}
			actualOCPUs, err := strconv.ParseInt(ocpusMatches[1], 10, 32)
			if err != nil {
				t.Fatalf("Failed to parse instance_ocpus: %v", err)
			}
			if int32(actualOCPUs) != tt.expectedOCPUs {
				t.Errorf("Expected instance_ocpus to be %d, got %d", tt.expectedOCPUs, actualOCPUs)
			}

			memoryRe := regexp.MustCompile(`instance_memory_gb\s*=\s*(\d+)`)
			memoryMatches := memoryRe.FindStringSubmatch(tfvarsContent)
			if len(memoryMatches) < 2 {
				t.Fatal("instance_memory_gb not found in terraform.tfvars")
			}
			actualMemory, err := strconv.ParseInt(memoryMatches[1], 10, 32)
			if err != nil {
				t.Fatalf("Failed to parse instance_memory_gb: %v", err)
			}
			if int32(actualMemory) != tt.expectedMemoryGB {
				t.Errorf("Expected instance_memory_gb to be %d, got %d", tt.expectedMemoryGB, actualMemory)
			}
			t.Logf("✓ Shape: %s, OCPUs: %d, Memory: %d GB", actualShape, actualOCPUs, actualMemory)
		})
	}
}

func TestArchitectureTagging(t *testing.T) {
	tests := []struct {
		name           string
		vmArchitecture string
		expectedTag    string
	}{
		{"x86_64 architecture tagged", "x86_64", `"source-architecture" = "x86_64"`},
		{"ARM64 architecture tagged", "ARM64", `"source-architecture" = "ARM64"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfg := &config.Config{
				OCICompartmentID:  "test-compartment",
				OCISubnetID:       "test-subnet",
				OCIRegion:         "us-ashburn-1",
				OCIInstanceName:   "test-instance",
				OCIImageName:      "test-image",
				TemplateOutputDir: tmpDir,
			}
			log := logger.New(false)
			gen := NewOCIGenerator(cfg, log, "test-namespace", "test-object.qcow2", nil, nil, 50, 2, 8, tt.vmArchitecture)
			if err := gen.GenerateTemplate(); err != nil {
				t.Fatalf("GenerateTemplate failed: %v", err)
			}
			tfvarsPath := filepath.Join(tmpDir, "terraform.tfvars")
			content, err := os.ReadFile(tfvarsPath)
			if err != nil {
				t.Fatalf("Failed to read terraform.tfvars: %v", err)
			}
			tfvarsContent := string(content)
			if !regexp.MustCompile(regexp.QuoteMeta(tt.expectedTag)).MatchString(tfvarsContent) {
				t.Errorf("Expected to find tag %s in terraform.tfvars", tt.expectedTag)
			}
			t.Logf("✓ Architecture tag correctly set: %s", tt.expectedTag)
		})
	}
}

func TestARM64ShapeManagementGeneration(t *testing.T) {
	tests := []struct {
		name                         string
		vmArchitecture               string
		uefiEnabled                  bool
		shouldContainUEFISchema      bool
		shouldContainShapeManagement bool
	}{
		{"ARM64 architecture should include shape management and UEFI", "ARM64", false, true, true},
		{"x86_64 architecture should not include shape management", "x86_64", false, false, false},
		{"ARM64 with UEFI should include both UEFI and shape management", "ARM64", true, true, true},
		{"x86_64 with UEFI should include UEFI but not shape management", "x86_64", true, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfg := &config.Config{
				OCICompartmentID:   "test-compartment",
				OCISubnetID:        "test-subnet",
				OCIRegion:          "us-ashburn-1",
				OCIInstanceName:    "test-instance",
				OCIImageName:       "test-image",
				OCIImageEnableUEFI: tt.uefiEnabled,
				TemplateOutputDir:  tmpDir,
			}
			log := logger.New(false)
			gen := NewOCIGenerator(cfg, log, "test-namespace", "test-object.qcow2", nil, nil, 50, 4, 16, tt.vmArchitecture)
			if err := gen.GenerateTemplate(); err != nil {
				t.Fatalf("GenerateTemplate failed: %v", err)
			}
			mainTfPath := filepath.Join(tmpDir, "main.tf")
			content, err := os.ReadFile(mainTfPath)
			if err != nil {
				t.Fatalf("Failed to read main.tf: %v", err)
			}
			mainTfContent := string(content)

			// Check for UEFI capability schema
			hasGlobalSchemaData := regexp.MustCompile(`data\s+"oci_core_compute_global_image_capability_schemas"`).MatchString(mainTfContent)
			hasImageSchemaData := regexp.MustCompile(`image_schema_data\s*=`).MatchString(mainTfContent)
			hasCapabilitySchemaResource := regexp.MustCompile(`resource\s+"oci_core_compute_image_capability_schema"`).MatchString(mainTfContent)
			hasComputeFirmware := regexp.MustCompile(`Compute\.Firmware`).MatchString(mainTfContent)
			hasUEFI64 := regexp.MustCompile(`UEFI_64`).MatchString(mainTfContent)

			// Check for shape management resource
			hasShapeManagementResource := regexp.MustCompile(`resource\s+"oci_core_shape_management"`).MatchString(mainTfContent)
			hasA1FlexShape := regexp.MustCompile(regexp.QuoteMeta(DefaultARM64Shape)).MatchString(mainTfContent)

			// Verify shape management (replaces old shape family schema approach)
			if tt.shouldContainShapeManagement {
				if !hasShapeManagementResource {
					t.Error("Expected main.tf to contain oci_core_shape_management resource")
				}
				if !hasA1FlexShape {
					t.Error("Expected main.tf to contain VM.Standard.A1.Flex shape")
				}
				t.Log("✓ ARM64 shape management resource correctly included in main.tf")
			} else {
				if hasShapeManagementResource {
					t.Error("Expected main.tf to NOT contain oci_core_shape_management resource")
				}
				t.Log("✓ Shape management resource correctly excluded from main.tf")
			}

			// Verify UEFI schema
			if tt.shouldContainUEFISchema {
				if !hasGlobalSchemaData {
					t.Error("Expected main.tf to contain oci_core_compute_global_image_capability_schemas data source")
				}
				if !hasImageSchemaData {
					t.Error("Expected main.tf to contain image_schema_data local")
				}
				if !hasCapabilitySchemaResource {
					t.Error("Expected main.tf to contain oci_core_compute_image_capability_schema resource")
				}
				if !hasComputeFirmware {
					t.Error("Expected main.tf to contain Compute.Firmware configuration")
				}
				if !hasUEFI64 {
					t.Error("Expected main.tf to contain UEFI_64 value")
				}
				t.Log("✓ UEFI firmware capability schema correctly configured in main.tf")
			} else {
				if hasGlobalSchemaData {
					t.Error("Expected main.tf to NOT contain oci_core_compute_global_image_capability_schemas data source")
				}
				if hasCapabilitySchemaResource {
					t.Error("Expected main.tf to NOT contain oci_core_compute_image_capability_schema resource")
				}
				if hasComputeFirmware {
					t.Error("Expected main.tf to NOT contain Compute.Firmware configuration")
				}
				if hasUEFI64 {
					t.Error("Expected main.tf to NOT contain UEFI_64 value")
				}
				t.Log("✓ UEFI firmware capability schema correctly excluded from main.tf")
			}

			// Verify old approach is NOT present (ShapeFamily in schema)
			hasShapeFamily := regexp.MustCompile(`Compute\.ShapeFamily`).MatchString(mainTfContent)
			hasA1Family := regexp.MustCompile(`A1-Family`).MatchString(mainTfContent)
			hasA2Family := regexp.MustCompile(`A2-Family`).MatchString(mainTfContent)
			if hasShapeFamily || hasA1Family || hasA2Family {
				t.Error("Expected main.tf to NOT contain old Compute.ShapeFamily approach (A1-Family, A2-Family)")
			}
		})
	}
}

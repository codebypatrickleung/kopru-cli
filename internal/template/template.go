// Package template provides template-related operations for OCI deployment.
package template

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/codebypatrickleung/kopru-cli/internal/common"
	"github.com/codebypatrickleung/kopru-cli/internal/config"
	"github.com/codebypatrickleung/kopru-cli/internal/logger"
)

const DefaultAvailabilityDomain = "1"

// OCI Flex shape resource constraints
const (
	MinOCPUs              = 1  // Minimum OCPUs for OCI Flex shapes
	DefaultOCPUs          = 1  // Default OCPUs when source VM config is unavailable
	DefaultMemoryGB       = 12 // Default memory in GB when source VM config is unavailable
	MinMemoryPerOCPU      = 1  // Minimum GB of memory per OCPU
	MaxMemoryPerOCPU      = 64 // Maximum GB of memory per OCPU
)

// uefiSchemaData is the JSON configuration for enabling UEFI_64 firmware in OCI image capability schema
const uefiSchemaData = `{\"values\": [\"UEFI_64\"],\"defaultValue\": \"UEFI_64\",\"descriptorType\": \"enumstring\",\"source\": \"IMAGE\"}`

// defaultImageCapabilitySchemaVersion is the fallback version when no global schemas are available
const defaultImageCapabilitySchemaVersion = "1"

// OCIGenerator handles template generation for OCI.
type OCIGenerator struct {
	config                *config.Config
	logger                *logger.Logger
	namespace             string
	objectName            string
	dataDiskSnapshotIDs   []string
	dataDiskSnapshotNames []string
	bootVolumeSizeGB      int64
	vmCPUs                int32
	vmMemoryGB            int32
	vmArchitecture        string
}

// NewOCIGenerator creates a new OCI template generator.
func NewOCIGenerator(cfg *config.Config, log *logger.Logger, namespace, objectName string, dataDiskSnapshotIDs, dataDiskSnapshotNames []string, bootVolumeSizeGB int64, vmCPUs int32, vmMemoryGB int32, vmArchitecture string) *OCIGenerator {
	return &OCIGenerator{
		config:                cfg,
		logger:                log,
		namespace:             namespace,
		objectName:            objectName,
		dataDiskSnapshotIDs:   dataDiskSnapshotIDs,
		dataDiskSnapshotNames: dataDiskSnapshotNames,
		bootVolumeSizeGB:      bootVolumeSizeGB,
		vmCPUs:                vmCPUs,
		vmMemoryGB:            vmMemoryGB,
		vmArchitecture:        vmArchitecture,
	}
}

// formatTemplateList converts a string slice to template list format.
func formatTemplateList(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteString("[\n")
	for i, item := range items {
		b.WriteString(fmt.Sprintf("  \"%s\"", item))
		if i < len(items)-1 {
			b.WriteString(",\n")
		} else {
			b.WriteString("\n")
		}
	}
	b.WriteString("]")
	return b.String()
}

// selectOCIShape determines the appropriate OCI shape based on the architecture.
// For x86_64: VM.Standard.E5.Flex (AMD EPYC or Intel Xeon)
// For ARM64: VM.Standard.A1.Flex (Ampere Altra)
func (g *OCIGenerator) selectOCIShape() string {
	if g.vmArchitecture == "ARM64" {
		g.logger.Infof("Selecting ARM64 shape (VM.Standard.A1.Flex) based on source VM architecture")
		return "VM.Standard.A1.Flex"
	}
	g.logger.Infof("Selecting x86_64 shape (VM.Standard.E5.Flex) based on source VM architecture")
	return "VM.Standard.E5.Flex"
}

// calculateOCIResources determines the appropriate OCPU and memory configuration for OCI.
// For x86_64 (E5.Flex): 1 OCPU minimum, memory ratio 1-64 GB per OCPU
// For ARM64 (A1.Flex): 1 OCPU minimum, memory ratio 1-64 GB per OCPU
func (g *OCIGenerator) calculateOCIResources() (ocpus int32, memoryGB int32) {
	// If no source VM configuration is available, use defaults
	if g.vmCPUs == 0 || g.vmMemoryGB == 0 {
		g.logger.Warning(fmt.Sprintf("No source VM configuration available, using default: %d OCPU, %d GB memory", DefaultOCPUs, DefaultMemoryGB))
		return DefaultOCPUs, DefaultMemoryGB
	}
	
	// Azure CPUs typically map to OCPUs
	ocpus = g.vmCPUs
	memoryGB = g.vmMemoryGB
	
	// Ensure minimum OCPUs
	if ocpus < MinOCPUs {
		ocpus = MinOCPUs
	}
	
	// OCI Flex shapes support 1-64 GB memory per OCPU
	// Ensure memory is within valid range
	minMemory := ocpus * MinMemoryPerOCPU
	maxMemory := ocpus * MaxMemoryPerOCPU
	
	if memoryGB < minMemory {
		g.logger.Infof("Adjusting memory from %d GB to minimum %d GB for %d OCPUs", memoryGB, minMemory, ocpus)
		memoryGB = minMemory
	} else if memoryGB > maxMemory {
		g.logger.Infof("Adjusting memory from %d GB to maximum %d GB for %d OCPUs", memoryGB, maxMemory, ocpus)
		memoryGB = maxMemory
	}
	
	g.logger.Infof("Mapped Azure VM (%d CPUs, %d GB) to OCI (%d OCPUs, %d GB)", g.vmCPUs, g.vmMemoryGB, ocpus, memoryGB)
	
	return ocpus, memoryGB
}

// GenerateTemplate generates all template configuration files.
func (g *OCIGenerator) GenerateTemplate() error {
	if err := common.EnsureDir(g.config.TemplateOutputDir); err != nil {
		return fmt.Errorf("failed to create template output directory: %w", err)
	}
	g.logger.Infof("Generating template files in: %s", g.config.TemplateOutputDir)

	generators := []func() error{
		g.generateProviderTF,
		g.generateVariablesTF,
		g.generateMainTF,
		g.generateOutputsTF,
		g.generateTFVars,
		g.generateReadme,
	}
	for _, gen := range generators {
		if err := gen(); err != nil {
			return err
		}
	}
	g.logger.Successf("Template generated in %s", g.config.TemplateOutputDir)
	return nil
}

// DeployTemplate executes OpenTofu commands to deploy the infrastructure.
func (g *OCIGenerator) DeployTemplate() error {
	if err := common.CheckCommand("tofu"); err != nil {
		return fmt.Errorf("tofu not found: %w", err)
	}
	dir := g.config.TemplateOutputDir

	steps := []struct {
		msg     string
		args    []string
		succ    string
	}{
		{"Running tofu init...", []string{"-chdir=" + dir, "init"}, "✓ OpenTofu initialized"},
		{"Running tofu plan...", []string{"-chdir=" + dir, "plan", "-out=tfplan"}, "✓ OpenTofu plan created"},
		{"Running tofu apply (this may take a while)...", []string{"-chdir=" + dir, "apply", "-auto-approve", "tfplan"}, "Instance deployed with OpenTofu"},
	}
	for _, step := range steps {
		g.logger.Info(step.msg)
		out, err := common.RunCommand("tofu", step.args...)
		if err != nil {
			return fmt.Errorf("%s failed: %w\nOutput: %s", strings.Fields(step.msg)[1], err, out)
		}
		g.logger.Success(step.succ)
	}
	g.logger.Infof("Run 'tofu output' in %s to see instance details", dir)
	return nil
}

func (g *OCIGenerator) generateProviderTF() error {
	content := `# --------------------------------------------------------------------------------------------
# OCI Provider Configuration
# --------------------------------------------------------------------------------------------
# This file configures the OCI provider for OpenTofu.
# Credentials are sourced from OCI CLI configuration or environment variables.
# --------------------------------------------------------------------------------------------

terraform {
  required_version = ">= 1.0.0"
  required_providers {
	oci = {
	  source  = "oracle/oci"
	  version = ">= 5.0.0"
	}
  }
}

provider "oci" {
  region = var.region
}
`
	return os.WriteFile(filepath.Join(g.config.TemplateOutputDir, "provider.tf"), []byte(content), 0644)
}

func (g *OCIGenerator) generateVariablesTF() error {
	content := `# --------------------------------------------------------------------------------------------
# Variable Definitions for OCI Instance Deployment
# --------------------------------------------------------------------------------------------

variable "compartment_id" {
  description = "The OCID of the compartment where resources will be created"
  type        = string
}

variable "subnet_id" {
  description = "The OCID of the subnet for the instance"
  type        = string
}

variable "namespace" {
  description = "The Object Storage namespace"
  type        = string
}

variable "bucket_name" {
  description = "The Object Storage bucket name containing the image"
  type        = string
}

variable "object_name" {
  description = "The name of the image object in Object Storage"
  type        = string
}

variable "image_name" {
  description = "The display name for the custom image"
  type        = string
}

variable "operating_system" {
  description = "The operating system of the image"
  type        = string
}

variable "operating_system_version" {
  description = "The operating system version of the image"
  type        = string
  default     = ""
}

variable "instance_ad_number" {
  description = "The availability domain number where the instance will be created"
  type        = number
  default     = 1
}

variable "instance_name" {
  description = "Display name for the OCI instance"
  type        = string
  default     = "kopru-instance"
}

variable "instance_shape" {
  description = "The shape of the instance (e.g., VM.Standard.E5.Flex)"
  type        = string
  default     = "VM.Standard.E5.Flex"
}

variable "instance_ocpus" {
  description = "Number of OCPUs for flex shapes"
  type        = number
  default     = 1
}

variable "instance_memory_gb" {
  description = "Amount of memory in GB for flex shapes"
  type        = number
  default     = 12
}

variable "assign_public_ip" {
  description = "Whether to assign a public IP to the instance"
  type        = bool
  default     = true
}

variable "region" {
  description = "OCI region"
  type        = string
  default     = "eu-frankfurt-1"
}

variable "data_disk_snapshot_ids" {
  description = "List of block volume backup (snapshot) OCIDs to restore as data disks"
  type        = list(string)
  default     = []
}

variable "data_disk_names" {
  description = "List of display names for restored data disk volumes"
  type        = list(string)
  default     = []
}

variable "boot_volume_size_in_gbs" {
  description = "Size of the boot volume in GB (minimum 50GB)"
  type        = number
  default     = 50
}

variable "freeform_tags" {
  description = "Freeform tags for resources"
  type        = map(string)
  default = {
	"created-by" = "kopru"
  }
}
`
	return os.WriteFile(filepath.Join(g.config.TemplateOutputDir, "variables.tf"), []byte(content), 0644)
}

func (g *OCIGenerator) generateMainTF() error {
	// Build the base content
	var b strings.Builder
	b.WriteString(`# --------------------------------------------------------------------------------------------
# OCI Instance Configuration
# --------------------------------------------------------------------------------------------

locals {
  data_volume_names = [
	for idx in range(length(var.data_disk_snapshot_ids)) :
	length(var.data_disk_names) > idx ? var.data_disk_names[idx] : "restored-data-disk-${idx}"
  ]
}

data "oci_identity_availability_domain" "ad" {
  compartment_id = var.compartment_id
  ad_number      = var.instance_ad_number
}

`)

	// Add image import resource
	imageImportSection := fmt.Sprintf(`# --------------------------------------------------------------------------------------------
# Custom Image Import from Object Storage
# --------------------------------------------------------------------------------------------
# This resource imports the custom image from Object Storage.
# Terraform will handle the image creation and wait for it to become available.
# --------------------------------------------------------------------------------------------

resource "oci_core_image" "imported_image" {
  compartment_id = var.compartment_id
  display_name   = var.image_name
  launch_mode    = "PARAVIRTUALIZED"

  image_source_details {
    source_type = "objectStorageTuple"
    namespace_name   = var.namespace
    bucket_name      = var.bucket_name
    object_name      = var.object_name
    operating_system = var.operating_system
    operating_system_version = var.operating_system_version
  }
}

`)
	b.WriteString(imageImportSection)

	// Add UEFI capability schema resources if UEFI is enabled
	if g.config.OCIImageEnableUEFI {
		uefiSection := fmt.Sprintf(`# --------------------------------------------------------------------------------------------
# UEFI Image Capability Schema Configuration
# --------------------------------------------------------------------------------------------
# This section configures the image capability schema to enable UEFI_64 firmware for the
# imported image. This is only created when UEFI boot mode is enabled in the configuration.
# --------------------------------------------------------------------------------------------

data "oci_core_compute_global_image_capability_schemas" "image_capability_schemas" {
  compartment_id = null
}

locals {
  global_image_capability_schemas = data.oci_core_compute_global_image_capability_schemas.image_capability_schemas.compute_global_image_capability_schemas
  # Select the first available schema version, or use a default if none exist
  schema_version_name = length(local.global_image_capability_schemas) > 0 ? local.global_image_capability_schemas[0].current_version_name : "%s"
  image_schema_data = {
    "Compute.Firmware" = "%s"
  }
}

resource "oci_core_compute_image_capability_schema" "worker_image_capability_schema" {
  compartment_id                                      = var.compartment_id
  compute_global_image_capability_schema_version_name = local.schema_version_name
  image_id                                            = oci_core_image.imported_image.id
  schema_data                                         = local.image_schema_data
}

`, defaultImageCapabilitySchemaVersion, uefiSchemaData)
		b.WriteString(uefiSection)
	}

	b.WriteString(`resource "oci_core_instance" "kopru_instance" {
  compartment_id      = var.compartment_id
  availability_domain = data.oci_identity_availability_domain.ad.name
  display_name        = var.instance_name
  shape               = var.instance_shape

  dynamic "shape_config" {
	for_each = can(regex("Flex", var.instance_shape)) ? [1] : []
	content {
	  ocpus         = var.instance_ocpus
	  memory_in_gbs = var.instance_memory_gb
	}
  }

  source_details {
	source_type = "image"
	source_id   = oci_core_image.imported_image.id
	boot_volume_size_in_gbs = var.boot_volume_size_in_gbs
  }

  create_vnic_details {
	subnet_id        = var.subnet_id
	assign_public_ip = var.assign_public_ip
	display_name     = "${var.instance_name}-vnic"
  }

  lifecycle {
	prevent_destroy = false
  }

  freeform_tags = var.freeform_tags
}

resource "oci_core_volume" "data_volumes" {
  count = length(var.data_disk_snapshot_ids)
  compartment_id      = var.compartment_id
  availability_domain = data.oci_identity_availability_domain.ad.name
  display_name        = local.data_volume_names[count.index]
  source_details {
	type = "volumeBackup"
	id   = var.data_disk_snapshot_ids[count.index]
  }
  freeform_tags = var.freeform_tags
}

resource "oci_core_volume_attachment" "data_volume_attachments" {
  count = length(var.data_disk_snapshot_ids)
  attachment_type = "paravirtualized"
  instance_id     = oci_core_instance.kopru_instance.id
  volume_id       = oci_core_volume.data_volumes[count.index].id
  display_name    = "attachment-${local.data_volume_names[count.index]}"
  depends_on      = [oci_core_instance.kopru_instance]
}
`)

	return os.WriteFile(filepath.Join(g.config.TemplateOutputDir, "main.tf"), []byte(b.String()), 0644)
}

func (g *OCIGenerator) generateOutputsTF() error {
	content := `# --------------------------------------------------------------------------------------------
# Output Definitions
# --------------------------------------------------------------------------------------------

output "imported_image_id" {
  description = "The OCID of the imported custom image"
  value       = oci_core_image.imported_image.id
}

output "imported_image_state" {
  description = "The lifecycle state of the imported image"
  value       = oci_core_image.imported_image.state
}

output "instance_id" {
  description = "The OCID of the created instance"
  value       = oci_core_instance.kopru_instance.id
}

output "instance_name" {
  description = "The display name of the instance"
  value       = oci_core_instance.kopru_instance.display_name
}

output "instance_state" {
  description = "The current state of the instance"
  value       = oci_core_instance.kopru_instance.state
}

output "instance_public_ip" {
  description = "The public IP address of the instance (if assigned)"
  value       = oci_core_instance.kopru_instance.public_ip
}

output "instance_private_ip" {
  description = "The private IP address of the instance"
  value       = oci_core_instance.kopru_instance.private_ip
}

output "data_volume_ids" {
  description = "The OCIDs of the attached data volumes"
  value       = oci_core_volume.data_volumes[*].id
}

output "data_volume_attachment_ids" {
  description = "The OCIDs of the volume attachments"
  value       = oci_core_volume_attachment.data_volume_attachments[*].id
}

output "ssh_connection" {
  description = "SSH connection string"
  value = (
	oci_core_instance.kopru_instance.public_ip != null
	? "ssh -i <private-key-file> <user>@${oci_core_instance.kopru_instance.public_ip}"
	: "ssh -i <private-key-file> <user>@${oci_core_instance.kopru_instance.private_ip}"
  )
}
`
	return os.WriteFile(filepath.Join(g.config.TemplateOutputDir, "outputs.tf"), []byte(content), 0644)
}

func (g *OCIGenerator) generateTFVars() error {
	ad := g.config.OCIAvailabilityDomain
	if ad == "" {
		ad = DefaultAvailabilityDomain
	}
	
	snapshotIDsList := formatTemplateList(g.dataDiskSnapshotIDs)
	snapshotNamesList := formatTemplateList(g.dataDiskSnapshotNames)

	// Calculate boot volume size: max of 50GB or the source Azure VM boot disk size
	bootVolumeSize := int64(50)
	if g.bootVolumeSizeGB > bootVolumeSize {
		bootVolumeSize = g.bootVolumeSizeGB
	}
	
	// Select OCI shape based on architecture
	ociShape := g.selectOCIShape()
	
	// Calculate OCPU and memory based on source VM configuration
	ocpus, memoryGB := g.calculateOCIResources()

	content := fmt.Sprintf(`# --------------------------------------------------------------------------------------------
# Variable Values for OpenTofu
# --------------------------------------------------------------------------------------------
# Generated by Kopru
# Modify these values as needed before deployment
# --------------------------------------------------------------------------------------------

compartment_id      = "%s"
subnet_id           = "%s"
namespace           = "%s"
bucket_name         = "%s"
object_name         = "%s"
image_name          = "%s"
operating_system    = "%s"
operating_system_version = "%s"
instance_ad_number  = "%s"

instance_name      = "%s"
instance_shape     = "%s"
instance_ocpus     = %d
instance_memory_gb = %d
assign_public_ip   = true

boot_volume_size_in_gbs = %d

region = "%s"

data_disk_snapshot_ids = %s
data_disk_names        = %s

freeform_tags = {
  "created-by"    = "kopru"
  "source-image"  = "%s"
  "source-cpus"   = "%d"
  "source-memory-gb" = "%d"
  "source-architecture" = "%s"
}
`,
		g.config.OCICompartmentID,
		g.config.OCISubnetID,
		g.namespace,
		g.config.OCIBucketName,
		g.objectName,
		g.config.OCIImageName,
		g.config.OCIImageOS,
		g.config.OCIImageOSVersion,
		ad,
		g.config.OCIInstanceName,
		ociShape,
		ocpus,
		memoryGB,
		bootVolumeSize,
		g.config.OCIRegion,
		snapshotIDsList,
		snapshotNamesList,
		g.config.OCIImageName,
		g.vmCPUs,
		g.vmMemoryGB,
		g.vmArchitecture,
	)
	return os.WriteFile(filepath.Join(g.config.TemplateOutputDir, "terraform.tfvars"), []byte(content), 0644)
}

func (g *OCIGenerator) generateReadme() error {
	content := `# OpenTofu Configuration for OCI Instance

This directory contains OpenTofu configuration files generated by Kopru.
Use these files to deploy the imported VM in OCI.

## Files

- ` + "`provider.tf`" + ` - OCI provider configuration
- ` + "`variables.tf`" + ` - Variable definitions
- ` + "`main.tf`" + ` - Main infrastructure configuration (instance, volumes, attachments)
- ` + "`outputs.tf`" + ` - Output definitions
- ` + "`terraform.tfvars`" + ` - Variable values (customize before deployment)
- ` + "`README.md`" + ` - This file

## Usage

### 1. Review and Customize Configuration

Before deploying, review ` + "`terraform.tfvars`" + ` and adjust values as needed:

` + "```" + `hcl
# Adjust instance resources
instance_ocpus     = 2      # Number of OCPUs
instance_memory_gb = 16     # Memory in GB

# Change instance shape if needed
instance_shape = "VM.Standard.E5.Flex"
` + "```" + `

### 2. Initialize OpenTofu

` + "```" + `bash
cd template-output
tofu init
` + "```" + `

### 3. Review Deployment Plan

` + "```" + `bash
tofu plan
` + "```" + `

### 4. Deploy the Instance

` + "```" + `bash
tofu apply --auto-approve
` + "```" + `

### 5. View Outputs

After successful deployment:

` + "```" + `bash
tofu output
` + "```" + `

This will display:
- Instance OCID
- Public and private IP addresses
- SSH connection string
- Attached volume information

### 6. Connect to the Instance

` + "```" + `bash
# Using the SSH connection from outputs
ssh -i <private-key-file> <user>@<public_ip>

# Or use the output directly
$(tofu output -raw ssh_connection)
` + "```" + `

### Destroy Resources

**Warning**: This will terminate the instance and delete all attached volumes!

` + "```" + `bash
tofu destroy
` + "```" + `

`
	return os.WriteFile(filepath.Join(g.config.TemplateOutputDir, "README.md"), []byte(content), 0644)
}


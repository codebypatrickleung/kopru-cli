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

const (
	// DefaultAvailabilityDomain is the default availability domain when not specified
	DefaultAvailabilityDomain = "1"
)

// OCIGenerator handles template generation for OCI.
type OCIGenerator struct {
	config                *config.Config
	logger                *logger.Logger
	importedImageID       string
	dataDiskSnapshotIDs   []string
	dataDiskSnapshotNames []string
}

// NewOCIGenerator creates a new OCI template generator.
func NewOCIGenerator(cfg *config.Config, log *logger.Logger, importedImageID string, dataDiskSnapshotIDs []string, dataDiskSnapshotNames []string) *OCIGenerator {
	return &OCIGenerator{
		config:                cfg,
		logger:                log,
		importedImageID:       importedImageID,
		dataDiskSnapshotIDs:   dataDiskSnapshotIDs,
		dataDiskSnapshotNames: dataDiskSnapshotNames,
	}
}

// formatTemplateList converts a string slice to template list format
func formatTemplateList(items []string) string {
	if len(items) == 0 {
		return "[]"
	}

	var result strings.Builder
	result.WriteString("[\n")
	for i, item := range items {
		result.WriteString(fmt.Sprintf("  \"%s\"", item))
		if i < len(items)-1 {
			result.WriteString(",\n")
		} else {
			result.WriteString("\n")
		}
	}
	result.WriteString("]")
	return result.String()
}

// GenerateTemplate generates all template configuration files.
func (g *OCIGenerator) GenerateTemplate() error {
	// Create template output directory
	if err := common.EnsureDir(g.config.TemplateOutputDir); err != nil {
		return fmt.Errorf("failed to create template output directory: %w", err)
	}

	g.logger.Infof("Generating template files in: %s", g.config.TemplateOutputDir)

	// Generate provider.tf
	if err := g.generateProviderTF(); err != nil {
		return fmt.Errorf("failed to generate provider.tf: %w", err)
	}

	// Generate variables.tf
	if err := g.generateVariablesTF(); err != nil {
		return fmt.Errorf("failed to generate variables.tf: %w", err)
	}

	// Generate main.tf
	if err := g.generateMainTF(); err != nil {
		return fmt.Errorf("failed to generate main.tf: %w", err)
	}

	// Generate outputs.tf
	if err := g.generateOutputsTF(); err != nil {
		return fmt.Errorf("failed to generate outputs.tf: %w", err)
	}

	// Generate terraform.tfvars
	if err := g.generateTFVars(); err != nil {
		return fmt.Errorf("failed to generate terraform.tfvars: %w", err)
	}

	// Generate README.md
	if err := g.generateReadme(); err != nil {
		return fmt.Errorf("failed to generate README.md: %w", err)
	}

	g.logger.Successf("Template generated in %s", g.config.TemplateOutputDir)
	return nil
}

// DeployTemplate executes OpenTofu commands to deploy the infrastructure.
func (g *OCIGenerator) DeployTemplate() error {
	// Check if tofu is available
	if err := common.CheckCommand("tofu"); err != nil {
		return fmt.Errorf("tofu not found: %w", err)
	}

	templateDir := g.config.TemplateOutputDir

	// Initialize OpenTofu
	g.logger.Info("Running tofu init...")
	output, err := common.RunCommand("tofu", "-chdir="+templateDir, "init")
	if err != nil {
		return fmt.Errorf("tofu init failed: %w\nOutput: %s", err, output)
	}
	g.logger.Success("✓ OpenTofu initialized")

	// Run tofu plan
	g.logger.Info("Running tofu plan...")
	output, err = common.RunCommand("tofu", "-chdir="+templateDir, "plan", "-out=tfplan")
	if err != nil {
		return fmt.Errorf("tofu plan failed: %w\nOutput: %s", err, output)
	}
	g.logger.Success("✓ OpenTofu plan created")

	// Run tofu apply
	g.logger.Info("Running tofu apply (this may take several minutes)...")
	output, err = common.RunCommand("tofu", "-chdir="+templateDir, "apply", "-auto-approve", "tfplan")
	if err != nil {
		return fmt.Errorf("tofu apply failed: %w\nOutput: %s", err, output)
	}

	g.logger.Success("Instance deployed with OpenTofu")
	g.logger.Infof("Run 'tofu output' in %s to see instance details", templateDir)
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

# OCI Provider - uses default configuration from ~/.oci/config or environment variables
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

# --------------------------------------------------------------------------------------------
# Required Variables
# --------------------------------------------------------------------------------------------

variable "compartment_id" {
  description = "The OCID of the compartment where resources will be created"
  type        = string
}

variable "subnet_id" {
  description = "The OCID of the subnet for the instance"
  type        = string
}

variable "image_id" {
  description = "The OCID of the custom image to use for the instance"
  type        = string
}

variable "instance_ad_number" {
  description = "The availability domain number where the instance will be created"
  type        = number
  default     = 1
}

# --------------------------------------------------------------------------------------------
# Instance Configuration
# --------------------------------------------------------------------------------------------

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

# --------------------------------------------------------------------------------------------
# Region Configuration
# --------------------------------------------------------------------------------------------

variable "region" {
  description = "OCI region"
  type        = string
  default     = "eu-frankfurt-1"
}

# --------------------------------------------------------------------------------------------
# Data Disk Snapshots
# --------------------------------------------------------------------------------------------

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

# --------------------------------------------------------------------------------------------
# Tags
# --------------------------------------------------------------------------------------------

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
	content := `# --------------------------------------------------------------------------------------------
# OCI Instance Configuration
# --------------------------------------------------------------------------------------------
# This file defines the OCI compute instance and associated resources.
# --------------------------------------------------------------------------------------------

# --------------------------------------------------------------------------------------------
# Locals
# --------------------------------------------------------------------------------------------

locals {
  # Generate volume names from provided list or use default naming
  data_volume_names = [
    for idx in range(length(var.data_disk_snapshot_ids)) :
    length(var.data_disk_names) > idx ? var.data_disk_names[idx] : "restored-data-disk-${idx}"
  ]
}

# --------------------------------------------------------------------------------------------
# Data
# --------------------------------------------------------------------------------------------

data "oci_identity_availability_domain" "ad" {
  compartment_id = var.compartment_id
  ad_number      = var.instance_ad_number
}

# --------------------------------------------------------------------------------------------
# Compute Instance
# --------------------------------------------------------------------------------------------

resource "oci_core_instance" "kopru_instance" {
  compartment_id      = var.compartment_id
  availability_domain = data.oci_identity_availability_domain.ad.name
  display_name        = var.instance_name
  shape               = var.instance_shape

  # Flex shape configuration
  dynamic "shape_config" {
    for_each = can(regex("Flex", var.instance_shape)) ? [1] : []
    content {
      ocpus         = var.instance_ocpus
      memory_in_gbs = var.instance_memory_gb
    }
  }

  # Use the imported custom image
  source_details {
    source_type = "image"
    source_id   = var.image_id
  }

  # Network configuration
  create_vnic_details {
    subnet_id        = var.subnet_id
    assign_public_ip = var.assign_public_ip
    display_name     = "${var.instance_name}-vnic"
  }

  # Lifecycle configuration
  # Note: prevent_destroy is set to false to allow Terraform destroy operations
  # Set to true in production environments to prevent accidental deletion
  lifecycle {
    prevent_destroy = false
  }

  freeform_tags = var.freeform_tags
}

# --------------------------------------------------------------------------------------------
# Block Volumes from Snapshots
# --------------------------------------------------------------------------------------------

# Create block volumes from snapshots
resource "oci_core_volume" "data_volumes" {
  count = length(var.data_disk_snapshot_ids)

  compartment_id      = var.compartment_id
  availability_domain = data.oci_identity_availability_domain.ad.name
  display_name        = local.data_volume_names[count.index]
  
  # Restore from snapshot (volume backup)
  source_details {
    type = "volumeBackup"
    id   = var.data_disk_snapshot_ids[count.index]
  }

  freeform_tags = var.freeform_tags
}

# Attach block volumes to the instance
resource "oci_core_volume_attachment" "data_volume_attachments" {
  count = length(var.data_disk_snapshot_ids)

  attachment_type = "paravirtualized"
  instance_id     = oci_core_instance.kopru_instance.id
  volume_id       = oci_core_volume.data_volumes[count.index].id
  display_name    = "attachment-${local.data_volume_names[count.index]}"

  # Wait for instance to be available before attaching volumes
  depends_on = [oci_core_instance.kopru_instance]
}
`
	return os.WriteFile(filepath.Join(g.config.TemplateOutputDir, "main.tf"), []byte(content), 0644)
}

func (g *OCIGenerator) generateOutputsTF() error {
	content := `# --------------------------------------------------------------------------------------------
# Output Definitions
# --------------------------------------------------------------------------------------------

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
	// Get availability domain
	ad := g.config.OCIAvailabilityDomain
	if ad == "" {
		ad = DefaultAvailabilityDomain
	}

	// Use the imported image ID if available, otherwise use placeholder
	imageID := "REPLACE_WITH_IMPORTED_IMAGE_OCID"
	imageIDComment := ""
	if g.importedImageID != "" {
		imageID = g.importedImageID
	} else {
		// Add comment about how to find the image OCID
		imageIDComment = fmt.Sprintf(`# IMPORTANT: Replace the placeholder below with the actual image OCID after import completes
# You can find the image OCID in the OCI console or by running:
#   oci compute image list --compartment-id %s --display-name "%s"
`, g.config.OCICompartmentID, g.config.OCIImageName)
	}

	// Format data disk snapshot IDs and names as template lists
	snapshotIDsList := formatTemplateList(g.dataDiskSnapshotIDs)
	snapshotNamesList := formatTemplateList(g.dataDiskSnapshotNames)

	content := fmt.Sprintf(`# --------------------------------------------------------------------------------------------
# Variable Values for OpenTofu
# --------------------------------------------------------------------------------------------
# Generated by Kopru
# Modify these values as needed before deployment
# --------------------------------------------------------------------------------------------

# Required Configuration
compartment_id      = "%s"
subnet_id           = "%s"
%simage_id            = "%s"
instance_ad_number = "%s"

# Instance Configuration
instance_name      = "%s"
instance_shape     = "VM.Standard.E5.Flex"
instance_ocpus     = 1
instance_memory_gb = 12
assign_public_ip   = true

# Region Configuration
region = "%s"

# Data Disk Snapshots (restored as block volumes and attached to instance)
data_disk_snapshot_ids = %s
data_disk_names        = %s

# Tags
freeform_tags = {
  "created-by"    = "kopru"
  "source-image"  = "%s"
}
`,
		g.config.OCICompartmentID,
		g.config.OCISubnetID,
		imageIDComment,
		imageID,
		ad,
		g.config.OCIInstanceName,
		g.config.OCIRegion,
		snapshotIDsList,
		snapshotNamesList,
		imageID,
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

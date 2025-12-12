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

// OCIGenerator handles template generation for OCI.
type OCIGenerator struct {
	config                *config.Config
	logger                *logger.Logger
	importedImageID       string
	dataDiskSnapshotIDs   []string
	dataDiskSnapshotNames []string
	bootVolumeSizeGB      int64
}

// NewOCIGenerator creates a new OCI template generator.
func NewOCIGenerator(cfg *config.Config, log *logger.Logger, importedImageID string, dataDiskSnapshotIDs, dataDiskSnapshotNames []string, bootVolumeSizeGB int64) *OCIGenerator {
	return &OCIGenerator{
		config:                cfg,
		logger:                log,
		importedImageID:       importedImageID,
		dataDiskSnapshotIDs:   dataDiskSnapshotIDs,
		dataDiskSnapshotNames: dataDiskSnapshotNames,
		bootVolumeSizeGB:      bootVolumeSizeGB,
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
		msg    string
		args   []string
		succ   string
	}{
		{"Running tofu init...", []string{"-chdir=" + dir, "init"}, "✓ OpenTofu initialized"},
		{"Running tofu plan...", []string{"-chdir=" + dir, "plan", "-out=tfplan"}, "✓ OpenTofu plan created"},
		{"Running tofu apply (this may take several minutes)...", []string{"-chdir=" + dir, "apply", "-auto-approve", "tfplan"}, "Instance deployed with OpenTofu"},
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

variable "image_id" {
  description = "The OCID of the custom image to use for the instance"
  type        = string
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
	content := `# --------------------------------------------------------------------------------------------
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

resource "oci_core_instance" "kopru_instance" {
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
	source_id   = var.image_id
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
	ad := g.config.OCIAvailabilityDomain
	if ad == "" {
		ad = DefaultAvailabilityDomain
	}
	imageID := "REPLACE_WITH_IMPORTED_IMAGE_OCID"
	imageIDComment := ""
	if g.importedImageID != "" {
		imageID = g.importedImageID
	} else {
		imageIDComment = fmt.Sprintf(`# IMPORTANT: Replace the placeholder below with the actual image OCID after import completes
# You can find the image OCID in the OCI console or by running:
#   oci compute image list --compartment-id %s --display-name "%s"
`, g.config.OCICompartmentID, g.config.OCIImageName)
	}
	snapshotIDsList := formatTemplateList(g.dataDiskSnapshotIDs)
	snapshotNamesList := formatTemplateList(g.dataDiskSnapshotNames)

	// Calculate boot volume size: max of 50GB or the source Azure VM boot disk size
	bootVolumeSize := int64(50)
	if g.bootVolumeSizeGB > bootVolumeSize {
		bootVolumeSize = g.bootVolumeSizeGB
	}

	content := fmt.Sprintf(`# --------------------------------------------------------------------------------------------
# Variable Values for OpenTofu
# --------------------------------------------------------------------------------------------
# Generated by Kopru
# Modify these values as needed before deployment
# --------------------------------------------------------------------------------------------

compartment_id      = "%s"
subnet_id           = "%s"
%simage_id            = "%s"
instance_ad_number  = "%s"

instance_name      = "%s"
instance_shape     = "VM.Standard.E5.Flex"
instance_ocpus     = 1
instance_memory_gb = 12
assign_public_ip   = true

boot_volume_size_in_gbs = %d

region = "%s"

data_disk_snapshot_ids = %s
data_disk_names        = %s

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
		bootVolumeSize,
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


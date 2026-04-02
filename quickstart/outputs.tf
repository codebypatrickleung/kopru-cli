# ==============================================================================
# OUTPUTS
# ==============================================================================

locals {
  kopru_init_log_path = "/var/log/kopru-init.log"
}

output "instance_id" {
  description = "OCID of the created compute instance"
  value       = oci_core_instance.kopru_instance.id
}

output "instance_display_name" {
  description = "Display name of the compute instance"
  value       = oci_core_instance.kopru_instance.display_name
}

output "instance_public_ip" {
  description = "Public IP address of the compute instance"
  value       = oci_core_instance.kopru_instance.public_ip
}

output "instance_private_ip" {
  description = "Private IP address of the compute instance"
  value       = oci_core_instance.kopru_instance.private_ip
}

output "ssh_connection_command" {
  description = "SSH command to connect to the instance"
  value       = "ssh opc@${oci_core_instance.kopru_instance.public_ip}"
}

output "oci_region" {
  description = "OCI Region where resources are created"
  value       = var.region
}

output "boot_volume_id" {
  description = "OCID of the boot volume attached to the instance"
  value       = oci_core_instance.kopru_instance.boot_volume_id
}

output "data_volume_ids" {
  description = "OCIDs of the created data block volumes"
  value       = oci_core_volume.data_volume[*].id
}

output "data_volume_attachment_ids" {
  description = "OCIDs of the data volume attachments"
  value       = oci_core_volume_attachment.data_volume_attachment[*].id
}

output "dynamic_group_id" {
  description = "OCID of the Kopru dynamic group for instance principal authentication"
  value       = oci_identity_dynamic_group.kopru_dynamic_group.id
}

output "policy_id" {
  description = "OCID of the Kopru IAM policy"
  value       = oci_identity_policy.kopru_policy.id
}

output "kopru_init_log_path" {
  description = "Path to the Kopru CLI initialization log on the instance"
  value       = local.kopru_init_log_path
}
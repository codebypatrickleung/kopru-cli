# ==============================================================================
# TERRAFORM CONFIGURATION
# ==============================================================================

terraform {
  required_providers {
    oci = {
      source = "oracle/oci"
    }
  }
}

# ==============================================================================
# DATA SOURCES
# ==============================================================================

data "oci_core_images" "oracle_linux_9" {
  compartment_id           = var.kopru_compartment_ocid
  operating_system         = "Oracle Linux"
  operating_system_version = "9"
  shape                    = var.shape
  sort_by                  = "TIMECREATED"
  sort_order               = "DESC"
}

data "oci_identity_availability_domain" "ad" {
  compartment_id = var.kopru_compartment_ocid
  ad_number      = var.instance_ad_number
}

# ==============================================================================
# COMPUTE INSTANCE
# ==============================================================================

resource "oci_core_instance" "kopru_instance" {
  display_name        = "${var.display_name}-instance"
  availability_domain = data.oci_identity_availability_domain.ad.name
  compartment_id      = var.kopru_compartment_ocid
  shape               = var.shape

  shape_config {
    ocpus         = var.instance_ocpus
    memory_in_gbs = var.instance_memory_in_gbs
  }

  create_vnic_details {
    assign_public_ip = var.assign_public_ip
    subnet_id        = var.subnet_ocid
  }

  launch_options {
    network_type = "VFIO"
  }

  source_details {
    source_type             = "image"
    source_id               = data.oci_core_images.oracle_linux_9.images[0].id
    boot_volume_size_in_gbs = var.boot_volume_size_in_gbs
  }

  metadata = {
    ssh_authorized_keys = var.ssh_public_key
    user_data           = base64encode(file("${path.module}/user-data.sh"))
  }

  agent_config {
    are_all_plugins_disabled = false
    is_management_disabled   = false
    is_monitoring_disabled   = false

    plugins_config {
      desired_state = "ENABLED"
      name          = "Compute Instance Monitoring"
    }
    plugins_config {
      desired_state = "ENABLED"
      name          = "Block Volume Management"
    }
  }

  freeform_tags = var.freeform_tags
  defined_tags  = var.defined_tags
}

# ==============================================================================
# INSTANCE PRINCIPAL - DYNAMIC GROUP AND POLICY
# ==============================================================================

resource "oci_identity_dynamic_group" "kopru_dynamic_group" {
  compartment_id = var.tenancy_ocid
  name           = "${var.display_name}-dynamic-group"
  description    = "Dynamic group for Kopru CLI instance principal authentication"
  matching_rule  = "Any {instance.id = '${oci_core_instance.kopru_instance.id}'}"
  freeform_tags  = var.freeform_tags
  defined_tags   = var.defined_tags
}

resource "oci_identity_policy" "kopru_policy" {
  compartment_id = var.kopru_compartment_ocid
  name           = "${var.display_name}-policy"
  description    = "Policy granting Kopru CLI instance principal permissions to perform migrations"
  statements = [
    "Allow dynamic-group ${oci_identity_dynamic_group.kopru_dynamic_group.name} to manage object-family in compartment id ${var.kopru_compartment_ocid}",
    "Allow dynamic-group ${oci_identity_dynamic_group.kopru_dynamic_group.name} to manage instance-images in compartment id ${var.kopru_compartment_ocid}",
    "Allow dynamic-group ${oci_identity_dynamic_group.kopru_dynamic_group.name} to manage instance-family in compartment id ${var.kopru_compartment_ocid}",
    "Allow dynamic-group ${oci_identity_dynamic_group.kopru_dynamic_group.name} to manage volume-family in compartment id ${var.kopru_compartment_ocid}",
    "Allow dynamic-group ${oci_identity_dynamic_group.kopru_dynamic_group.name} to manage virtual-network-family in compartment id ${var.kopru_compartment_ocid}",
    "Allow dynamic-group ${oci_identity_dynamic_group.kopru_dynamic_group.name} to read compartments in compartment id ${var.kopru_compartment_ocid}",
  ]
  freeform_tags = var.freeform_tags
  defined_tags  = var.defined_tags
}

# ==============================================================================
# BLOCK STORAGE
# ==============================================================================

resource "oci_core_volume" "data_volume" {
  count                = var.data_volume_count
  availability_domain  = data.oci_identity_availability_domain.ad.name
  compartment_id       = var.kopru_compartment_ocid
  display_name         = "${var.display_name}-data-volume-${count.index}"
  size_in_gbs          = var.data_volume_size_in_gbs
  vpus_per_gb          = 30
  is_auto_tune_enabled = true
  freeform_tags        = var.freeform_tags
  defined_tags         = var.defined_tags
}

resource "oci_core_volume_attachment" "data_volume_attachment" {
  count           = var.data_volume_count
  instance_id     = oci_core_instance.kopru_instance.id
  volume_id       = oci_core_volume.data_volume[count.index].id
  attachment_type = "PARAVIRTUALIZED"
  display_name    = "${var.display_name}-data-volume-attachment-${count.index}"
}
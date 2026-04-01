# ==============================================================================
# VARIABLES
# ==============================================================================

variable "tenancy_ocid" {
  type        = string
  description = "The OCID of the tenancy"
}

variable "kopru_compartment_ocid" {
  type        = string
  description = "The OCID of the compartment"
}

variable "shape" {
  type        = string
  description = "The shape of the instance"
  default     = "VM.Optimized3.Flex"
}

variable "instance_ad_number" {
  type        = number
  description = "The availability domain number"
  default     = 1
}
variable "region" {
  description = "The OCI region where resources will be created"
  type        = string
}

variable "instance_ocpus" {
  type        = number
  description = "The number of OCPUs"
  default     = 6
}

variable "instance_memory_in_gbs" {
  type        = number
  description = "The amount of memory in GB"
  default     = 12
}

variable "assign_public_ip" {
  type        = bool
  description = "Whether to assign a public IP"
}

variable "subnet_ocid" {
  type        = string
  description = "The OCID of the subnet"
}

variable "ssh_public_key" {
  type        = string
  description = "The SSH public key"
}

variable "boot_volume_size_in_gbs" {
  type        = number
  description = "The size of the boot volume in GB"
  default     = 100
}

variable "data_volume_size_in_gbs" {
  type        = number
  description = "The size of the data volume in GB"
  default     = 500
}

variable "data_volume_count" {
  type        = number
  default     = 4
  description = "The number of data volumes"
}

variable "display_name" {
  type        = string
  description = "The display name of the instance"
  default     = "kopru"
}

variable "freeform_tags" {
  type        = map(string)
  default     = {}
  description = "The freeform tags"
}

variable "defined_tags" {
  type        = map(string)
  default     = {}
  description = "The defined tags"
}
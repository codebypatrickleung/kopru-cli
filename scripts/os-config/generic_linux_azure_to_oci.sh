#!/bin/bash
# Linux Azure to OCI OS Configuration Script
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

MOUNT_DIR="${1:-${KOPRU_MOUNT_DIR:-}}"
if [[ -z "$MOUNT_DIR" ]]; then
    log_warning "Mount directory not provided"
    echo "Usage: $0 <mount_dir>"
    exit 1
fi
if [[ ! -d "$MOUNT_DIR" ]]; then
    log_warning "Mount directory does not exist: $MOUNT_DIR"
    exit 1
fi

main() {
    log_info "Starting generic Linux Azure to OCI configuration..."
    local os_family os_version
    os_family=$(detect_os_family)
    log_info "Detected OS family: $os_family"
    os_version=$(detect_os_version)
    log_info "Detected OS version: $os_version"

    log_info "=== Phase 1: Disabling Azure-specific configurations ==="
    disable_azure_udev_rules           || log_warning "Failed to disable Azure udev rules, continuing..."
    disable_azure_cloudinit_datasource || log_warning "Failed to disable Azure cloud-init datasource, continuing..."
    disable_azure_chrony_refclock      || log_warning "Failed to disable Azure chrony refclock, continuing..."
    disable_azure_hyperv_daemon        || log_warning "Failed to disable Hyper-V daemon, continuing..."
    disable_azure_linux_agent          || log_warning "Failed to disable Azure Linux agent, continuing..."

    log_info "=== Phase 2: Adding OCI-specific configurations ==="
    add_oci_chrony_config              || log_warning "Failed to add OCI chrony config, continuing..."
    add_oci_cloudinit_datasource       || log_warning "Failed to add OCI cloud-init datasource, continuing..."
    add_ssh_host_keys_fix              || log_warning "Failed to add SSH host keys fix, continuing..."
    auto_relabel_selinux_contexts      || log_warning "Failed to relabel SELinux contexts, continuing..."

    log_success "Generic Linux Azure to OCI configuration complete"
    log_info "Configuration was successful for OS family: $os_family"
}

main
exit 0

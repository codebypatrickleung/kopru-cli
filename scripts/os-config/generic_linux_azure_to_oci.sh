#!/bin/bash
# Linux Azure to OCI OS Configuration Script

set -euo pipefail

export LIBGUESTFS_BACKEND=direct

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

IMAGE_FILE="${1:-${KOPRU_IMAGE_FILE:-}}"
if [[ -z "$IMAGE_FILE" ]]; then
    log_error "Image file not provided"
    echo "Usage: $0 <image_file>"
    exit 1
fi

if [[ ! -f "$IMAGE_FILE" ]]; then
    log_error "Image file does not exist: $IMAGE_FILE"
    exit 1
fi

main() {
    log_info "Starting Azure to OCI configuration..."
    log_info "Image file: $IMAGE_FILE"

    local os_info os_family os_version guest_arch
    os_info=$(detect_os_info_from_image)
    os_family=$(echo "$os_info" | cut -d'|' -f1)
    os_version=$(echo "$os_info" | cut -d'|' -f2)
    log_info "Detected OS family: $os_family"
    log_info "Detected OS version: $os_version"

    guest_arch=$(detect_guest_architecture "$IMAGE_FILE")
    log_info "Detected guest architecture: $guest_arch"

    log_info "=== Applying OS configurations ==="
    log_info "Phase 1: Disabling Azure-specific configurations..."
    disable_azure_cloud_init "$IMAGE_FILE" "$os_family"
    disable_azure_chrony "$IMAGE_FILE" "$os_family"
    disable_azure_hyperv_daemons "$IMAGE_FILE" "$os_family"
    disable_azure_agent "$IMAGE_FILE" "$os_family"
    disable_azure_temp_disk_warning "$IMAGE_FILE" "$os_family"

    log_info "Phase 2: Adding OCI-specific configurations..."
    add_oci_chrony_config "$IMAGE_FILE" "$os_family"
    add_oci_cloud_init "$IMAGE_FILE" "$os_family"
    fix_ssh_host_keys "$IMAGE_FILE" "$os_family"
    cloud_init_clean "$IMAGE_FILE" "$os_family"

    log_info "=== OS configurations complete ==="
}

main

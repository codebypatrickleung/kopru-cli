#!/bin/bash
# Linux Image to OCI OS Configuration Script

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

OS_INFO=$(detect_os_info_from_image)
OS_FAMILY=$(echo "$OS_INFO" | cut -d'|' -f1)
OS_VERSION=$(echo "$OS_INFO" | cut -d'|' -f2)
OS_ID=$(echo "$OS_INFO" | cut -d'|' -f3)
log_info "Detected OS family: $OS_FAMILY"
log_info "Detected OS version: $OS_VERSION"
log_info "Detected OS ID: $OS_ID"

main() {
    log_info "Starting Linux Image to OCI configuration..."
    log_info "Image file: $IMAGE_FILE"
    log_info "OS Family: $OS_FAMILY"
    log_info "OS Version: $OS_VERSION"
    log_info "OS ID: $OS_ID"
    log_info "=== Applying Linux Image to OCI configurations ==="
    add_oci_cloud_init "$IMAGE_FILE" "$OS_FAMILY" "$OS_ID"
    
    if [[ "$OS_ID" == "debian" ]]; then
        log_info "=== Configuring iSCSI for Debian OS ==="
        install_iscsi_initiator "$IMAGE_FILE"
        rebuild_iscsi_initramfs "$IMAGE_FILE"
        configure_fstab_netdev "$IMAGE_FILE"
        configure_iscsi_automatic_startup "$IMAGE_FILE"
    fi
    
    cloud_init_clean "$IMAGE_FILE" "$OS_FAMILY"
    log_info "=== Linux Image to OCI configuration complete ==="
}

main

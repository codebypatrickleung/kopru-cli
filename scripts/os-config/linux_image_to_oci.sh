#!/bin/bash
# Linux Image to OCI OS Configuration Script
#
# This script supports multiple Linux distributions including:
# - Debian

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

# Detect OS information from the image
OS_INFO=$(detect_os_info_from_image)
OS_FAMILY=$(echo "$OS_INFO" | cut -d'|' -f1)
OS_VERSION=$(echo "$OS_INFO" | cut -d'|' -f2)
OS_ID=$(echo "$OS_INFO" | cut -d'|' -f3)
log_info "Detected OS family: $OS_FAMILY"
log_info "Detected OS version: $OS_VERSION"
log_info "Detected OS ID: $OS_ID"

install_iscsi_initiator() {
    local image_file=$1
    log_info "Installing iSCSI initiator..."
    
    local iscsi_package
    case "$OS_FAMILY" in
        debian)
            iscsi_package="open-iscsi"
            ;;
        rhel)
            iscsi_package="iscsi-initiator-utils"
            ;;
        *)
            log_warning "Unknown OS family: $OS_FAMILY, attempting with open-iscsi"
            iscsi_package="open-iscsi"
            ;;
    esac
    
    log_info "Installing iSCSI package: $iscsi_package"
    if ! virt-customize -a "$image_file" --install "$iscsi_package" &>/dev/null; then
        log_warning "Failed to install $iscsi_package package"
    fi

    if [[ "$OS_FAMILY" == "rhel" ]]; then
        log_info "Installing dracut-network package"
        if ! virt-customize -a "$image_file" --install "dracut-network" &>/dev/null; then
            log_warning "Failed to install dracut-network package"
        fi
    fi

    log_success "iSCSI initiator installed successfully"
}

rebuild_iscsi_initramfs() {
    local image_file=$1
    log_info "Configuring iSCSI in initramfs..."
    
    case "$OS_FAMILY" in
        debian)
            virt-customize -a "$image_file" --run-command "
                mkdir -p /etc/iscsi
                echo 'ISCSI_AUTO=true' > /etc/iscsi/iscsi.initramfs
                grep -q '^iscsi_ibft$' /etc/initramfs-tools/modules || echo 'iscsi_ibft' >> /etc/initramfs-tools/modules
                grep -q '^iscsi_tcp$' /etc/initramfs-tools/modules || echo 'iscsi_tcp' >> /etc/initramfs-tools/modules
                grep -q '^libiscsi$' /etc/initramfs-tools/modules || echo 'libiscsi' >> /etc/initramfs-tools/modules
                update-initramfs -u
            " &>/dev/null || log_warning "Failed to configure iSCSI in initramfs (Debian/Ubuntu)"
            ;;
        rhel)
            virt-customize -a "$image_file" --run-command "
                mkdir -p /etc/iscsi
                echo 'node.startup = automatic' >> /etc/iscsi/iscsid.conf || true
                # Add iSCSI modules to dracut
                mkdir -p /etc/dracut.conf.d
                echo 'add_drivers+=\" iscsi_tcp iscsi_ibft \"' > /etc/dracut.conf.d/iscsi.conf
                echo 'add_dracutmodules+=\" iscsi \"' >> /etc/dracut.conf.d/iscsi.conf
                # Rebuild initramfs for all installed kernels
                dracut -f --regenerate-all || dracut -f
            " &>/dev/null || log_warning "Failed to configure iSCSI in initramfs (RHEL/Fedora)"
            ;;
        *)
            log_warning "Unknown OS family: $OS_FAMILY, skipping iSCSI configuration"
    esac
    log_success "iSCSI configured in initramfs successfully"
}

configure_fstab_netdev() {
    local image_file=$1
    log_info "Configuring /etc/fstab with _netdev and x-systemd.requires mount options..."
    virt-customize -a "$image_file" --run-command "
        cp /etc/fstab /etc/fstab.backup
        awk 'BEGIN{OFS=FS}
            /^[^#]/ && \$2 == \"/\" && \$4 !~ /_netdev/ {
                \$4 = \$4 \",_netdev,x-systemd.requires=iscsid.service\"
            }
            /^[^#]/ && \$2 == \"/boot/efi\" && \$4 !~ /_netdev/ {
                \$4 = \$4 \",_netdev\"
            }
            {print}' /etc/fstab > /etc/fstab.new && mv /etc/fstab.new /etc/fstab
    " &>/dev/null || log_warning "Failed to configure fstab with _netdev and x-systemd.requires options"
    log_success "/etc/fstab configured with _netdev and x-systemd.requires mount options"
}

configure_iscsi_automatic_startup() {
    local image_file=$1
    log_info "Configuring iSCSI automatic startup..."
    virt-customize -a "$image_file" --run-command "
        if [ -f /etc/iscsi/iscsid.conf ]; then
            sed -i 's/^node.startup = manual$/node.startup = automatic/' /etc/iscsi/iscsid.conf
        else
            mkdir -p /etc/iscsi
            echo 'node.startup = automatic' >> /etc/iscsi/iscsid.conf
        fi
    " &>/dev/null || log_warning "Failed to configure iSCSI automatic startup"
    
    log_success "iSCSI automatic startup configured"
}

main() {
    log_info "Starting Linux Image to OCI configuration..."
    log_info "Image file: $IMAGE_FILE"
    log_info "OS Family: $OS_FAMILY"
    log_info "OS Version: $OS_VERSION"
    log_info "=== Applying Linux Image to OCI configurations ==="
    install_iscsi_initiator "$IMAGE_FILE"
    rebuild_iscsi_initramfs "$IMAGE_FILE"
    configure_fstab_netdev "$IMAGE_FILE"
    configure_iscsi_automatic_startup "$IMAGE_FILE"
    add_oci_cloud_init "$IMAGE_FILE" "$OS_FAMILY" "$OS_ID"
    cloud_init_clean "$IMAGE_FILE" "$OS_FAMILY"
    log_info "=== Linux Image to OCI configuration complete ==="
}

main

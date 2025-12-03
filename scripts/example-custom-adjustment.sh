#!/bin/bash
# Sample custom OS adjustment script for Kopru
# This script demonstrates how to create a custom adjustment script
# for use with OCI_IMAGE_OS=CUSTOM
#
# Usage: This script will be called by Kopru with the mount directory as the first argument
#        The mount directory is also available via KOPRU_MOUNT_DIR environment variable
#
# Example:
#   export OCI_IMAGE_OS=CUSTOM
#   export CUSTOM_OS_ADJUSTMENT_SCRIPT=/path/to/this/script.sh
#   ./kopru --config kopru-config.env

set -euo pipefail

# Get mount directory from argument or environment variable
MOUNT_DIR="${1:-${KOPRU_MOUNT_DIR:-}}"

if [[ -z "$MOUNT_DIR" ]]; then
    echo "ERROR: Mount directory not provided"
    echo "Usage: $0 <mount_dir>"
    exit 1
fi

if [[ ! -d "$MOUNT_DIR" ]]; then
    echo "ERROR: Mount directory does not exist: $MOUNT_DIR"
    exit 1
fi

echo "Running custom OS adjustments on mounted filesystem: $MOUNT_DIR"

# Example 1: Remove cloud provider-specific agents
echo "Removing Azure WALinux Agent..."
sudo rm -rf "$MOUNT_DIR/var/lib/waagent" 2>/dev/null || true
sudo rm -f "$MOUNT_DIR/usr/sbin/waagent" 2>/dev/null || true

# Example 2: Configure cloud-init for OCI
echo "Configuring cloud-init for OCI..."
sudo mkdir -p "$MOUNT_DIR/etc/cloud/cloud.cfg.d"
echo "datasource_list: [Oracle, None]" | sudo tee "$MOUNT_DIR/etc/cloud/cloud.cfg.d/99_oci.cfg" > /dev/null

# Example 3: Update network configuration
echo "Updating network configuration..."
if [[ -f "$MOUNT_DIR/etc/netplan/50-cloud-init.yaml" ]]; then
    sudo sed -i 's/ens[0-9]\+/eth0/g' "$MOUNT_DIR/etc/netplan/50-cloud-init.yaml"
fi

# Example 4: Clear machine-id for regeneration
echo "Clearing machine-id..."
if [[ -f "$MOUNT_DIR/etc/machine-id" ]]; then
    echo "" | sudo tee "$MOUNT_DIR/etc/machine-id" > /dev/null
fi

# Example 5: Update GRUB for OCI console
echo "Updating GRUB configuration..."
if [[ -f "$MOUNT_DIR/etc/default/grub" ]]; then
    if ! grep -q "console=ttyS0" "$MOUNT_DIR/etc/default/grub"; then
        sudo sed -i '/^GRUB_CMDLINE_LINUX="/ s/"$/ console=ttyS0,115200"/' "$MOUNT_DIR/etc/default/grub"
    fi
fi

# Add your custom adjustments here
# For example:
# - Install additional packages (using chroot)
# - Modify configuration files
# - Add custom scripts or services
# - etc.

echo "Custom OS adjustments completed successfully"
exit 0

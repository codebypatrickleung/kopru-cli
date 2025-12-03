#!/bin/bash
# --------------------------------------------------------------------------------------------
# Kopru Environment Setup Script
# --------------------------------------------------------------------------------------------
# This script sets up the environment for using Kopru by installing required tools 
# such as QEMU, OpenTofu, Azure CLI, and OCI CLI.
#
# Supported Operating Systems: Oracle Linux 9 and above
#
# Usage: ./setup-environment.sh
#   Installs all required tools for Kopru.
#
# --------------------------------------------------------------------------------------------
# System Commands Required by Kopru:
# --------------------------------------------------------------------------------------------
# Core System Utilities (should be pre-installed):
#   - sudo: Run commands with elevated privileges
#   - df: Check disk space availability
#   - lsblk: List block devices
#   - blkid: Check filesystem types
#   - mount/umount: Mount and unmount filesystems
#   - modprobe: Load kernel modules (NBD)
#   - mkdir/rm/mv: File and directory operations
#   - dd: Disk data copying
#
# QEMU Tools (installed by this script):
#   - qemu-img: Convert and resize disk images
#   - qemu-nbd: Network block device operations
#
# Infrastructure Tools (installed by this script):
#   - tofu (OpenTofu): Infrastructure as Code tool
#
# OCI Instance Tools (pre-installed on OCI instances):
#   - oci-metadata: Query OCI instance metadata
# --------------------------------------------------------------------------------------------

set -e

# Check for Oracle Linux 9 and above
check_oracle_linux_version() {
    if [[ ! -f /etc/oracle-release ]]; then
        echo "Error: This script only supports Oracle Linux."
        echo "Please run Kopru on Oracle Linux 9 or above."
        exit 1
    fi
    
    local version
    version=$(grep -oP '(?<=release )\d+' /etc/oracle-release 2>/dev/null || echo "0")
    
    if [[ "$version" -lt 9 ]]; then
        echo "Error: Oracle Linux version $version is not supported."
        echo "Please use Oracle Linux 9 or above."
        exit 1
    fi
    
    echo "Oracle Linux $version detected. Proceeding with setup..."
}

# Run OS check
check_oracle_linux_version

# Verify core system utilities are available
echo "Verifying core system utilities..."
core_utils=("sudo" "df" "lsblk" "blkid" "mount" "umount" "modprobe" "mkdir" "rm" "mv" "dd")
missing_utils=()

for util in "${core_utils[@]}"; do
    if ! command -v "$util" &> /dev/null; then
        missing_utils+=("$util")
    fi
done

if [ ${#missing_utils[@]} -ne 0 ]; then
    echo "Error: The following required utilities are missing: ${missing_utils[*]}"
    echo "Please install the following packages:"
    echo "  - util-linux (provides: df, lsblk, blkid, mount, umount)"
    echo "  - kmod (provides: modprobe)"
    echo "  - coreutils (provides: mkdir, rm, mv, dd)"
    exit 1
fi
echo "✓ Core system utilities verified"

# Install QEMU (including qemu-img and qemu-nbd)
if ! command -v qemu-img &> /dev/null || ! command -v qemu-nbd &> /dev/null; then
    echo "Installing QEMU tools..."
    if command -v dnf &> /dev/null; then
        # Try to install both qemu-img and qemu-kvm-tools
        if ! sudo dnf install -y qemu-img qemu-kvm-tools 2>/dev/null; then
            echo "qemu-kvm-tools not available, trying qemu-kvm package instead..."
            sudo dnf install -y qemu-kvm
        fi
    else
        # Try to install both qemu-img and qemu-kvm-tools
        if ! sudo yum install -y qemu-img qemu-kvm-tools 2>/dev/null; then
            echo "qemu-kvm-tools not available, trying qemu-kvm package instead..."
            sudo yum install -y qemu-kvm
        fi
    fi
else
    echo "QEMU tools already installed."
fi

# Verify QEMU tools are available
if ! command -v qemu-img &> /dev/null; then
    echo "Error: qemu-img not found after installation"
    exit 1
fi
if ! command -v qemu-nbd &> /dev/null; then
    echo "Error: qemu-nbd not found after installation"
    exit 1
fi
echo "✓ qemu-img is installed"
echo "✓ qemu-nbd is installed"

# Install OpenTofu
if ! command -v tofu &> /dev/null; then
    echo "Installing OpenTofu..."
    # Download the installer script
    curl --proto '=https' --tlsv1.2 -fsSL https://get.opentofu.org/install-opentofu.sh -o install-opentofu.sh

    # Give it execution permissions
    chmod +x install-opentofu.sh

    echo "Please inspect the downloaded install-opentofu.sh script before proceeding."
    # Run the installer
    ./install-opentofu.sh --install-method rpm

    # Remove the installer
    rm -f install-opentofu.sh
else
    echo "OpenTofu already installed."
fi

echo "Kopru environment setup complete."

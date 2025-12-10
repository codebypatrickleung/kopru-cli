#!/bin/bash
# --------------------------------------------------------------------------------------------
# Kopru Environment Setup Script
# --------------------------------------------------------------------------------------------
# Sets up Kopru environment by installing required tools.
# Supported OS: Oracle Linux 9+
# Usage: ./setup-environment.sh
# --------------------------------------------------------------------------------------------

set -e

check_oracle_linux_version() {
    if [[ ! -f /etc/oracle-release ]]; then
        echo "Error: This script only supports Oracle Linux 9 or above."
        exit 1
    fi
    local version
    version=$(grep -oP '(?<=release )\d+' /etc/oracle-release 2>/dev/null || echo "0")
    if [[ "$version" -lt 9 ]]; then
        echo "Error: Oracle Linux version $version is not supported."
        exit 1
    fi
    echo "Oracle Linux $version detected."
}

verify_core_utilities() {
    echo "Verifying core system utilities..."
    core_utils=(sudo df lsblk blkid mount umount modprobe mkdir rm mv dd oci-metadata)
    missing_utils=()
    for util in "${core_utils[@]}"; do
        if ! command -v "$util" &>/dev/null; then
            missing_utils+=("$util")
        else
            echo -n "✓ $util version: "
            "$util" --version 2>/dev/null | head -n 1 || echo "version info not available"
        fi
    done
    if [ ${#missing_utils[@]} -ne 0 ]; then
        echo "Error: Missing utilities: ${missing_utils[*]}"
        echo "Install packages: util-linux, kmod, coreutils"
        exit 1
    fi
    echo "Core system utilities verified."
}

install_qemu_tools() {
    if ! command -v qemu-img &>/dev/null || ! command -v qemu-nbd &>/dev/null; then
        echo "Installing QEMU tools..."
        if command -v dnf &>/dev/null; then
            sudo dnf install -y qemu-img qemu-kvm-tools 2>/dev/null || sudo dnf install -y qemu-kvm
        else
            sudo yum install -y qemu-img qemu-kvm-tools 2>/dev/null || sudo yum install -y qemu-kvm
        fi
    else
        echo "✓ QEMU tools already installed."
    fi
}

install_opentofu() {
    if ! command -v tofu &>/dev/null; then
        echo "Installing OpenTofu..."
        curl --proto '=https' --tlsv1.2 -fsSL https://get.opentofu.org/install-opentofu.sh -o install-opentofu.sh
        chmod +x install-opentofu.sh
        echo "Please inspect install-opentofu.sh before proceeding."
        ./install-opentofu.sh --install-method rpm
        rm -f install-opentofu.sh
    else
        echo "✓ OpenTofu already installed."
    fi
}

main() {
    check_oracle_linux_version
    verify_core_utilities
    install_qemu_tools
    install_opentofu
    echo "Kopru environment setup complete."
}

main

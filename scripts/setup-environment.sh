#!/bin/bash
# --------------------------------------------------------------------------------------------
# Kopru Environment Setup Script
# --------------------------------------------------------------------------------------------
# Sets up Kopru environment by installing required tools.
# Supported OS: Oracle Linux 9
# Usage: ./setup-environment.sh
# --------------------------------------------------------------------------------------------

set -euo pipefail

check_oracle_linux_version() {
    if [[ ! -f /etc/oracle-release ]]; then
        echo "Error: This script only supports Oracle Linux 9."
        exit 1
    fi
    local version
    version=$(grep -oP '(?<=release )\d+' /etc/oracle-release 2>/dev/null || echo "0")
    if [[ "$version" -ne 9 ]]; then
        echo "Error: Oracle Linux version $version is not supported."
        exit 1
    fi
    echo "Oracle Linux $version detected."
}

verify_core_utilities() {
    echo "Verifying core system utilities..."
    local core_utils=(sudo df lsblk blkid mount umount modprobe mkdir rm mv dd oci-metadata)
    local missing_utils=()
    for util in "${core_utils[@]}"; do
        if ! command -v "$util" &>/dev/null; then
            missing_utils+=("$util")
        else
            echo -n "✓ $util version: "
            "$util" --version 2>/dev/null | head -n 1 || echo "version info not available"
        fi
    done
    if (( ${#missing_utils[@]} )); then
        echo "Error: Missing utilities: ${missing_utils[*]}"
        echo "Install packages: util-linux, kmod, coreutils"
        exit 1
    fi
    echo "Core system utilities verified."
}

install_qemu_tools() {
    if ! command -v qemu-img &>/dev/null; then
        echo "Installing QEMU tools..."
        if command -v dnf &>/dev/null; then
            sudo dnf install -y qemu-img qemu-kvm-tools 2>/dev/null || sudo dnf install -y qemu-kvm
        else
            sudo yum install -y qemu-img qemu-kvm-tools 2>/dev/null || sudo yum install -y qemu-kvm
        fi
    else
        echo "✓ QEMU tools already installed."
    fi

    if systemctl list-unit-files | grep -q virtqemud.socket; then
        echo "Starting virtqemud.socket..."
        systemctl enable virtqemud.socket
        systemctl start virtqemud.socket
    else
        echo "virtqemud.socket not found, skipping start."
    fi
}

install_libguestfs() {
    if ! command -v guestfish &>/dev/null; then
        echo "Installing libguestfs tools..."
        if command -v dnf &>/dev/null; then
            sudo dnf install -y libguestfs-tools
        else
            sudo yum install -y libguestfs-tools
        fi
    else
        echo "✓ libguestfs tools already installed."
    fi
}

install_go(){
    if ! command -v go &>/dev/null; then
        echo "Installing latest Go..."
        if command -v dnf &>/dev/null; then
            sudo dnf install -y golang
        else
            sudo yum install -y golang
        fi
    else
        echo "✓ Go already installed."
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

install_oci_cli() {
    if ! command -v oci &>/dev/null; then
        echo "Installing OCI CLI..."
        if command -v dnf &>/dev/null; then
            dnf -y install oraclelinux-developer-release-el9
            dnf -y install python39-oci-cli
        else
            yum -y install oraclelinux-developer-release-el9
            yum -y install python39-oci-cli
        fi
    else
        echo "✓ OCI CLI already installed."
    fi
}   

main() {
    check_oracle_linux_version
    verify_core_utilities
    install_libguestfs
    install_qemu_tools
    install_opentofu
    install_go
    install_oci_cli
    echo "Kopru environment setup complete."
}

main

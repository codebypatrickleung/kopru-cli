#!/bin/bash
# Ubuntu Azure to OCI OS Configuration Script
# Applies Ubuntu-specific configurations for Azure to OCI migration

set -euo pipefail

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Get mount directory from argument or environment variable
MOUNT_DIR="${1:-${KOPRU_MOUNT_DIR:-}}"
if [[ -z "$MOUNT_DIR" ]]; then
    echo -e "${RED}ERROR: Mount directory not provided${NC}"
    echo "Usage: $0 <mount_dir>"
    exit 1
fi
if [[ ! -d "$MOUNT_DIR" ]]; then
    echo -e "${RED}ERROR: Mount directory does not exist: $MOUNT_DIR${NC}"
    exit 1
fi

log_info()    { echo -e "${BLUE}[INFO] $1${NC}"; }
log_success() { echo -e "${GREEN}[DONE] ✓ $1${NC}"; }
log_warning() { echo -e "${YELLOW}[WARN] $1${NC}"; }

disable_azure_udev_rules() {
    log_info "Disabling Azure-specific udev rules..."
    for rule in 66-azure-storage.rules 99-azure-product-uuid.rules; do
        local rule_path="$MOUNT_DIR/etc/udev/rules.d/$rule"
        [[ -f "$rule_path" ]] && mv "$rule_path" "${rule_path}.disable" 2>/dev/null && log_success "Disabled $rule" || true
    done
}

disable_azure_linux_agent() {
    log_info "Disabling WALinux Agent files..."
    for path in /var/lib/waagent /etc/init/walinuxagent.conf /etc/init.d/walinuxagent /usr/sbin/waagent /usr/sbin/waagent2.0 /etc/waagent.conf /var/log/waagent.log; do
        local full_path="$MOUNT_DIR$path"
        [[ -e "$full_path" ]] && mv "$full_path" "${full_path}.disable" 2>/dev/null && log_success "Disabled $path" || true
    done
}

disable_azure_hosts_template() {
    log_info "Disabling Azure hosts template..."
    local hosts_template="$MOUNT_DIR/etc/cloud/templates/hosts.azurelinux.tmpl"
    [[ ! -f "$hosts_template" ]] && log_info "Azure hosts template not found, skipping..." && return
    grep -q "^disable$" "$hosts_template" 2>/dev/null && log_info "✓ Azure hosts template already disabled" && return
    echo "disable" >> "$hosts_template"
    log_success "Disabled Azure hosts template"
}

disable_azure_chrony_refclock() {
    log_info "Disabling Azure PTP hyperv refclock in chrony config..."
    local chrony_conf="$MOUNT_DIR/etc/chrony/chrony.conf"
    [[ ! -f "$chrony_conf" ]] && log_info "Chrony config not found, skipping..." && return
    local target="refclock PHC /dev/ptp_hyperv poll 3 dpoll -2 offset 0"
    grep -q "$target" "$chrony_conf" 2>/dev/null || { log_info "Azure PTP hyperv refclock not found, skipping..."; return; }
    grep -q "^$target$" "$chrony_conf" 2>/dev/null || { log_info "✓ Azure PTP hyperv refclock already disabled"; return; }
    sed -i "s|^$target$|# $target|" "$chrony_conf"
    log_success "Disabled Azure PTP hyperv refclock"
}

add_oci_chrony_server() {
    log_info "Adding OCI metadata server to chrony config..."
    local chrony_conf="$MOUNT_DIR/etc/chrony/chrony.conf"
    [[ ! -f "$chrony_conf" ]] && log_info "Chrony config not found, skipping..." && return
    local oci_server="server 169.254.169.254 iburst"
    grep -q "^$oci_server$" "$chrony_conf" 2>/dev/null && { log_info "✓ OCI metadata server already present"; return; }
    echo "$oci_server" >> "$chrony_conf"
    log_success "Added OCI metadata server to chrony config"
}

add_oci_datasource() {
    log_info "Configuring cloud-init for OCI..."
    local cfg_dir="$MOUNT_DIR/etc/cloud/cloud.cfg.d"
    mkdir -p "$cfg_dir"
    echo "datasource_list: [Oracle, None]" > "$cfg_dir/99_oci.cfg"
    for file in 10-azure-kvp.cfg 90-azure.cfg 90_dpkg.cfg; do
        local file_path="$cfg_dir/$file"
        [[ -f "$file_path" ]] && mv "$file_path" "${file_path}.disable" 2>/dev/null && log_success "Disabled $file" || true
    done
    log_success "Configured cloud-init datasource"
}

cleanup_bind_mounts() {
    log_info "Cleaning up bind mounts..."
    local mounts=("$@")
    for ((i=${#mounts[@]}-1; i>=0; i--)); do
        local mount_path="$MOUNT_DIR/${mounts[$i]}"
        umount "$mount_path" 2>/dev/null || log_warning "Failed to unmount ${mounts[$i]}"
    done
}

set_oracle_kernel_as_default() {
    local kernel_version="$1"
    log_info "Setting Oracle kernel as default in GRUB..."
    local grub_path="$MOUNT_DIR/etc/default/grub"
    [[ ! -f "$grub_path" ]] && log_info "GRUB config not found, skipping..." && return
    local menu_entry="Advanced options for Ubuntu>Ubuntu, with Linux ${kernel_version}-oracle"
    grep -q "^GRUB_DEFAULT=" "$grub_path" && \
        sed -i "s|^GRUB_DEFAULT=.*|GRUB_DEFAULT=\"$menu_entry\"|" "$grub_path" || \
        sed -i "1i GRUB_DEFAULT=\"$menu_entry\"" "$grub_path"
    log_success "Set Oracle kernel ${kernel_version}-oracle as default in GRUB"
}

switch_to_oracle_kernel() {
    log_info "Switching from Azure-optimized kernel to Oracle-optimized kernel..."
    local bind_mounts=("proc" "sys" "dev") mounted=()
    for mount in "${bind_mounts[@]}"; do
        mount --bind "/$mount" "$MOUNT_DIR/$mount" 2>/dev/null && mounted+=("$mount") || { log_warning "Failed to bind $mount"; cleanup_bind_mounts "${mounted[@]}"; return 1; }
    done
    [ -e "$MOUNT_DIR/etc/resolv.conf" ] && mv "$MOUNT_DIR/etc/resolv.conf" "$MOUNT_DIR/etc/resolv.conf.bak"
    cp /etc/resolv.conf "$MOUNT_DIR/etc/resolv.conf"
    local chroot_script="$MOUNT_DIR/tmp/install-oracle-kernel.sh"
    mkdir -p "$MOUNT_DIR/tmp"
    cat > "$chroot_script" << 'CHROOT_EOF'
#!/bin/bash
set -e
sed -i 's|azure.archive.ubuntu.com|archive.ubuntu.com|g' /etc/apt/sources.list
apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install -y linux-oracle
KERNEL_VERSION=$(dpkg-query -W -f='${Package}\n' linux-image-*-oracle 2>/dev/null | head -1 | sed 's/linux-image-//')
[ -z "$KERNEL_VERSION" ] && echo "Error: Failed to detect installed Oracle kernel version." >&2 && exit 1
echo "$KERNEL_VERSION" > /tmp/oracle-kernel-version.txt
CHROOT_EOF
    chmod +x "$chroot_script"
    if chroot "$MOUNT_DIR" /bin/bash /tmp/install-oracle-kernel.sh; then
        local kernel_version_file="$MOUNT_DIR/tmp/oracle-kernel-version.txt"
        if [[ -f "$kernel_version_file" ]]; then
            local kernel_version
            kernel_version=$(cat "$kernel_version_file" | tr -d '\n ')
            [[ -n "$kernel_version" && "$kernel_version" == *.* ]] && set_oracle_kernel_as_default "$kernel_version" && log_success "Installed Oracle kernel version: $kernel_version" || log_warning "Invalid kernel version format: $kernel_version"
            rm -f "$kernel_version_file"
        else
            log_warning "Failed to read kernel version"
        fi
    else
        log_warning "Failed to install Oracle kernel in chroot"
    fi
    rm -f "$chroot_script"
    cleanup_bind_mounts "${bind_mounts[@]}"
    [ -e "$MOUNT_DIR/etc/resolv.conf.bak" ] && mv "$MOUNT_DIR/etc/resolv.conf.bak" "$MOUNT_DIR/etc/resolv.conf"
    log_success "Successfully switched to Oracle kernel"
}

update_grub() {
    log_info "Updating GRUB for OCI serial console..."
    local grub_path="$MOUNT_DIR/etc/default/grub"
    [[ ! -f "$grub_path" ]] && log_info "GRUB config not found, skipping..." && return
    grep -q "console=ttyS0" "$grub_path" && log_info "✓ GRUB console already configured" && return
    grep -q "console=" "$grub_path" && log_info "GRUB already has a console parameter, skipping..." && return
    grep -q '^GRUB_CMDLINE_LINUX="' "$grub_path" && \
        sed -i 's|^\(GRUB_CMDLINE_LINUX=".*\)\(".*$\)|\1 console=ttyS0,115200\2|' "$grub_path" && \
        log_success "Updated GRUB console configuration" || \
        log_warning "GRUB_CMDLINE_LINUX not found or has unexpected format, skipping"
}

main() {
    log_info "Ubuntu configurations started..."
    disable_azure_udev_rules
    disable_azure_linux_agent
    disable_azure_hosts_template
    disable_azure_chrony_refclock
    add_oci_chrony_server
    add_oci_datasource
    #switch_to_oracle_kernel
    update_grub
    log_success "Ubuntu configurations complete"
}

main
exit 0

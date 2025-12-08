#!/bin/bash
# Ubuntu Azure to OCI OS Configuration Script
# Applies Ubuntu-specific configurations for Azure to OCI migration

set -euo pipefail

log_info()    { echo "$1"; }
log_success() { echo "$1"; }
log_warning() { echo "$1"; }

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

disable_azure_udev_rules() {
    log_info "Disabling Azure-specific udev rules..."
    for rule in 66-azure-storage.rules 99-azure-product-uuid.rules; do
        local rule_path="$MOUNT_DIR/etc/udev/rules.d/$rule"
        [[ -f "$rule_path" ]] && mv "$rule_path" "${rule_path}.disable" 2>/dev/null && log_success "Disabled $rule"
    done
}

disable_azure_linux_agent() {
    log_info "Disabling WALinux Agent files..."
    for path in /var/lib/waagent /etc/init/walinuxagent.conf /etc/init.d/walinuxagent /usr/sbin/waagent /usr/sbin/waagent2.0 /etc/waagent.conf /var/log/waagent.log; do
        local full_path="$MOUNT_DIR$path"
        [[ -e "$full_path" ]] && mv "$full_path" "${full_path}.disable" 2>/dev/null && log_success "Disabled $path"
    done
}

disable_azure_cloudinit_datasource() {
    log_info "Disabling Azure cloud-init datasource configs..."
    local cfg_dir="$MOUNT_DIR/etc/cloud/cloud.cfg.d"
    for file in 10-azure-kvp.cfg 90-azure.cfg 90_dpkg.cfg; do
        local file_path="$cfg_dir/$file"
        [[ -f "$file_path" ]] && mv "$file_path" "${file_path}.disable" 2>/dev/null && log_success "Disabled $file"
    done
}

disable_azure_chrony_refclock() {
    log_info "Disabling Azure PTP hyperv refclock in chrony config..."
    local chrony_conf="$MOUNT_DIR/etc/chrony/chrony.conf"
    local target="refclock PHC /dev/ptp_hyperv poll 3 dpoll -2 offset 0"
    if [[ ! -f "$chrony_conf" ]]; then
        log_info "Chrony config not found, skipping..."
        return
    fi
    if ! grep -q "$target" "$chrony_conf" 2>/dev/null; then
        log_info "Azure PTP hyperv refclock not found, skipping..."
        return
    fi
    if ! grep -q "^$target$" "$chrony_conf" 2>/dev/null; then
        log_info "✓ Azure PTP hyperv refclock already disabled"
        return
    fi
    sed -i "s|^$target$|# $target|" "$chrony_conf"
    log_success "Disabled Azure PTP hyperv refclock"
}

uninstall_azure_linux_agent() {
    log_info "Uninstalling WALinux Agent from chroot..."
    local bind_mounts=("proc" "sys" "dev" "dev/pts")
    local mounted=()
    IFS=' ' read -r -a mounted <<< "$(create_bind_mounts "${bind_mounts[@]}")"
    [[ ${#mounted[@]} -eq 0 ]] && return 1

    if chroot "$MOUNT_DIR" /bin/bash -c "apt-get remove -y walinuxagent >/dev/null 2>&1"; then
        log_success "Successfully uninstalled WALinux Agent"
    else
        log_warning "Failed to uninstall WALinux Agent in chroot"
    fi

    cleanup_bind_mounts "${bind_mounts[@]}"
}   

add_oci_chrony_config() {
    log_info "Adding OCI metadata server to chrony config..."
    local chrony_conf="$MOUNT_DIR/etc/chrony/chrony.conf"
    local oci_server="server 169.254.169.254 iburst"
    if [[ ! -f "$chrony_conf" ]]; then
        log_info "Chrony config not found, skipping..."
        return
    fi
    if grep -q "^$oci_server$" "$chrony_conf" 2>/dev/null; then
        log_info "✓ OCI metadata server already present"
        return
    fi
    echo "$oci_server" >> "$chrony_conf"
    log_success "Added OCI metadata server to chrony config"
}

add_oci_cloudinit_datasource() {
    log_info "Configuring cloud-init for OCI..."
    local cfg_dir="$MOUNT_DIR/etc/cloud/cloud.cfg.d"
    mkdir -p "$cfg_dir"
    echo "datasource_list: [Oracle, None]" > "$cfg_dir/99_oci.cfg"
    log_success "Configured cloud-init datasource for OCI"
}

create_bind_mounts() {
    log_info "Creating bind mounts for chroot..."
    local mounts=("$@")
    local mounted=()
    for mount in "${mounts[@]}"; do
        if mount --bind "/$mount" "$MOUNT_DIR/$mount" 2>/dev/null; then
            mounted+=("$mount")
        else
            log_warning "Failed to bind $mount"
            cleanup_bind_mounts "${mounted[@]}"
            return 1
        fi
    done
    echo "${mounted[@]}"
}

cleanup_bind_mounts() {
    log_info "Cleaning up bind mounts..."
    local mounts=("$@")
    for ((i=${#mounts[@]}-1; i>=0; i--)); do
        local mount_path="$MOUNT_DIR/${mounts[$i]}"
        if mountpoint -q "$mount_path"; then
            if umount "$mount_path" 2>/dev/null; then
                log_success "Unmounted ${mounts[$i]}"
            else
                log_warning "Failed to unmount ${mounts[$i]}, retrying with lazy unmount..."
                umount -l "$mount_path" 2>/dev/null && log_success "Lazy unmounted ${mounts[$i]}"
            fi
        fi
    done
    log_success "Bind mount cleanup complete"
}

set_oracle_kernel_as_default() {
    local kernel_version="$1"
    log_info "Setting Oracle kernel as default in GRUB..."
    local grub_path="$MOUNT_DIR/etc/default/grub"
    local menu_entry="Advanced options for Ubuntu>Ubuntu, with Linux ${kernel_version}"
    if [[ ! -f "$grub_path" ]]; then
        log_info "GRUB config not found, skipping..."
        return
    fi
    if grep -q "^GRUB_DEFAULT=" "$grub_path"; then
        sed -i "s|^GRUB_DEFAULT=.*|GRUB_DEFAULT=\"$menu_entry\"|" "$grub_path"
    else
        sed -i "1i GRUB_DEFAULT=\"$menu_entry\"" "$grub_path"
    fi
    log_info "GRUB_DEFAULT set to $menu_entry"
    log_success "Set Oracle kernel ${kernel_version}-oracle as default in GRUB"
}

switch_to_oracle_optimized_kernel() {
    log_info "Switching from Azure-optimized kernel to Oracle-optimized kernel..."
    local bind_mounts=("proc" "sys" "dev" "dev/pts")
    local mounted=()
    IFS=' ' read -r -a mounted <<< "$(create_bind_mounts "${bind_mounts[@]}")"
    [[ ${#mounted[@]} -eq 0 ]] && return 1

    log_info "Setting up DNS resolution in chroot..."
    [[ -e "$MOUNT_DIR/etc/resolv.conf" || -L "$MOUNT_DIR/etc/resolv.conf" ]] && mv "$MOUNT_DIR/etc/resolv.conf" "$MOUNT_DIR/etc/resolv.conf.bak"
    cp /etc/resolv.conf "$MOUNT_DIR/etc/resolv.conf"
    log_success "DNS resolution setup complete"

    local chroot_script="$MOUNT_DIR/tmp/install-oracle-kernel.sh"
    mkdir -p "$MOUNT_DIR/tmp"
    cat > "$chroot_script" << 'CHROOT_EOF'
#!/bin/bash
set -e
sed -i 's|azure.archive.ubuntu.com|archive.ubuntu.com|g' /etc/apt/sources.list
apt-get update >/dev/null 2>&1
DEBIAN_FRONTEND=noninteractive apt-get install -y linux-oracle >/dev/null 2>&1
KERNEL_VERSION=$(dpkg-query -W -f='${Package}\n' linux-image-*-oracle 2>/dev/null | head -1 | sed 's/linux-image-//')
[ -z "$KERNEL_VERSION" ] && echo "Error: Failed to detect installed Oracle kernel version." >&2 && exit 1
echo "$KERNEL_VERSION" > /tmp/oracle-kernel-version.txt
CHROOT_EOF
    chmod +x "$chroot_script"

    log_info "Installing Oracle kernel in chroot..."
    if chroot "$MOUNT_DIR" /bin/bash /tmp/install-oracle-kernel.sh; then
        local kernel_version_file="$MOUNT_DIR/tmp/oracle-kernel-version.txt"
        if [[ -f "$kernel_version_file" ]]; then
            local kernel_version
            kernel_version=$(<"$kernel_version_file" tr -d '\n ')
            if [[ -n "$kernel_version" && "$kernel_version" == *.* ]]; then
                set_oracle_kernel_as_default "$kernel_version" 
                log_success "Installed Oracle kernel version: $kernel_version"
            else
                log_warning "Invalid kernel version format: $kernel_version"
            fi
            rm -f "$kernel_version_file"
        else
            log_warning "Failed to read kernel version"
        fi
    else
        log_warning "Failed to install Oracle kernel in chroot"
    fi
    log_success "Oracle kernel installation in chroot complete"

    rm -f "$chroot_script"
    log_success "Removed temporary Oracle kernel installation script"
    log_info "Cleaning up bind mounts..."
    cleanup_bind_mounts "${bind_mounts[@]}"
    log_success "Bind mount cleanup complete"
    log_info "Restoring original resolv.conf..."
    [[ -e "$MOUNT_DIR/etc/resolv.conf.bak" ]] && mv "$MOUNT_DIR/etc/resolv.conf.bak" "$MOUNT_DIR/etc/resolv.conf"
    log_success "Restored original resolv.conf"
    log_success "Successfully switched to Oracle kernel"
}

add_grub_config_for_oci_serial_console() {
    log_info "Updating GRUB for OCI serial console..."
    local grub_path="$MOUNT_DIR/etc/default/grub"
    if [[ ! -f "$grub_path" ]]; then
        log_info "GRUB config not found, skipping..."
        return
    fi
    if grep -q '^GRUB_SERIAL_COMMAND=' "$grub_path"; then
        sed -i 's|^GRUB_SERIAL_COMMAND=.*|GRUB_SERIAL_COMMAND="serial --unit=0 --speed=115200"|' "$grub_path"
        log_success "Updated GRUB_SERIAL_COMMAND to OCI recommended value"
    else
        echo 'GRUB_SERIAL_COMMAND="serial --unit=0 --speed=115200"' >> "$grub_path"
        log_success "Added GRUB_SERIAL_COMMAND for serial console support"
    fi
    if grep -q "console=ttyS0" "$grub_path"; then
        log_info "✓ GRUB console already configured"
        return
    fi
    if grep -q "console=" "$grub_path"; then
        log_info "GRUB already has a console parameter, skipping..."
        return
    fi
    if grep -q '^GRUB_CMDLINE_LINUX="' "$grub_path"; then
        sed -i 's|^\(GRUB_CMDLINE_LINUX=".*\)\(".*$\)|\1console=ttyS0,115200\2|' "$grub_path"
        log_success "Updated GRUB console configuration"
    fi
    log_success "GRUB configuration for OCI serial console complete"
}

run_grub_update_in_chroot() {
    log_info "Running grub-update in chroot..."
    local bind_mounts=("proc" "sys" "dev" "dev/pts")
    local mounted=()
    IFS=' ' read -r -a mounted <<< "$(create_bind_mounts "${bind_mounts[@]}")"
    [[ ${#mounted[@]} -eq 0 ]] && return 1

    if chroot "$MOUNT_DIR" /bin/bash -c "update-grub" > /dev/null 2>&1; then
        log_success "Successfully ran update-grub in chroot"
    else
        log_warning "Failed to run update-grub in chroot"
    fi

    log_info "Cleaning up bind mounts..."
    cleanup_bind_mounts "${bind_mounts[@]}"
    log_success "Bind mount cleanup complete"
    log_info "Restoring original resolv.conf..."
    [[ -e "$MOUNT_DIR/etc/resolv.conf.bak" ]] && mv "$MOUNT_DIR/etc/resolv.conf.bak" "$MOUNT_DIR/etc/resolv.conf"
    log_success "Restored original resolv.conf"
    log_success "GRUB update in chroot complete"
}

main() {
    log_info "Ubuntu configurations started..."
    
    # Azure-specific removals
    disable_azure_udev_rules
    disable_azure_cloudinit_datasource
    disable_azure_chrony_refclock
    uninstall_azure_linux_agent
    
    # OCI-specific additions
    add_oci_chrony_config
    add_oci_cloudinit_datasource

    # Kernel switch
    switch_to_oracle_optimized_kernel 

    # GRUB updates
    add_grub_config_for_oci_serial_console
    run_grub_update_in_chroot

    log_success "Ubuntu configurations complete"
}

main
exit 0

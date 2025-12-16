#!/bin/bash
# Common functions for OS configuration scripts

# Logging functions
log_info()    { echo -e "[INFO] $1"; }
log_success() { echo -e "\033[1;32m[SUCCESS]\033[0m $1"; }
log_warning() { echo -e "\033[1;33m[WARN]\033[0m $1"; }
log_error()   { echo -e "\033[1;31m[ERROR]\033[0m $1"; }

# Detect OS family (debian-based or rhel-based)
detect_os_family() {
    local os_release="$MOUNT_DIR/etc/os-release"
    if [[ -f "$os_release" ]]; then
        local os_id
        os_id=$(grep -E '^ID=' "$os_release" | cut -d= -f2 | tr -d '"')
        case "$os_id" in
            ubuntu|debian) echo "debian" ;;
            rhel|centos|almalinux|rocky|ol|fedora) echo "rhel" ;;
            *) echo "unknown" ;;
        esac
    else
        log_warning "Could not detect OS family - /etc/os-release not found"
        echo "unknown"
    fi
}

# Detect OS version
detect_os_version() {
    local os_release="$MOUNT_DIR/etc/os-release"
    if [[ -f "$os_release" ]]; then
        grep -E '^VERSION_ID=' "$os_release" | cut -d= -f2 | tr -d '"'
    else
        log_warning "Could not detect OS version - /etc/os-release not found"
        echo "unknown"
    fi
}

# Create bind mounts for chroot operations
create_bind_mounts() {
    log_info "Creating bind mounts for chroot..."
    local mounts=("$@") mounted=()
    for mount in "${mounts[@]}"; do
        if mountpoint -q "$MOUNT_DIR/$mount" 2>/dev/null; then
            log_info "✓ $mount already mounted, skipping..."
            mounted+=("$mount")
        elif mount --bind "/$mount" "$MOUNT_DIR/$mount" 2>/dev/null; then
            mounted+=("$mount")
            log_info "✓ Mounted $mount"
        else
            log_warning "Failed to bind $mount"
            cleanup_bind_mounts "${mounted[@]}"
            return 1
        fi
    done
    echo "${mounted[@]}"
}

# Cleanup bind mounts
cleanup_bind_mounts() {
    log_info "Cleaning up bind mounts..."
    local mounts=("$@")
    for ((i=${#mounts[@]}-1; i>=0; i--)); do
        local mount_path="$MOUNT_DIR/${mounts[$i]}"
        if mountpoint -q "$mount_path" 2>/dev/null; then
            umount "$mount_path" 2>/dev/null \
                && log_info "✓ Unmounted ${mounts[$i]}" \
                || { log_warning "Failed to unmount ${mounts[$i]}, retrying with lazy unmount..."; umount -l "$mount_path" 2>/dev/null && log_info "✓ Lazy unmounted ${mounts[$i]}"; }
        fi
    done
    log_success "Bind mount cleanup complete"
}

# Setup DNS resolution in chroot environment
setup_chroot_dns() {
    log_info "Setting up DNS resolution in chroot..."
    local resolv="$MOUNT_DIR/etc/resolv.conf"
    if [[ -e "$resolv" || -L "$resolv" ]]; then
        if [[ ! -e "$resolv.bak" ]]; then
            mv "$resolv" "$resolv.bak"
            log_info "✓ Backed up existing resolv.conf"
        else
            log_info "✓ resolv.conf.bak already exists, skipping backup"
        fi
    fi
    cp /etc/resolv.conf "$resolv"
    log_success "DNS resolution setup complete"
}

# Restore original DNS configuration
restore_chroot_dns() {
    log_info "Restoring original resolv.conf..."
    local resolv="$MOUNT_DIR/etc/resolv.conf"
    if [[ -e "$resolv.bak" ]]; then
        mv "$resolv.bak" "$resolv"
        log_success "Restored original resolv.conf"
    else
        log_info "No backup resolv.conf found, skipping restore"
    fi
}

# Disable Azure-specific udev rules
disable_azure_udev_rules() {
    log_info "Disabling Azure-specific udev rules..."
    local udev_dir="$MOUNT_DIR/etc/udev/rules.d"
    [[ ! -d "$udev_dir" ]] && { log_info "udev rules directory not found, skipping Azure udev rules disable"; return 0; }
    local rules=(66-azure-storage.rules 99-azure-product-uuid.rules 68-azure-sriov-nm-unmanaged.rules)
    for rule in "${rules[@]}"; do
        local rule_path="$udev_dir/$rule"
        if [[ -f "$rule_path" && ! -f "${rule_path}.disable" ]]; then
            mv "$rule_path" "${rule_path}.disable" 2>/dev/null && log_success "✓ Disabled $rule"
        elif [[ -f "${rule_path}.disable" ]]; then
            log_info "✓ $rule already disabled"
        fi
    done
    shopt -s nullglob
    for rule_file in "$udev_dir"/*-azure-*.rules; do
        if [[ -f "$rule_file" && ! -f "${rule_file}.disable" ]]; then
            mv "$rule_file" "${rule_file}.disable" 2>/dev/null && log_success "✓ Disabled $(basename "$rule_file")"
        elif [[ -f "${rule_file}.disable" ]]; then
            log_info "✓ $(basename "$rule_file") already disabled"
        fi
    done
    shopt -u nullglob
    return 0
}

# Disable Azure cloud-init datasource configuration
disable_azure_cloudinit_datasource() {
    log_info "Disabling Azure cloud-init datasource configs..."
    local cfg_dir="$MOUNT_DIR/etc/cloud/cloud.cfg.d"
    [[ ! -d "$cfg_dir" ]] && { log_info "cloud.cfg.d directory not found, skipping..."; return 0; }
    local os_family files_to_disable
    os_family=$(detect_os_family)
    if [[ "$os_family" == "debian" ]]; then
        files_to_disable=(10-azure-kvp.cfg 90-azure.cfg 90_dpkg.cfg)
    elif [[ "$os_family" == "rhel" ]]; then
        files_to_disable=(10-azure-kvp.cfg 90-azure.cfg 91-azure_datasource.cfg)
    else
        files_to_disable=(10-azure-kvp.cfg 90-azure.cfg 90_dpkg.cfg 91-azure_datasource.cfg)
    fi
    for file in "${files_to_disable[@]}"; do
        local file_path="$cfg_dir/$file"
        if [[ -f "$file_path" && ! -f "${file_path}.disable" ]]; then
            mv "$file_path" "${file_path}.disable" 2>/dev/null && log_success "✓ Disabled $file"
        elif [[ -f "${file_path}.disable" ]]; then
            log_info "✓ $file already disabled"
        fi
    done
    return 0
}

# Add OCI cloud-init datasource configuration
add_oci_cloudinit_datasource() {
    log_info "Configuring cloud-init for OCI..."
    local cfg_dir="$MOUNT_DIR/etc/cloud/cloud.cfg.d"
    [[ ! -d "$cfg_dir" ]] && { log_info "cloud.cfg.d directory not found, skipping OCI cloud-init datasource config..."; return 0; }
    local oci_cfg="$cfg_dir/99_oci.cfg"
    if [[ -f "$oci_cfg" ]] && grep -q "datasource_list: \[Oracle, None\]" "$oci_cfg" 2>/dev/null; then
        log_info "✓ OCI cloud-init datasource already configured"
        return 0
    fi
    echo "datasource_list: [Oracle, None]" > "$oci_cfg"
    log_success "Configured cloud-init datasource for OCI"
}

# Disable Azure PTP Hyper-V refclock in chrony config
disable_azure_chrony_refclock() {
    log_info "Disabling Azure PTP Hyper-V refclock in chrony config..."
    local os_family chrony_conf target
    os_family=$(detect_os_family)
    if [[ "$os_family" == "debian" ]]; then
        chrony_conf="$MOUNT_DIR/etc/chrony/chrony.conf"
    else
        chrony_conf="$MOUNT_DIR/etc/chrony.conf"
    fi
    target="refclock PHC /dev/ptp_hyperv poll 3 dpoll -2 offset 0"
    [[ ! -f "$chrony_conf" ]] && { log_info "Chrony config not found at $chrony_conf, skipping..."; return 0; }
    grep -q "$target" "$chrony_conf" 2>/dev/null || { log_info "Azure PTP hyperv refclock not found, skipping..."; return 0; }
    grep -q "^$target$" "$chrony_conf" 2>/dev/null || { log_info "✓ Azure PTP hyperv refclock already disabled"; return 0; }
    sed -i "s|^$target$|# $target|" "$chrony_conf"
    log_success "Disabled Azure PTP hyperv refclock"
}

# Add OCI metadata server to chrony config
add_oci_chrony_config() {
    log_info "Adding OCI metadata server to chrony config..."
    local os_family chrony_conf oci_server
    os_family=$(detect_os_family)
    if [[ "$os_family" == "debian" ]]; then
        chrony_conf="$MOUNT_DIR/etc/chrony/chrony.conf"
    else
        chrony_conf="$MOUNT_DIR/etc/chrony.conf"
    fi
    oci_server="server 169.254.169.254 iburst"
    [[ ! -f "$chrony_conf" ]] && { log_info "Chrony config not found at $chrony_conf, skipping..."; return 0; }
    grep -q "^$oci_server$" "$chrony_conf" 2>/dev/null && { log_info "✓ OCI metadata server already present"; return 0; }
    echo "$oci_server" >> "$chrony_conf"
    log_success "Added OCI metadata server to chrony config"
}

# Uninstall Azure Linux Agent (OS-specific)
uninstall_azure_linux_agent() {
    log_info "Uninstalling Azure Linux Agent..."
    local os_family bind_mounts mounted
    os_family=$(detect_os_family)
    bind_mounts=("proc" "sys" "dev" "dev/pts")
    IFS=' ' read -r -a mounted <<< "$(create_bind_mounts "${bind_mounts[@]}")"
    [[ ${#mounted[@]} -eq 0 ]] && { log_warning "Failed to create bind mounts for Azure agent uninstall"; return 0; }
    if [[ "$os_family" == "debian" ]]; then
        if chroot "$MOUNT_DIR" /bin/bash -c "dpkg -s walinuxagent >/dev/null 2>&1"; then
            log_info "Found WALinux Agent package, uninstalling..."
            chroot "$MOUNT_DIR" /bin/bash -c "apt-get remove -y walinuxagent >/dev/null 2>&1" \
                && log_success "Successfully uninstalled WALinux Agent" \
                || log_warning "Failed to uninstall WALinux Agent in chroot"
        else
            log_info "WALinux Agent not installed, skipping uninstall"
        fi
    elif [[ "$os_family" == "rhel" ]]; then
        local systemd_dir="$MOUNT_DIR/etc/systemd/system"
        if [[ -d "$systemd_dir/multi-user.target.wants" && -L "$systemd_dir/multi-user.target.wants/waagent.service" ]]; then
            rm -f "$systemd_dir/multi-user.target.wants/waagent.service" \
                && log_success "Disabled waagent.service" \
                || log_warning "Failed to disable waagent.service"
        else
            log_info "waagent.service not enabled, skipping..."
        fi
    fi
    cleanup_bind_mounts "${bind_mounts[@]}"
}

# Disable Hyper-V KVP daemon for all OS families
disable_hyperv_kvp_daemon() {
    log_info "Disabling Hyper-V KVP daemon service..."
    local systemd_dir service_link
    
    systemd_dir="$MOUNT_DIR/etc/systemd/system"
    service_link="$systemd_dir/multi-user.target.wants/hv-kvp-daemon.service"
    
    # Check if service is enabled (symlink exists)
    if [[ -L "$service_link" ]]; then
        log_info "Found enabled hv-kvp-daemon.service, disabling..."
        rm -f "$service_link" \
            && log_success "✓ Disabled hv-kvp-daemon.service" \
            || log_warning "Failed to disable hv-kvp-daemon.service"
    else
        log_info "hv-kvp-daemon.service not enabled, skipping..."
    fi
    
    return 0
}

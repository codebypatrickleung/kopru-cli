#!/bin/bash
# Common functions for OS configuration scripts

log_info()    { echo -e "[INFO] $1"; }
log_success() { echo -e "\033[1;32m[SUCCESS]\033[0m $1"; }
log_warning() { echo -e "\033[1;33m[WARN]\033[0m $1"; }
log_error()   { echo -e "\033[1;31m[ERROR]\033[0m $1"; }

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

detect_os_version() {
    local os_release="$MOUNT_DIR/etc/os-release"
    if [[ -f "$os_release" ]]; then
        grep -E '^VERSION_ID=' "$os_release" | cut -d= -f2 | tr -d '"'
    else
        log_warning "Could not detect OS version - /etc/os-release not found"
        echo "unknown"
    fi
}

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

add_oci_cloudinit_datasource() {
    log_info "Configuring cloud-init for OCI..."
    local cfg_dir="$MOUNT_DIR/etc/cloud/cloud.cfg.d"
    [[ ! -d "$cfg_dir" ]] && { log_info "cloud.cfg.d directory not found, skipping OCI cloud-init datasource config..."; return 0; }
    local oci_cfg="$cfg_dir/90_dpkg.cfg"
    if [[ -f "$oci_cfg" ]] && grep -q "datasource_list: \[ Oracle \]" "$oci_cfg" 2>/dev/null; then
        log_info "✓ OCI cloud-init datasource already configured"
        return 0
    fi
    echo "datasource_list: [ Oracle ]" > "$oci_cfg"
    log_success "Configured cloud-init datasource for OCI"

    return 0
}

add_ssh_host_keys_fix() {
    log_info "Adding SSH host keys fix for OCI..."
    local os_release="$MOUNT_DIR/etc/os-release"
    local os_id
    if [[ -f "$os_release" ]]; then
        os_id=$(grep -E '^ID=' "$os_release" | cut -d= -f2 | tr -d '"')
    else
        log_info "os-release not found, skipping SSH host keys fix..."
        return 0
    fi

    # Only apply fix for Ubuntu and Debian
    if [[ "$os_id" != "ubuntu" && "$os_id" != "debian" ]]; then
        log_info "Not Ubuntu or Debian, skipping SSH host keys fix..."
        return 0
    fi

    local cfg_dir="$MOUNT_DIR/etc/cloud/cloud.cfg.d"
    [[ ! -d "$cfg_dir" ]] && { log_info "cloud.cfg.d directory not found, skipping SSH host keys fix..."; return 0; }
    local oci_cfg="$cfg_dir/99_ssh_host_keys_fix.cfg"
    if [[ -f "$oci_cfg" ]]; then
        log_info "✓ SSH host keys fix already present"
        return 0
    fi
    cat > "$oci_cfg" <<EOF
ssh_deletekeys: false
ssh_genkeytypes:
  - rsa
  - ecdsa
  - ed25519
EOF
    log_success "Added SSH host keys fix for OCI"
    return 0
}

disable_azure_chrony_refclock() {
    log_info "Disabling Azure PTP Hyper-V refclock in chrony config..."
    local os_family chrony_conf target
    os_family=$(detect_os_family)
    if [[ "$os_family" == "debian" ]]; then
        chrony_conf="$MOUNT_DIR/etc/chrony/chrony.conf"
    else
        chrony_conf="$MOUNT_DIR/etc/chrony.conf"
    fi
    local target="refclock PHC /dev/ptp_hyperv poll 3 dpoll -2 offset 0"
    [[ ! -f "$chrony_conf" ]] && { log_info "Chrony config not found at $chrony_conf, skipping..."; return 0; }

    if grep -Eq "^$target.*" "$chrony_conf" 2>/dev/null; then
        sed -i "s|^$target\(.*\)|# $target\1|" "$chrony_conf"
        log_success "Disabled Azure PTP hyperv refclock ($target ...)"
    else
        log_info "Azure PTP hyperv refclock not found, skipping..."
    fi

    return 0
}

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

    return 0
}

disable_azure_linux_agent() {
    log_info "Disabling Azure Linux Agent..."
    local os_family
    os_family=$(detect_os_family)
    
    if [[ "$os_family" == "debian" ]]; then
        # For Debian-based systems, disable walinuxagent by renaming service files
        local walinuxagent_service_etc="$MOUNT_DIR/etc/systemd/system/multi-user.target.wants/walinuxagent.service"
        if [[ -L "$walinuxagent_service_etc" ]]; then
            log_info "Found walinuxagent.service symlink, disabling..."
            mv "$walinuxagent_service_etc" "${walinuxagent_service_etc}.disable" 2>/dev/null \
                && log_success "✓ Disabled walinuxagent.service" \
                || log_warning "Failed to disable walinuxagent.service"
        else
            log_info "walinuxagent.service not found or not enabled, skipping..."
        fi
        
        local walinuxagent_service_lib="$MOUNT_DIR/lib/systemd/system/walinuxagent.service"
        if [[ -f "$walinuxagent_service_lib" && ! -f "${walinuxagent_service_lib}.disable" ]]; then
            log_info "Found walinuxagent.service file, disabling..."
            mv "$walinuxagent_service_lib" "${walinuxagent_service_lib}.disable" 2>/dev/null \
                && log_success "✓ Disabled walinuxagent.service file" \
                || log_warning "Failed to disable walinuxagent.service file"
        else
            log_info "walinuxagent.service file not found or already disabled, skipping..."
        fi

        local network_setup_service_lib="$MOUNT_DIR/lib/systemd/system/walinuxagent-network-setup.service"
        if [[ -f "$network_setup_service_lib" && ! -f "${network_setup_service_lib}.disable" ]]; then
            log_info "Found walinuxagent-network-setup.service file, disabling..."
            mv "$network_setup_service_lib" "${network_setup_service_lib}.disable" 2>/dev/null \
                && log_success "✓ Disabled walinuxagent-network-setup.service" \
                || log_warning "Failed to disable walinuxagent-network-setup.service"
        else
            log_info "walinuxagent-network-setup.service not found or not enabled, skipping..."
        fi
        
    elif [[ "$os_family" == "rhel" ]]; then
        # For RHEL-based systems, disable waagent by renaming service symlink
        local systemd_dir="$MOUNT_DIR/lib/systemd/system"
        local waagent_service="$systemd_dir/multi-user.target.wants/waagent.service"
        
        if [[ -L "$waagent_service" ]]; then
            log_info "Found waagent.service symlink, disabling..."
            mv "$waagent_service" "${waagent_service}.disable" 2>/dev/null \
                && log_success "✓ Disabled waagent.service" \
                || log_warning "Failed to disable waagent.service"
        else
            log_info "waagent.service not found or not enabled, skipping..."
        fi
    fi
    
    log_success "Azure Linux Agent disabled"

    return 0
}

disable_azure_hyperv_daemon() {
    log_info "Disabling Hyper-V daemons..."

    log_info "Disabling Hyper-V KVP daemon service..."
    local kvp_service_link="$MOUNT_DIR/lib/systemd/system/hv-kvp-daemon.service"
    if [[ -f "$kvp_service_link" && ! -f "${kvp_service_link}.disable" ]]; then
        log_info "Found enabled hv-kvp-daemon.service, disabling..."
        mv -f "$kvp_service_link" "${kvp_service_link}.disable" \
            && log_success "✓ Disabled hv-kvp-daemon.service" \
            || log_warning "Failed to disable hv-kvp-daemon.service"
    else
        log_info "hv-kvp-daemon.service not enabled, skipping..."
    fi

    log_info "Disabling Hyper-V VSS daemon service..."
    local vss_service_link="$MOUNT_DIR/lib/systemd/system/hv-vss-daemon.service"
    if [[ -f "$vss_service_link" && ! -f "${vss_service_link}.disable" ]]; then
        log_info "Found enabled hv-vss-daemon.service, disabling..."
        mv -f "$vss_service_link" "${vss_service_link}.disable" \
            && log_success "✓ Disabled hv-vss-daemon.service" \
            || log_warning "Failed to disable hv-vss-daemon.service"
    else
        log_info "hv-vss-daemon.service not enabled, skipping..."
    fi

    log_info "Disabling Hyper-V fcopy daemon service..."
    local fcopy_service_link="$MOUNT_DIR/lib/systemd/system/hv-fcopy-daemon.service"
    if [[ -f "$fcopy_service_link" && ! -f "${fcopy_service_link}.disable" ]]; then
        log_info "Found enabled hv-fcopy-daemon.service, disabling..."
        mv -f "$fcopy_service_link" "${fcopy_service_link}.disable" \
            && log_success "✓ Disabled hv-fcopy-daemon.service" \
            || log_warning "Failed to disable hv-fcopy-daemon.service"
    else
        log_info "hv-fcopy-daemon.service not enabled, skipping..."
    fi
    
    return 0
}

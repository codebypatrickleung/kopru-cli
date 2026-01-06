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
    log_info "Adding SSH host keys fix..."
    local os_release="$MOUNT_DIR/etc/os-release"
    local os_id
    if [[ -f "$os_release" ]]; then
        os_id=$(grep -E '^ID=' "$os_release" | cut -d= -f2 | tr -d '"')
    else
        log_info "os-release not found, skipping SSH host keys fix..."
        return 0
    fi

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
    log_success "Added SSH host keys fix"
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
    sed -i "\$a$oci_server" "$chrony_conf"
    log_success "Added OCI metadata server to chrony config"
    return 0
}

disable_azure_linux_agent() {
    log_info "Disabling Azure Linux Agent..."
    local os_family
    os_family=$(detect_os_family)
    if [[ "$os_family" == "debian" ]]; then
        local services=(walinuxagent.service walinuxagent-network-setup.service)
        local targets=(multi-user.target.wants network.target.wants)
    elif [[ "$os_family" == "rhel" ]]; then
        local services=(waagent.service)
        local targets=(multi-user.target.wants)
    else
        return 0
    fi
    for i in "${!services[@]}"; do
        local svc="${services[$i]}"
        local tgt="${targets[$i]}"
        local link="$MOUNT_DIR/etc/systemd/system/$tgt/$svc"
        if [[ -L "$link" ]]; then
            log_info "Found $svc symlink, repointing to /dev/null..."
            ln -sf /dev/null "$link" \
                && log_success "✓ Repointed $svc symlink to /dev/null" \
                || log_warning "Failed to repoint $svc symlink"
        else
            log_info "$svc symlink not found, skipping..."
        fi
    done
    return 0
}

auto_relabel_selinux_contexts() {
    log_info "Resetting SELinux labels on /etc/ssh/sshd_config..."
    local selinux_config="$MOUNT_DIR/etc/selinux/config"
    if [[ ! -f "$selinux_config" ]]; then
        log_info "SELinux config not found, skipping SELinux label reset..."
        return 0
    fi
    if ! grep -Eq '^\s*SELINUX=(enforcing|permissive)' "$selinux_config"; then
        log_info "SELinux is not enabled, skipping SELinux label reset..."
        return 0
    fi
    touch "$MOUNT_DIR/.autorelabel" \
        && log_success "✓ Requested SELinux relabel by creating .autorelabel" \
        || log_warning "Failed to create .autorelabel for SELinux relabel"
    return 0
}

disable_azure_hyperv_daemon() {
    local services=(hv-kvp-daemon hv-vss-daemon hv-fcopy-daemon)
    for svc in "${services[@]}"; do
        log_info "Disabling Hyper-V ${svc} service..."
        local svc_link="$MOUNT_DIR/etc/systemd/system/multi-user.target.wants/${svc}.service"
        if [[ -L "$svc_link" ]]; then
            log_info "Found ${svc}.service symlink, repointing to /dev/null..."
            ln -sf /dev/null "$svc_link" \
                && log_success "✓ Repointed ${svc}.service symlink to /dev/null" \
                || log_warning "Failed to repoint ${svc}.service symlink"
        else
            log_info "${svc}.service symlink not found, skipping..."
        fi
    done
    return 0
}

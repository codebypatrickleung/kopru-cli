#!/bin/bash
# Common functions for OS configuration scripts

log_info()    { echo -e "[INFO] $1"; }
log_success() { echo -e "\033[1;32m[SUCCESS]\033[0m $1"; }
log_warning() { echo -e "\033[1;33m[WARN]\033[0m $1"; }
log_error()   { echo -e "\033[1;31m[ERROR]\033[0m $1"; }

detect_os_info_from_image() {
    local output os_id os_version os_family
    output=$(virt-cat -a "$IMAGE_FILE" /etc/os-release 2>/dev/null || echo "")
    os_id=$(echo "$output" | grep -E "^ID=" | head -n1 | cut -d= -f2 | tr -d '"')
    os_version=$(echo "$output" | grep -E "^VERSION_ID=" | head -n1 | cut -d= -f2 | tr -d '"')
    case "$os_id" in
        ubuntu|debian) os_family="debian" ;;
        rhel|centos|almalinux|rocky|ol|fedora|opensuse|opensuse-leap|opensuse-tumbleweed|sles) os_family="rhel" ;;
        *) os_family="unknown" ;;
    esac
    echo "$os_family|${os_version:-unknown}|${os_id:-unknown}"
}

detect_guest_architecture() {
    local image_file=$1
    local arch
    arch=$(virt-inspector -a "$image_file" | virt-inspector --xpath "string(//arch)")
    case "$arch" in
        x86_64|x86-64|amd64) echo "x86_64" ;;
        aarch64|arm64) echo "aarch64" ;;
        *) echo "x86_64" ;;
    esac
}

disable_azure_cloud_init() {
    local image_file=$1 os_family=$2
    log_info "Disabling Azure cloud-init datasource..."
    local files_to_disable
    files_to_disable=(10-azure-kvp.cfg 90-azure.cfg 90_dpkg.cfg 91-azure_datasource.cfg)
    for file in "${files_to_disable[@]}"; do
        virt-customize -a "$image_file" --edit "/etc/cloud/cloud.cfg.d/$file:s/^/# /" &>/dev/null || true
    done
}

disable_azure_chrony() {
    local image_file=$1 os_family=$2 os_id=$3
    log_info "Disabling Azure chrony refclock..."
    if [[ "$os_id" == "sles" ]]; then
        virt-customize -a "$image_file" --edit "/etc/chrony.d/azure.conf:s/^/# /" &>/dev/null || log_warning "Failed to comment out Azure chrony config for SLES"
        log_success "Successfully commented out Azure chrony config for SLES"
        return 0
    fi
    local chrony_conf
    [[ "$os_family" == "debian" ]] && chrony_conf="/etc/chrony/chrony.conf" || chrony_conf="/etc/chrony.conf"
    virt-customize -a "$image_file" --edit "$chrony_conf:s|^refclock PHC /dev/ptp_hyperv|# refclock PHC /dev/ptp_hyperv|" &>/dev/null || log_warning "Failed to disable Azure chrony refclock"
}

disable_azure_hyperv_daemons() {
    local image_file=$1 os_family=$2 
    log_info "Disabling Azure Hyper-V daemons..."
    local hv_services=(hv-kvp-daemon hv-vss-daemon hv-fcopy-daemon)
    for svc in "${hv_services[@]}"; do
        local cmd="systemctl disable --now ${svc} || true"
        if ! virt-customize -a "$image_file" --run-command "$cmd" &>/dev/null; then
            log_warning "Failed to run command directly for ${svc}, scheduling at first boot"
            virt-customize -a "$image_file" --firstboot-command "$cmd" &>/dev/null || log_warning "Failed to disable ${svc}"
        fi
    done
}

disable_azure_agent() {
    local image_file=$1 os_family=$2 
    log_info "Disabling Azure Linux Agent..."
    if [[ "$os_family" == "debian" ]]; then
        local cmd="systemctl disable --now walinuxagent || true"
        if ! virt-customize -a "$image_file" --run-command "$cmd" &>/dev/null; then
            log_warning "Failed to run command directly for walinuxagent, scheduling at first boot"
            virt-customize -a "$image_file" --firstboot-command "$cmd" &>/dev/null || log_warning "Failed to disable walinuxagent"
        fi
    elif [[ "$os_family" == "rhel" ]]; then
        local cmd="systemctl disable --now waagent || true"
        if ! virt-customize -a "$image_file" --run-command "$cmd" &>/dev/null; then
            log_warning "Failed to run command directly for waagent, scheduling at first boot"
            virt-customize -a "$image_file" --firstboot-command "$cmd" &>/dev/null || log_warning "Failed to disable waagent"
        fi
    fi
}

disable_azure_temp_disk_warning() {
    local image_file=$1 os_family=$2 
    log_info "Disabling Azure temp-disk-dataloss-warning service..."
    [[ "$os_family" != "rhel" ]] && return
    local cmd="systemctl disable --now temp-disk-dataloss-warning.service || true"
    if ! virt-customize -a "$image_file" --run-command "$cmd" &>/dev/null; then
        log_warning "Failed to run command directly for temp-disk-dataloss-warning, scheduling at first boot"
        virt-customize -a "$image_file" --firstboot-command "$cmd" &>/dev/null || log_warning "Failed to remove temp-disk-dataloss-warning service"
    fi
}

cloud_init_clean() {
    local image_file=$1 os_family=$2 
    log_info "Adding cloud-init clean..."
    if ! virt-customize -a "$image_file" --run-command "cloud-init clean --logs || true" &>/dev/null; then
        log_warning "Failed to run cloud-init clean, scheduling at first boot"
        virt-customize -a "$image_file" --firstboot-command "cloud-init clean --logs || true" &>/dev/null || log_warning "Failed to schedule cloud-init clean at first boot"
    fi
}

add_oci_chrony_config() {
    local image_file=$1 os_family=$2 os_id=$3
    log_info "Adding OCI chrony config..."
    if [[ "$os_id" == "sles" ]]; then
        log_info "Detected SLES - skipping OCI chrony config"
        return 0
    fi
    local chrony_conf oci_server
    [[ "$os_family" == "debian" ]] && chrony_conf="/etc/chrony/chrony.conf" || chrony_conf="/etc/chrony.conf"
    oci_server="server 169.254.169.254 iburst"
    if virt-cat -a "$image_file" "$chrony_conf" 2>/dev/null | grep -q "^$oci_server$"; then
        log_info "OCI chrony server already configured"
        return 0
    fi
    virt-customize -a "$image_file" --append-line "$chrony_conf:$oci_server" &>/dev/null || log_warning "Failed to add OCI chrony config"
}

add_oci_cloud_init() {
    local image_file=$1 os_family=$2 os_id=$3
    log_info "Adding OCI cloud-init datasource..."
    if [[ "$os_id" == "sles" ]]; then
        log_info "Detected SLES - editing /etc/cloud/cloud.cfg"
        if virt-customize -a "$image_file" --edit "/etc/cloud/cloud.cfg:s/datasource_list: *\[ *Azure *\]/datasource_list: [ Oracle ]/" &>/dev/null; then
            log_success "Updated datasource in /etc/cloud/cloud.cfg for SLES"
            return 0
        else
            log_warning "Failed to edit cloud.cfg datasource for SLES, trying fallback method"
        fi
    fi
    if ! virt-ls -a "$image_file" /etc/cloud/cloud.cfg.d &>/dev/null; then
        virt-customize -a "$image_file" --mkdir /etc/cloud/cloud.cfg.d &>/dev/null || log_warning "Failed to create cloud-init directory"
    fi
    if virt-customize -a "$image_file" --write "/etc/cloud/cloud.cfg.d/90_oci_datasource.cfg:datasource_list: [ Oracle ]" &>/dev/null; then
        log_success "OCI cloud-init datasource configured successfully"
    else
        log_warning "Failed to write OCI cloud-init datasource file"
    fi
}

fix_ssh_host_keys() {
    local image_file=$1 os_family=$2 
    [[ "$os_family" != "debian" ]] && return
    log_info "Configuring SSH host keys fix for cloud-init..."
    if ! virt-ls -a "$image_file" /etc/cloud/cloud.cfg.d &>/dev/null; then
        virt-customize -a "$image_file" --mkdir /etc/cloud/cloud.cfg.d &>/dev/null || log_warning "Failed to create cloud-init directory"
    fi
    local ssh_config="ssh_deletekeys: false
ssh_genkeytypes:
  - rsa
  - ecdsa
  - ed25519"
    virt-customize -a "$image_file" --write "/etc/cloud/cloud.cfg.d/99_ssh_host_keys_fix.cfg:$ssh_config" &>/dev/null || log_warning "Failed to write SSH host keys fix configuration"
}

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
                mkdir -p /etc/dracut.conf.d
                echo 'add_drivers+=\" iscsi_tcp iscsi_ibft \"' > /etc/dracut.conf.d/iscsi.conf
                echo 'add_dracutmodules+=\" iscsi \"' >> /etc/dracut.conf.d/iscsi.conf
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
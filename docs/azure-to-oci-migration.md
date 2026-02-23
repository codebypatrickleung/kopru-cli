# Azure to OCI Migration Workflow

This guide describes the process for migrating Microsoft Azure virtual machines (VMs) to Oracle Cloud Infrastructure (OCI) using the Kopru CLI.

## Supported Configurations

Kopru has been validated with the following Azure platform images:

- **Source Platform:** Microsoft Azure  
- **Operating System:**  
  - Ubuntu 22.04 LTS (x86_64), 24.04 LTS (x86_64, ARM)
  - Debian 13 Trixie (x86_64)
  - Red Hat Enterprise Linux 7.9, 8.1, 9.4 (x86_64)
  - Windows Server 2019, 2022, 2025 Datacenter
  - SUSE Linux Enterprise 15 SP5, 15 SP6 (x86_64)
- **Execution Environment:** Oracle Linux 9 on OCI
- **Target Platform:** Oracle Cloud Infrastructure

### Data Disks

Kopru automatically migrates and reattaches data disks in OCI. For best results, use UUIDs or LVM to mount data disks, not device paths (such as `/dev/sdb1`). If device paths are used, update `/etc/fstab` after migration to ensure device mappings are correct.

## Migration Steps

1. **Verify Virtio Drivers in Source OS**

   **Ubuntu/Debian:**  
   Virtio drivers are included by default. Verify drivers with the following commands:

   ```bash
   sudo grep -i virtio /boot/config-$(uname -r)
   sudo lsinitrd /boot/initramfs-$(uname -r).img | grep virtio
   ```

   **Red Hat/CentOS:**  
   Virtio drivers may not be included in initramfs. Rebuild if needed:

   ```bash
   KERNEL_VERSION=$(uname -r)
   INITRAMFS_PATH="/boot/initramfs-${KERNEL_VERSION}.img"
   sudo dracut -v -f --add-drivers "virtio virtio_pci virtio_scsi" "$INITRAMFS_PATH" "$KERNEL_VERSION"
   ```

   **Windows:**  
   Install Virtio drivers as described in the [Oracle documentation](https://docs.oracle.com/operating-systems/oracle-linux/kvm-virtio/kvm-virtio-InstallingtheOracleVirtIODriversforMicrosoftWindows.html).

Install Virtio drivers as described [here](https://docs.oracle.com/operating-systems/oracle-linux/kvm-virtio/kvm-virtio-InstallingtheOracleVirtIODriversforMicrosoftWindows.html).

### 2. Launch an Oracle Linux 9 Instance in OCI

See [OCI documentation](https://docs.oracle.com/iaas/Content/Compute/Tasks/launchinginstance.htm). Apply security best practices and consider using [Cloud Guard](https://www.oracle.com/security/cloud-security/cloud-guard/). 

### 3. Clone the Repository

```bash
dnf install -y git
git clone https://github.com/codebypatrickleung/kopru-cli.git
cd kopru-cli
```

4. **Set Up the Environment**  
   Install dependencies:
   ```bash
   chmod +x ./scripts/setup-environment.sh
   bash ./scripts/setup-environment.sh
   ```

5. **Build Kopru**
   ```bash
   go build -buildvcs=false -o kopru ./cmd/kopru
   ```

6. **Configure Authentication**

   - **Azure:**  
     Uses a Service Principal. Assign `Disk Snapshot Contributor` and `Reader` roles to the VM’s resource group. See [Azure Authentication documentation](https://learn.microsoft.com/azure/developer/go/sdk/authentication/authentication-on-premises-apps).

     Set credentials:

     ```bash
     export AZURE_TENANT_ID="your-tenant-id"
     export AZURE_CLIENT_ID="your-client-id"
     export AZURE_CLIENT_SECRET="YOUR_PASSWORD"   # PLEASE CHANGE YOUR_PASSWORD TO A REAL PASSWORD!
     export AZURE_SUBSCRIPTION_ID="your-subscription-id"
     ```

   - **OCI:**  
     Uses API key-based authentication. Ensure you have the correct IAM policies for the target compartment. See [OCI authentication documentation](https://docs.oracle.com/iaas/Content/API/SDKDocs/cliinstall.htm#configfile).

     Set up the config:

     ```bash
     oci setup config
     ```

7. **Run the Migration**

   Provide parameters using environment variables, command-line flags, or a config file.

   Example (environment variables):

   ```bash
   export AZURE_COMPUTE_NAME="azure-vm"
   export AZURE_RESOURCE_GROUP="azure-vm-rg"
   export OCI_COMPARTMENT_ID="ocid1.compartment.oc1..."
   export OCI_SUBNET_ID="ocid1.subnet.oc1..."
   export OCI_IMAGE_OS="Ubuntu"
   export OCI_IMAGE_OS_VERSION="24.04"
   export OCI_REGION="us-ashburn-1"
   export OCI_IMAGE_ENABLE_UEFI=true  # Set true for Windows Gen2 or ARM VMs
   ./kopru &
   ```

   For configuration parameters, run `./kopru --help` or refer to the sample configuration file.

8. **Manual OpenTofu Deployment (Optional)**

   If you used `--skip-template-deploy`, deploy manually:

   ```bash
   cd ./template-output
   tofu init
   tofu plan
   tofu apply
   ```

   Terraform is also supported. Replace `tofu` with `terraform` where appropriate.

## Logging

Kopru generates a log file named `kopru-<timestamp>.log` in the current directory. Logs are also written to the console.

## Performance Considerations

For simple VMs, migration may take 15–60 minutes. For larger VMs, consider the following optimisations:

- **Performance:**  
  Disk throughput is a common bottleneck. Use high-performance disks and allocate sufficient OCPUs to increase available network bandwidth for block storage.

- **Data Import:**  
  For faster, parallel disk operations, use the [concurrent-data-disk-import branch](https://github.com/codebypatrickleung/kopru-cli/tree/add-concurrent-data-disk-import) of the Kopru CLI.

Contact the project maintainer for additional downtime optimisation techniques.

## Post-Migration

After migration, perform health checks and testing to validate success. See the following for post-import tasks:
- [Post-Import tasks for Windows](https://docs.oracle.com/iaas/Content/Compute/Tasks/importingcustomimagewindows.htm#postimport)
- [Post-Import tasks for Linux](https://docs.oracle.com/iaas/Content/Compute/Tasks/importingcustomimagelinux.htm#postimport)

---

If you have specific instructions or requirements for further adaptation, let me know!
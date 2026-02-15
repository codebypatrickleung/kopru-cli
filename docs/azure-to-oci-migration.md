# Azure to OCI Migration Workflow

This guide details the steps for migrating Azure VMs to Oracle Cloud Infrastructure (OCI) using Kopru CLI.

## Supported Configurations

Kopru has been tested with the following Azure platform images:

- **Source Platform:** Microsoft Azure
- **Operating System:** (If your OS is not listed, update the [OS configuration script](./os-configurations.md))
  - Ubuntu 22.04 LTS (x86_64)
  - Ubuntu 24.04 LTS (x86_64, ARM)
  - Debian 13 Trixie (x86_64)
  - Red Hat Enterprise Linux 9.4 (x86_64)
  - Windows Server 2019, 2022, 2025 Datacenter
  - SuSE Enterprise Linux 15 SP6 (x86_64)
- **Execution Environment:** Oracle Linux 9 in OCI
- **Target Platform:** Oracle Cloud Infrastructure

### Data Disks

Kopru automatically migrates and reattaches data disks in OCI. For best results, ensure data disks are mounted using UUIDs or LVM, not device paths (e.g., `/dev/sdb1`). If device paths are used, update `/etc/fstab` after migration to reflect new device mappings.

## Migration Steps

### 1. Check Virtio Drivers in the Source OS

#### Ubuntu/Debian

Virtio drivers are included by default, but verify with:

```bash
sudo grep -i virtio /boot/config-$(uname -r)
sudo lsinitrd /boot/initramfs-$(uname -r).img | grep virtio
```

#### Red Hat/CentOS

Virtio drivers are included but not always in initramfs. Rebuild initramfs if needed:

```bash
KERNEL_VERSION=$(uname -r)
INITRAMFS_PATH="/boot/initramfs-${KERNEL_VERSION}.img"
sudo dracut -v -f --add-drivers "virtio virtio_pci virtio_scsi" "$INITRAMFS_PATH" "$KERNEL_VERSION"
```

#### Windows

Install Virtio drivers as described [here](https://docs.oracle.com/en/operating-systems/oracle-linux/kvm-virtio/kvm-virtio-InstallingtheOracleVirtIODriversforMicrosoftWindows.html).

### 2. Launch an Oracle Linux 9 Instance in OCI

See [OCI documentation](https://docs.oracle.com/en-us/iaas/Content/Compute/Tasks/launchinginstance.htm). Apply security best practices and consider using [Cloud Guard](https://www.oracle.com/uk/security/cloud-security/cloud-guard/).

### 3. Clone the Repository

```bash
dnf install -y git
git clone https://github.com/codebypatrickleung/kopru-cli.git
cd kopru-cli
```

### 4. Set Up the Environment

Install dependencies:

```bash
chmod +x ./scripts/setup-environment.sh
bash ./scripts/setup-environment.sh
```

### 5. Build the Binary

```bash
go build -buildvcs=false -o kopru ./cmd/kopru
```

### 6. Authentication Setup

Kopru requires authentication for both Azure and OCI.

#### Azure

- Uses a Service Principal.
- Requires `Disk Snapshot Contributor` and `Reader` roles on the VM's resource group.
- See [Azure Authentication docs](https://learn.microsoft.com/en-us/azure/developer/go/sdk/authentication/authentication-on-premises-apps).

Set credentials:

```bash
export AZURE_TENANT_ID="your-tenant-id"
export AZURE_CLIENT_ID="your-client-id"
export AZURE_CLIENT_SECRET="your-client-secret"
export AZURE_SUBSCRIPTION_ID="your-subscription-id"
```

#### OCI

- Uses API key-based authentication.
- Ensure proper IAM policies for the target compartment.
- See [OCI Authentication docs](https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/cliinstall.htm#configfile).

Set up config:

```bash
oci setup config
```

Follow the prompts to generate your OCI configuration file.
### 7. Running the Migration

Provide parameters via environment variables, command-line flags, or a config file.

Example using environment variables:

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

For all parameters, see `./kopru --help` or [Configuration Parameters](../kopru-config.env.template).

### 8. Manual OpenTofu Deployment (Optional)

If you used `--skip-template-deploy`, deploy manually:

```bash
cd ./template-output
tofu init
tofu plan
tofu apply
```

Terraform is also supported—replace `tofu` with `terraform`.

## Logging

Kopru creates a log file named `kopru-<timestamp>.log` in the current directory. Logs are also shown in the console.

## Post-Migration

After migration, perform health checks and testing to validate success.

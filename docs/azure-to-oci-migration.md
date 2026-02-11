# Azure to OCI Migration Workflow

This guide provides detailed steps for migrating Azure VMs to Oracle Cloud Infrastructure (OCI) using Kopru CLI.

## Supported Configurations

Kopru has been tested with the Azure platform images listed below:

- **Source Platform**: Microsoft Azure 
- **Operating System**: (If the OS is not listed, you might need to update the [OS configuration script](./os-configurations.md))
  - Ubuntu 22.04 LTS (x86_64) 
  - Ubuntu 24.04 LTS (x86_64 and ARM)
  - Debian 13 Trixie (x86_64)
  - Red Hat Enterprise Linux 9.4 (x86_64)
  - Windows Server 2019, 2022, 2025 Datacenter
  - SuSE Enterprise Linux 15 SP6 (x86_64)  
- **Execution Environment**: Oracle Linux 9 in OCI
- **Target Platform**: Oracle Cloud Infrastructure

### Data Disks

If your source VM has data disks attached, Kopru will automatically migrate and reattach them in OCI. For seamless operation, ensure your data disks are mounted using UUIDs or LVM rather than device paths (e.g., `/dev/sdb1`). If device paths are used, you may need to update `/etc/fstab` after migration to reflect the new device mappings in OCI.

## Prerequisites

### Step 1: Check Virtio Drivers in the Source OS

Before migrating, ensure that the source OS has virtio drivers installed.

#### Ubuntu/Debian

Virtio drivers are included by default, but it's always good to verify.

To check if virtio drivers are compiled into the kernel, run:
```bash
sudo grep -i virtio /boot/config-$(uname -r)
```

To check if virtio drivers are included in initramfs, run:
```bash
sudo lsinitrd /boot/initramfs-$(uname -r).img | grep virtio
```

#### Red Hat/CentOS

Virtio drivers are included by default, but not included in initramfs by default. 

You may need to rebuild initramfs to include them. Run:

```bash
KERNEL_VERSION=$(uname -r)
INITRAMFS_PATH="/boot/initramfs-${KERNEL_VERSION}.img"
sudo dracut -v -f --add-drivers "virtio virtio_pci virtio_scsi" "$INITRAMFS_PATH" "$KERNEL_VERSION"
```

#### Windows

Install the Virtio drivers by following the instructions [here](https://docs.oracle.com/en/operating-systems/oracle-linux/kvm-virtio/kvm-virtio-InstallingtheOracleVirtIODriversforMicrosoftWindows.html).

### Step 2: Launch an Oracle Linux 9 Instance in OCI

See [OCI documentation](https://docs.oracle.com/en-us/iaas/Content/Compute/Tasks/launchinginstance.htm). Ensure this virtual machine has security best practices applied, as it will handle all the VM's data during migration. Consider using [Cloud Guard](https://www.oracle.com/uk/security/cloud-security/cloud-guard/) to monitor the instance for any security issues.

### Step 3: Clone the Repository

Clone the Kopru CLI repository and navigate into it.

```bash
dnf install -y git
git clone https://github.com/codebypatrickleung/kopru-cli.git
cd kopru-cli
```

### Step 4: Set Up the Environment

The setup script installs dependencies like Go, qemu-img, and OpenTofu.

```bash
chmod +x ./scripts/setup-environment.sh
bash ./scripts/setup-environment.sh
```

### Step 5: Build the Binary

Build the Kopru CLI binary.

```bash
go build -buildvcs=false -o kopru ./cmd/kopru 
```

### Step 6: Authentication Setup

Kopru does not manage authentication itself. You must configure authentication for both Azure and OCI using official SDK or CLI tools.

## Azure Authentication 

Kopru uses a `Service Principal` for Azure authentication. For Azure, Kopru requires the `Disk Snapshot Contributor` and `Reader` roles on the VM's resource group.

See [Azure Authentication documentation](https://learn.microsoft.com/en-us/azure/developer/go/sdk/authentication/authentication-on-premises-apps) for more details.

### Setting up Azure Credentials

Set the following environment variables:

```bash
export AZURE_TENANT_ID="your-tenant-id"
export AZURE_CLIENT_ID="your-client-id"
export AZURE_CLIENT_SECRET="your-client-secret"
export AZURE_SUBSCRIPTION_ID="your-subscription-id"
```

### Required Azure Permissions

- `Disk Snapshot Contributor` - Allows creating and managing disk snapshots
- `Reader` - Allows reading resource group and VM information

## OCI Authentication

Kopru uses API key-based authentication for OCI. For OCI, ensure your user or group has the necessary IAM policies for the target compartment.

See [OCI Authentication documentation](https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/cliinstall.htm#configfile) for more details.

### Setting up OCI Credentials

Essentially, you just need to run `oci setup config`, and this config file will be used by Kopru as well as OpenTofu (or Terraform) automatically.

```bash
oci setup config
```

This command will guide you through setting up your OCI configuration file.

## Running the Migration

There are three ways to provide Kopru with the required parameters: environment variables, command-line flags, or a configuration file. There are only a few required parameters, which essentially identify the source Azure resource group/VM and target OCI compartment/subnet.

### Using Environment Variables

```bash
export AZURE_COMPUTE_NAME="azure-vm"
export AZURE_RESOURCE_GROUP="azure-vm-rg"
export OCI_COMPARTMENT_ID="ocid1.compartment.oc1..."
export OCI_SUBNET_ID="ocid1.subnet.oc1..."
export OCI_IMAGE_OS="Ubuntu"
export OCI_IMAGE_OS_VERSION="24.04"
export OCI_REGION="us-ashburn-1"
# Set UEFI to true for Windows Gen2 or ARM VMs; otherwise, leave as false (default).
export OCI_IMAGE_ENABLE_UEFI=true  
./kopru &
```

For a full list of parameters, see `./kopru --help` or refer to the [Configuration Parameters](../kopru-config.env.template) document.

## Manual OpenTofu Deployment (Optional)

This is an optional step, as the tool can auto-deploy the generated template. If you used `--skip-template-deploy`, navigate to the `template-output` directory and run OpenTofu commands to deploy the generated template:

```bash
cd ./template-output
tofu init
tofu plan
tofu apply
```

If you prefer Terraform, the generated templates are compatible. Just replace `tofu` with `terraform` in the commands above. OpenTofu is a fork of Terraform that has a fully open-source core and is part of the Linux Foundation. The generated templates maintain compatibility.

## Logging

Kopru creates a log file in the current directory named `kopru-<timestamp>.log`. Logs are shown in the console and saved for review.

## Post-Migration

As with all migrations, ensure health checks and testing are performed post-migration to validate success.

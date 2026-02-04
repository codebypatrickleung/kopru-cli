# Linux Image to OCI Deployment Workflow

This guide provides detailed steps for deploying Linux cloud images directly to Oracle Cloud Infrastructure (OCI) using Kopru CLI.

## Supported Configurations

Kopru supports direct deployment of Linux cloud images to OCI. You can use any Linux distribution that provides QCOW2 format cloud images.

- **Source**: Linux Cloud Images (downloaded from distribution repositories)
- **Supported Formats**: QCOW2
- **Supported Distributions**: 
  - [Debian](https://cloud.debian.org/images/cloud/trixie/latest/debian-13-genericcloud-amd64.qcow2)
  - [Fedora](https://download.fedoraproject.org/pub/fedora/linux/releases/43/Cloud/x86_64/images/Fedora-Cloud-Base-Generic-43-1.6.x86_64.qcow2)
  - [CentOS Stream](https://cloud.centos.org/centos/10-stream/x86_64/images/CentOS-Stream-GenericCloud-10-latest.x86_64.qcow2)
- **Execution Environment**: Oracle Linux 9 in OCI
- **Target Platform**: Oracle Cloud Infrastructure

## Image Preparation

Kopru automatically configures images for OCI including:

- iSCSI initiator installation
- Kernel parameter configuration (rd.iscsi.ibft=1 rd.iscsi.firmware=1)
- Initramfs rebuild
- Cloud-init version 20.3+ verification
- Oracle datasource configuration

## Prerequisites

### Step 1: Launch an Oracle Linux 9 Instance in OCI

See [OCI documentation](https://docs.oracle.com/en-us/iaas/Content/Compute/Tasks/launchinginstance.htm). Ensure this Virtual Machine has security best practices applied.

### Step 2: Clone the Repository

Clone the Kopru CLI repository and navigate into it.

```bash
dnf install -y git
git clone https://github.com/codebypatrickleung/kopru-cli.git
cd kopru-cli
```

### Step 3: Set Up the Environment

The setup script installs dependencies like Go, qemu-img, and OpenTofu.

```bash
chmod +x ./scripts/setup-environment.sh
bash ./scripts/setup-environment.sh
```

### Step 4: Build the Binary

Build the Kopru CLI binary.

```bash
go build -buildvcs=false -o kopru ./cmd/kopru 
```

### Step 5: Authentication Setup

Kopru uses `API Key-Based Authentication` for OCI authentication. See [OCI Authentication doc](https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/cliinstall.htm#configfile). 

For detailed authentication instructions, see [Authentication Setup Guide](authentication-setup.md).

## Running the Deployment

For Linux cloud image deployment, you only need to specify the target OCI configuration and the OS image URL. The image will be automatically downloaded, configured, and deployed.

### Using Environment Variables

```bash
export SOURCE_PLATFORM="linux_image"
export TARGET_PLATFORM="oci"
export OS_IMAGE_URL="https://cloud.debian.org/images/cloud/trixie/latest/debian-13-genericcloud-amd64.qcow2"  
export OCI_COMPARTMENT_ID="ocid1.compartment.oc1..."
export OCI_SUBNET_ID="ocid1.subnet.oc1..."
export OCI_REGION="us-ashburn-1"
export OCI_IMAGE_OS="Debian"  
export OCI_IMAGE_OS_VERSION="13"  
export OCI_BUCKET_NAME="linux-images"  
export OCI_IMAGE_NAME="debian-13-oci"   
export OCI_INSTANCE_NAME="debian-13-instance"  
export SSH_KEY_FILE="/path/to/your/public_key.pub"  
./kopru
```

For a full list of parameters, see `./kopru --help` or refer to the [Configuration Parameters](../kopru-config.env.template) document.

## Workflow Steps

The Linux Image to OCI workflow will:

1. Download the Linux cloud image from the specified URL (QCOW2 format)
2. Configure the image for OCI:
   - Install iSCSI initiator (open-iscsi package)
   - Add kernel parameters (rd.iscsi.ibft=1 rd.iscsi.firmware=1)
   - Rebuild initramfs with iSCSI modules
   - Verify cloud-init is version 20.3 or later
   - Configure Oracle as the authoritative cloud-init datasource
3. Upload the configured image to OCI Object Storage
4. Create a custom image in OCI
5. Deploy a compute instance from the custom image

## Manual OpenTofu Deployment (Optional)

This is an optional step as the tool can auto-deploy the generated template. If you used `--skip-template-deploy`, navigate to the `template-output` directory and run OpenTofu commands to deploy the generated template:

```bash
cd ./template-output
tofu init
tofu plan
tofu apply
```

If you prefer Terraform, the generated templates are compatible. Just replace `tofu` with `terraform` in the commands above. OpenTofu is a fork of Terraform that has a fully open-source core, and is part of the Linux Foundation. The generated templates maintain compatibility.

## Logging

Kopru creates a log file in the current directory named `kopru-<timestamp>.log`. Logs are shown in the console and saved for review.

## Post-Deployment

Ensure health checks and testing are performed post-deployment to validate success. The default user to login is debian for Debian, fedora for Fedora, and cloud-user for CentOS Stream.

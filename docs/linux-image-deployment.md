# Linux Image to OCI Deployment Workflow

This guide provides detailed steps for deploying Linux cloud images directly to Oracle Cloud Infrastructure (OCI) using the Kopru CLI.

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

## Prerequisites

### Step 1: Launch an Oracle Linux 9 Instance in OCI

See the [OCI documentation](https://docs.oracle.com/en-us/iaas/Content/Compute/Tasks/launchinginstance.htm). Ensure this virtual machine has security best practices applied.

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

Kopru uses API key-based authentication for OCI. For OCI, ensure your user or group has the necessary IAM policies for the target compartment.

See [OCI Authentication documentation](https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/cliinstall.htm#configfile) for more details.

### Setting up OCI Credentials

Essentially, you just need to run `oci setup config`, and this config file will be used by Kopru as well as OpenTofu (or Terraform) automatically.

```bash
oci setup config
```

This command will guide you through setting up your OCI configuration file.
## Running the Deployment

There are three ways to provide Kopru with the required parameters: environment variables, command-line flags, or a configuration file. There are only a few required parameters, which essentially identify the source Azure resource group/VM and target OCI compartment/subnet.

```bash
export SOURCE_PLATFORM="linux_image"
export TARGET_PLATFORM="oci"
export OS_IMAGE_URL="https://cloud.debian.org/images/cloud/trixie/latest/debian-13-genericcloud-amd64.qcow2"  
export OCI_COMPARTMENT_ID="ocid1.compartment.oc1..."
export OCI_SUBNET_ID="ocid1.subnet.oc1..."
export OCI_REGION="us-ashburn-1"
export OCI_IMAGE_OS="Debian"  
export OCI_IMAGE_OS_VERSION="13"  
export OCI_IMAGE_NAME="debian-13-oci"   
export OCI_INSTANCE_NAME="debian-13-instance"  
export SSH_KEY_FILE="/path/to/your/public_key.pub"  
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

## Post-Deployment

Ensure health checks and testing are performed post-deployment to validate success. The default user to log in is `debian` for Debian, `fedora` for Fedora, and `cloud-user` for CentOS Stream.

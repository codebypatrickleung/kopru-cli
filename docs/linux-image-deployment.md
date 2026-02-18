# Linux Image to OCI Deployment Workflow

This guide details the steps to deploy Linux cloud images to Oracle Cloud Infrastructure (OCI) using Kopru CLI.

## Supported Configurations

Kopru supports direct deployment of Linux cloud images to OCI. The following configurations have been tested:

- **Source**: Linux cloud images from official distribution repositories
- **Image Format**: QCOW2
- **Operating Systems**:  
  - [Debian](https://cloud.debian.org/images/cloud/trixie/latest/debian-13-genericcloud-amd64.qcow2)  
  - [Fedora](https://download.fedoraproject.org/pub/fedora/linux/releases/43/Cloud/x86_64/images/Fedora-Cloud-Base-Generic-43-1.6.x86_64.qcow2)  
  - [CentOS Stream](https://cloud.centos.org/centos/10-stream/x86_64/images/CentOS-Stream-GenericCloud-10-latest.x86_64.qcow2)  
  If your OS is not listed, update the [OS configuration script](./os-configurations.md).
- **Execution Environment**: Oracle Linux 9 on OCI
- **Target Platform**: Oracle Cloud Infrastructure

## Migration Steps

### 1. Check the URL of the Linux Image

Ensure the Linux image you want to deploy is in QCOW2 format and accessible via a public URL. You can use official cloud images from distribution repositories or custom-built images hosted on a web server.
### 2. Launch an Oracle Linux 9 Instance in OCI

See [OCI documentation](https://docs.oracle.com/en-us/iaas/Content/Compute/Tasks/launchinginstance.htm). Apply security best practices and consider using [Cloud Guard](https://www.oracle.com/uk/security/cloud-security/cloud-guard/).

### 3. Clone the Kopru CLI Repository

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

Kopru requires authentication for OCI.

#### OCI

- Uses API key-based authentication.
- Ensure proper IAM policies for the target compartment.
- See [OCI Authentication docs](https://docs.oracle.co

Set up config:

```bash
oci setup config
```

Follow the prompts to generate your OCI configuration file.

#### 7. Running the Deployment

Provide parameters via environment variables, command-line flags, or a config file.

Example using environment variables:

```bash
export SOURCE_PLATFORM="linux_image"
export TARGET_PLATFORM="oci"
export OS_IMAGE_URL="https://cloud.debian.org/images/cloud/trixie/latest/debian-13-genericcloud-amd64.qcow2"
export OCI_COMPARTMENT_ID="ocid1.compartment.oc1..."
export OCI_SUBNET_ID="ocid1.subnet.oc1..."
export OCI_REGION="us-ashburn-1"
export OCI_IMAGE_OS="Debian"
export OCI_IMAGE_OS_VERSION="13"
export OCI_IMAGE_NAME="debian-13-image"
export OCI_INSTANCE_NAME="debian-13-instance"
export SSH_KEY_FILE="/path/to/your/public_key.pub"
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

## Post-Deployment

After deployment, perform health checks and validation. Default login users:  
- Debian: `debian`  
- Fedora: `fedora`  
- CentOS Stream: `cloud-user`

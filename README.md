# Project Kopru

Kopru is a Go-based CLI tool that automates migration of Compute instances into Oracle Cloud Infrastructure (OCI). Migrating workloads between clouds is often manual and costly. Kopru streamlines this with a customizable, extensible workflow for simpler, affordable Compute imports into OCI.

## Features

- **Simple CLI**: Start a migration with just 4 parameters.
- **Go Implementation**: Built with Go, using Cobra and Viper for CLI and config management.
- **Native SDK Integration**: Uses official Azure and OCI Go SDKs for authentication and performance.
- **Workflow Resume**: `--skip-*` flags let you bypass completed steps and resume migrations.
- **OpenTofu Support**: Generates OpenTofu (Terraform-compatible) templates for OCI deployments.
- **Extensible & Open Source**: Easily adaptable for new platforms and OSes.

## Supported Configurations

- **Source Platform**: Microsoft Azure (more platforms coming soon)
- **Operating System**: Ubuntu (more OSes coming soon)
- **Execution Environment**: Oracle Linux 9+ in OCI
- **Target Platform**: Oracle Cloud Infrastructure

## Quick Start

1. Launch an Oracle Linux 9+ instance in OCI: 

See [OCI documentation](https://docs.oracle.com/en-us/iaas/Content/Compute/Tasks/launchinginstance.htm).

2. Clone this repository:
  
Clone the Kopru CLI repository and navigate into it.  

  ```bash
  git clone https://github.com/codebypatrickleung/kopru-cli.git
  cd kopru-cli
  ```

3. Set up the environment:
  
The setup script installs dependencies like Go, qemu-img, and OpenTofu.

  ```bash
  chmod +x ./scripts/setup-environment.sh
  bash ./scripts/setup-environment.sh
  ```

4. Build the binary:

Build the Kopru CLI binary. 

  ```bash
  go build -buildvcs=false -o kopru ./cmd/kopru 
  ```
5. **Authentication Setup**

Kopru does not handle authentication directly. Set up authentication for both Azure and OCI using the official SDK methods:

  - **Azure**: Kopru uses `DefaultAzureCredential`. See [Azure docs](https://learn.microsoft.com/en-us/azure/developer/go/sdk/authentication/authentication-on-premises-apps). Set:
    ```bash
    export AZURE_TENANT_ID="your-tenant-id"
    export AZURE_CLIENT_ID="your-client-id"
    export AZURE_CLIENT_SECRET="your-client-secret"
    export AZURE_SUBSCRIPTION_ID="your-subscription-id"
    ```
  - **OCI**: The SDK uses API Key-Based Authentication. See [OCI Go SDK docs](https://docs.oracle.com/en-us/iaas/Content/API/Concepts/sdk_authentication_methods.htm). 

6. Run Kopru using one of these methods:

Step 1-5 are the hard part! Now, run Kopru to start the migration. There are three ways to provide Kopru with the required parameters: environment variables, command-line flags, or a configuration file. There are only four required parameters, which essentially identify the source Azure VM and target OCI compartment/subnet.

  - **Environment variables**:
    ```bash
    export AZURE_COMPUTE_NAME="my-vm"
    export AZURE_RESOURCE_GROUP="my-rg"
    export OCI_COMPARTMENT_ID="ocid1.compartment.oc1..."
    export OCI_SUBNET_ID="ocid1.subnet.oc1..."
    ./kopru
    ```
  - **Command-line flags**:
    ```bash
    ./kopru --azure-compute-name my-vm \
         --azure-resource-group my-rg \
         --oci-compartment-id "ocid1.compartment.oc1..." \
         --oci-subnet-id "ocid1.subnet.oc1..."
    ```
  - **Configuration file**:
    ```bash
    ./kopru --config /path/to/kopru-config.env
    ```

  **Note:** Kopru creates a log file in the current directory named `kopru-<timestamp>.log`. Logs are shown in the console and saved for review.

7. **Manual OpenTofu Deployment** (if auto-deployment was skipped):

This is an optional step as the tool can auto-deploy the generated template. If you used `--skip-template-deploy`, navigate to the `template-output` directory and run OpenTofu commands to deploy the generated template:

  ```bash
  cd ./template-output
  tofu init
  tofu plan
  tofu apply
  ```

If you prefer Terraform, the generated templates are compatible. Just replace `tofu` with `terraform` in the commands above. OpenTofu is a fork of Terraform that has a fully open-source core, and is part of the Linux Foundation. The generated templates maintain compatibility.

## Contributing

Contributions are welcome! Open issues or pull requests for bug fixes, enhancements, or new features.

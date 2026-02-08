# Kopru

Kopru is a command-line tool written in Go that automates the import of Compute instances to Oracle Cloud Infrastructure (OCI). While manual migration through the OCI console is possible, it quickly becomes cumbersome and error-prone when handling multiple instances or intricate setups. Kopru simplifies this process by providing an extensible, repeatable workflow that accelerates and standardizes Compute deployments into OCI.

## Features

- **Simple CLI**: Start an import with just a few parameters.
- **Go Implementation**: Built with Go, using Cobra and Viper for CLI and config management.
- **Native SDK Integration**: Uses official Azure and OCI Go SDKs for authentication and performance.
- **Workflow Resume**: `--skip-*` flags let you bypass completed steps and resume migrations.
- **OpenTofu Support**: Generates OpenTofu (Terraform-compatible) templates for OCI deployments.
- **Extensible & Open Source**: Easily adaptable for new platforms and OSes.
- **Multiple Source Options**: Supports Azure VM migration and direct Linux cloud image deployment.

## Supported Workflows

Kopru currently supports two main import workflows:

### Migrate Azure VMs to OCI

Migrate Azure VMs (Ubuntu, Debian, RHEL, Windows Server) to Oracle Cloud Infrastructure with automatic data disk migration and virtio driver support.


[📖 View Detailed Azure to OCI Migration Guide](docs/azure-to-oci-migration.md)

### Import Linux Cloud Images to OCI

For OSes that are not available in OCI-provided images or the Marketplace, you can use this workflow to deploy Linux cloud images directly to OCI.


[📖 View Detailed Linux Cloud Image Deployment Guide](docs/linux-image-deployment.md)

## Quick Start

1. **Launch an Oracle Linux 9 instance in OCI** - [OCI Documentation](https://docs.oracle.com/en-us/iaas/Content/Compute/Tasks/launchinginstance.htm). Use a separate block device for the kopru-cli directory.

2. **Clone the repository**:
   ```bash
   dnf install -y git
   git clone https://github.com/codebypatrickleung/kopru-cli.git
   cd kopru-cli
   ```

3. **Set up the environment**:
   ```bash
   chmod +x ./scripts/setup-environment.sh
   bash ./scripts/setup-environment.sh
   ```

4. **Build the binary**:
   ```bash
   go build -buildvcs=false -o kopru ./cmd/kopru 
   ```

5. **Configure authentication** - [Authentication Setup Guide](docs/authentication-setup.md)

6. **Run Kopru** - See the workflow-specific guides for detailed instructions:
   - [Azure to OCI Migration Guide](docs/azure-to-oci-migration.md)
   - [Linux Image Deployment Guide](docs/linux-image-deployment.md)

## Documentation

- [Azure to OCI Migration](docs/azure-to-oci-migration.md) - Detailed guide for migrating Azure VMs to OCI
- [Linux Image Deployment](docs/linux-image-deployment.md) - Detailed guide for deploying Linux cloud images to OCI
- [OS Configurations](docs/os-configurations.md) - Information about OS-specific configuration scripts

## Additional Resources

- **Configuration**: For a full list of parameters, see `./kopru --help` or the [Configuration Parameters](kopru-config.env.template) file.
- **Logging**: Kopru creates a log file named `kopru-<timestamp>.log` in the current directory.

## Conclusion

For more details, please feel free to connect with me on [LinkedIn](https://www.linkedin.com/in/pgwl/) or GitHub. Happy migrating!

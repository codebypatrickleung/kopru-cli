# Kopru

Kopru is a command-line tool written in Go that automates the import of compute instances to Oracle Cloud Infrastructure (OCI). While manual import steps through the OCI Console are possible, they can quickly become cumbersome and error-prone, especially when managing complex environments. Kopru streamlines this process by providing an extensible, repeatable workflow that accelerates and standardizes compute deployments into OCI.

## Features

- **Simple CLI**: Initiate an import with just a few parameters.
- **Go Implementation**: Developed in Go, using Cobra and Viper for command-line interface and configuration management.
- **Native SDK Integration**: Integrates with official Azure and OCI Go SDKs for authentication and improved performance.
- **OpenTofu Support**: Generates OpenTofu (Terraform-compatible) templates for OCI deployments.
- **Extensible and Open Source**: Easily adaptable for new platforms and operating systems.
- **Multiple Source Options**: Supports both Azure VM migration and direct Linux cloud image deployment.

## Supported Workflows

Kopru currently supports two main import workflows, with additional workflows planned for the future.

### Migrate Azure VMs to OCI

Migrating VMs from Azure to OCI may involve multiple steps. Kopru automates this process by exporting the VM, converting it into a compatible format, and generating an OpenTofu template for deployment to OCI. If the source VM has data disks attached, Kopru automatically migrates and reattaches them in OCI.

[📖 View Detailed Azure to OCI Migration Guide](docs/azure-to-oci-migration.md)

### Import Linux Cloud Images to OCI

For operating systems that are not available in OCI-provided images or the Oracle Marketplace, you can use this workflow to deploy Linux cloud images directly to OCI.

[📖 View Detailed Linux Cloud Image Deployment Guide](docs/linux-image-deployment.md)

## Conclusion

For more details, please connect via [LinkedIn](https://www.linkedin.com/in/pgwl/) or GitHub. Happy migrating!
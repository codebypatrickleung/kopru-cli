# Kopru

Kopru is a command-line tool written in Go that automates the import of Compute instances to Oracle Cloud Infrastructure (OCI). While manual steps through the OCI console is possible, it quickly becomes cumbersome and error-prone when handling intricate setups. Kopru simplifies this process by providing an extensible, repeatable workflow that accelerates and standardises Compute deployments into OCI.

## Features

- **Simple CLI**: Start an import with just a few parameters.
- **Go Implementation**: Built with Go, using Cobra and Viper for CLI and config management.
- **Native SDK Integration**: Uses official Azure and OCI Go SDKs for authentication and performance.
- **Workflow Resume**: `--skip-*` flags let you bypass completed steps and resume migrations.
- **OpenTofu Support**: Generates OpenTofu (Terraform-compatible) templates for OCI deployments.
- **Extensible & Open Source**: Easily adaptable for new platforms and OSes.
- **Multiple Source Options**: Supports Azure VM migration and direct Linux cloud image deployment.

## Supported Workflows (More workflows coming)

Kopru currently supports two main import workflows:

### Migrate Azure VMs to OCI

Migrating VMs from Azure to OCI can be complex. Kopru automates this process by exporting the VM, converting it to a compatible format, and generating an OpenTofu template for deployment in OCI. If your source VM has data disks attached, Kopru will automatically migrate and reattach them in OCI.

[📖 View Detailed Azure to OCI Migration Guide](docs/azure-to-oci-migration.md)

### Import Linux Cloud Images to OCI

For OSes that are not available in OCI-provided images or the Marketplace, you can use this workflow to deploy Linux cloud images directly to OCI.

[📖 View Detailed Linux Cloud Image Deployment Guide](docs/linux-image-deployment.md)

## Conclusion

For more details, please feel free to connect with me on [LinkedIn](https://www.linkedin.com/in/pgwl/) or GitHub. Happy migrating!

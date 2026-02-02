# Authentication Setup

Kopru does not manage authentication itself. You must configure authentication for Azure and OCI using official SDK or CLI tools, just as you would for other CLI utilities.

## Azure Authentication (for Azure to OCI Migration)

Kopru uses `Service Principal` for Azure authentication. For Azure, Kopru requires the `Disk Snapshot Contributor` and `Reader` roles on the VM's resource group.

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

## OCI Authentication (required for all workflows)

Kopru uses `API Key-Based Authentication` for OCI authentication. For OCI, ensure your user or group has the necessary IAM policies for the target compartment.

See [OCI Authentication documentation](https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/cliinstall.htm#configfile) for more details.

### Setting up OCI Credentials

Essentially, you just need to run `oci setup config`, and this config file will be used by Kopru as well as OpenTofu (or Terraform) automatically.

```bash
oci setup config
```

This command will guide you through setting up your OCI configuration file, which typically includes:

- User OCID
- Tenancy OCID
- Region
- API Key fingerprint
- Path to your private key file

### Required OCI Permissions

Ensure your user or group has the necessary IAM policies for the target compartment, including:

- Ability to create and manage compute instances
- Ability to create and manage custom images
- Ability to create and manage object storage buckets and objects
- Ability to create and manage networking resources (if needed)

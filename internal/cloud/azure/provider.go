// Package azure provides Azure cloud operations.
package azure

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/codebypatrickleung/kopru-cli/internal/logger"
)

// Provider implements Azure cloud operations.
type Provider struct {
	subscriptionID string
	credential     azcore.TokenCredential
	logger         *logger.Logger
}

// NewProvider creates a new Azure provider instance.
func NewProvider(subscriptionID string, log *logger.Logger) (*Provider, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credential: %w", err)
	}
	fmt.Println("Successfully created DefaultAzureCredential")
	return &Provider{
		subscriptionID: subscriptionID,
		credential:     cred,
		logger:         log,
	}, nil
}

// CheckComputeExists checks if a Compute instance exists and is accessible.
func (p *Provider) CheckComputeExists(ctx context.Context, resourceGroup, computeName string) error {
	_, err := p.GetComputeInfo(ctx, resourceGroup, computeName)
	if err != nil {
		return fmt.Errorf("compute instance %s not found or not accessible: %w", computeName, err)
	}
	return nil
}

// GetComputeInfo retrieves information about a Compute instance.
func (p *Provider) GetComputeInfo(ctx context.Context, resourceGroup, computeName string) (*armcompute.VirtualMachine, error) {
	p.logger.Debugf("Getting Compute info for %s in resource group %s", computeName, resourceGroup)
	clientFactory, err := armcompute.NewClientFactory(p.subscriptionID, p.credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute client factory: %w", err)
	}
	vmClient := clientFactory.NewVirtualMachinesClient()
	vm, err := vmClient.Get(ctx, resourceGroup, computeName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get Compute instance: %w", err)
	}
	return &vm.VirtualMachine, nil
}

// GetComputeOSType retrieves the OS type of a Compute instance.
func (p *Provider) GetComputeOSType(ctx context.Context, resourceGroup, computeName string) (string, error) {
	vm, err := p.GetComputeInfo(ctx, resourceGroup, computeName)
	if err != nil {
		return "", err
	}
	if vm.Properties == nil || vm.Properties.StorageProfile == nil || vm.Properties.StorageProfile.OSDisk == nil {
		return "", fmt.Errorf("compute instance storage profile not found")
	}
	return string(*vm.Properties.StorageProfile.OSDisk.OSType), nil
}

// CheckComputeIsStopped checks if the Compute instance is stopped or deallocated.
func (p *Provider) CheckComputeIsStopped(ctx context.Context, resourceGroup, computeName string) (bool, error) {
	clientFactory, err := armcompute.NewClientFactory(p.subscriptionID, p.credential, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create compute client factory: %w", err)
	}
	vmClient := clientFactory.NewVirtualMachinesClient()
	instanceView, err := vmClient.InstanceView(ctx, resourceGroup, computeName, nil)
	if err != nil {
		return false, fmt.Errorf("failed to get Compute instance view: %w", err)
	}
	if instanceView.Statuses == nil {
		return false, fmt.Errorf("compute instance view has no statuses")
	}
	for _, status := range instanceView.Statuses {
		if status.Code == nil {
			continue
		}
		code := *status.Code
		if code == "PowerState/deallocated" || code == "PowerState/stopped" {
			return true, nil
		}
	}
	return false, nil
}

// GetComputeOSDiskName retrieves the OS disk name from a Compute instance.
func (p *Provider) GetComputeOSDiskName(ctx context.Context, resourceGroup, computeName string) (string, error) {
	vm, err := p.GetComputeInfo(ctx, resourceGroup, computeName)
	if err != nil {
		return "", err
	}
	if vm.Properties == nil || vm.Properties.StorageProfile == nil || vm.Properties.StorageProfile.OSDisk == nil {
		return "", fmt.Errorf("compute instance storage profile not found")
	}
	if vm.Properties.StorageProfile.OSDisk.Name == nil {
		return "", fmt.Errorf("OS disk name not found")
	}
	return *vm.Properties.StorageProfile.OSDisk.Name, nil
}

// GetComputeOSDiskSizeGB retrieves the OS disk size in GB from a Compute instance.
func (p *Provider) GetComputeOSDiskSizeGB(ctx context.Context, resourceGroup, computeName string) (int64, error) {
	vm, err := p.GetComputeInfo(ctx, resourceGroup, computeName)
	if err != nil {
		return 0, err
	}
	if vm.Properties == nil || vm.Properties.StorageProfile == nil || vm.Properties.StorageProfile.OSDisk == nil {
		return 0, fmt.Errorf("compute instance storage profile not found")
	}
	if vm.Properties.StorageProfile.OSDisk.DiskSizeGB == nil {
		return 0, fmt.Errorf("OS disk size property is not available in Azure VM")
	}
	return int64(*vm.Properties.StorageProfile.OSDisk.DiskSizeGB), nil
}

// GetComputeDataDiskNames retrieves the names of all data disks attached to a Compute instance.
func (p *Provider) GetComputeDataDiskNames(ctx context.Context, resourceGroup, computeName string) ([]string, error) {
	vm, err := p.GetComputeInfo(ctx, resourceGroup, computeName)
	if err != nil {
		return nil, err
	}
	if vm.Properties == nil || vm.Properties.StorageProfile == nil {
		return nil, fmt.Errorf("Compute instance storage profile not found")
	}
	var diskNames []string
	if vm.Properties.StorageProfile.DataDisks != nil {
		for _, disk := range vm.Properties.StorageProfile.DataDisks {
			if disk.Name != nil {
				diskNames = append(diskNames, *disk.Name)
			}
		}
	}
	return diskNames, nil
}

// ExportAzureDisk exports an Azure disk by creating a snapshot, generating a SAS URL, and downloading the VHD.
func (p *Provider) ExportAzureDisk(ctx context.Context, diskName, resourceGroup, exportDir string) (string, error) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 36)
	maxDiskNameLen := 80 - 4 - len(timestamp)
	truncatedDiskName := diskName
	if len(diskName) > maxDiskNameLen {
		truncatedDiskName = diskName[:maxDiskNameLen]
	}
	
	snapshotName := fmt.Sprintf("ss-%s-%s", truncatedDiskName, timestamp)
	vhdFile := filepath.Join(exportDir, fmt.Sprintf("%s.vhd", diskName))

	p.logger.Infof("Creating snapshot: %s", snapshotName)
	err := p.CreateSnapshot(ctx, resourceGroup, snapshotName, diskName)
	if err != nil {
		return "", fmt.Errorf("failed to create snapshot: %w", err)
	}
	p.logger.Success("✓ Snapshot created")

	defer func() {
		p.logger.Info("Cleaning up snapshot...")
		if err := p.RevokeSnapshotAccess(ctx, resourceGroup, snapshotName); err != nil {
			p.logger.Warning(fmt.Sprintf("Failed to revoke access to snapshot: %v", err))
		}
		if err := p.DeleteSnapshot(ctx, resourceGroup, snapshotName); err != nil {
			p.logger.Warning(fmt.Sprintf("Failed to delete snapshot %s - manual cleanup may be required", snapshotName))
		} else {
			p.logger.Success("✓ Snapshot cleaned up")
		}
	}()

	p.logger.Info("Generating SAS URL...")
	sasURL, err := p.GrantSnapshotAccess(ctx, resourceGroup, snapshotName, 7200)
	if err != nil {
		return "", fmt.Errorf("failed to generate SAS URL: %w", err)
	}
	p.logger.Success("✓ SAS URL generated")

	p.logger.Info("Downloading disk (this may take a while)...")
	err = p.DownloadFromSASURL(ctx, sasURL, vhdFile)
	if err != nil {
		return "", fmt.Errorf("failed to download disk: %w", err)
	}

	p.logger.Successf("✓ Disk downloaded: %s", vhdFile)
	return vhdFile, nil
}

// CreateSnapshot creates a snapshot of a disk.
func (p *Provider) CreateSnapshot(ctx context.Context, resourceGroup, snapshotName, diskName string) error {
	clientFactory, err := armcompute.NewClientFactory(p.subscriptionID, p.credential, nil)
	if err != nil {
		return fmt.Errorf("failed to create compute client factory: %w", err)
	}
	snapshotsClient := clientFactory.NewSnapshotsClient()
	disksClient := clientFactory.NewDisksClient()
	disk, err := disksClient.Get(ctx, resourceGroup, diskName, nil)
	if err != nil {
		return fmt.Errorf("failed to get disk: %w", err)
	}
	createOption := armcompute.DiskCreateOptionCopy
	poller, err := snapshotsClient.BeginCreateOrUpdate(ctx, resourceGroup, snapshotName,
		armcompute.Snapshot{
			Location: disk.Location,
			Properties: &armcompute.SnapshotProperties{
				CreationData: &armcompute.CreationData{
					CreateOption:     &createOption,
					SourceResourceID: disk.ID,
				},
			},
		}, nil)
	if err != nil {
		return fmt.Errorf("failed to begin snapshot creation: %w", err)
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}
	return nil
}

// GrantSnapshotAccess grants read access to a snapshot and returns the SAS URL.
func (p *Provider) GrantSnapshotAccess(ctx context.Context, resourceGroup, snapshotName string, durationInSeconds int32) (string, error) {
	clientFactory, err := armcompute.NewClientFactory(p.subscriptionID, p.credential, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create compute client factory: %w", err)
	}
	snapshotsClient := clientFactory.NewSnapshotsClient()
	accessLevel := armcompute.AccessLevelRead
	poller, err := snapshotsClient.BeginGrantAccess(ctx, resourceGroup, snapshotName,
		armcompute.GrantAccessData{
			Access:            &accessLevel,
			DurationInSeconds: &durationInSeconds,
		}, nil)
	if err != nil {
		return "", fmt.Errorf("failed to begin grant access: %w", err)
	}
	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to grant access: %w", err)
	}
	if result.AccessSAS == nil || *result.AccessSAS == "" {
		return "", fmt.Errorf("no access SAS returned")
	}
	return *result.AccessSAS, nil
}

// DownloadFromSASURL downloads a file from an Azure blob using a SAS URL.
func (p *Provider) DownloadFromSASURL(ctx context.Context, sasURL, destFile string) error {
	blobClient, err := blob.NewClientWithNoCredential(sasURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create blob client: %w", err)
	}
	out, err := os.Create(destFile)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()
	_, err = blobClient.DownloadFile(ctx, out, nil)
	if err != nil {
		return fmt.Errorf("failed to download blob: %w", err)
	}
	return nil
}

// RevokeSnapshotAccess revokes access to a snapshot.
func (p *Provider) RevokeSnapshotAccess(ctx context.Context, resourceGroup, snapshotName string) error {
	clientFactory, err := armcompute.NewClientFactory(p.subscriptionID, p.credential, nil)
	if err != nil {
		return fmt.Errorf("failed to create compute client factory: %w", err)
	}
	snapshotsClient := clientFactory.NewSnapshotsClient()
	poller, err := snapshotsClient.BeginRevokeAccess(ctx, resourceGroup, snapshotName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin revoke access: %w", err)
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to revoke access: %w", err)
	}
	return nil
}

// DeleteSnapshot deletes a snapshot.
func (p *Provider) DeleteSnapshot(ctx context.Context, resourceGroup, snapshotName string) error {
	clientFactory, err := armcompute.NewClientFactory(p.subscriptionID, p.credential, nil)
	if err != nil {
		return fmt.Errorf("failed to create compute client factory: %w", err)
	}
	snapshotsClient := clientFactory.NewSnapshotsClient()
	poller, err := snapshotsClient.BeginDelete(ctx, resourceGroup, snapshotName, nil)
	if err != nil {
		return fmt.Errorf("failed to begin snapshot deletion: %w", err)
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}
	return nil
}

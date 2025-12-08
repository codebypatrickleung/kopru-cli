// Package oci provides OCI operations.
package oci

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/codebypatrickleung/kopru-cli/internal/logger"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

// Provider implements OCI cloud operations.
type Provider struct {
	configProvider common.ConfigurationProvider
	region         string
	logger         *logger.Logger
}

// NewProvider creates a new OCI provider instance.
func NewProvider(region string, log *logger.Logger) (*Provider, error) {
	configProvider := common.DefaultConfigProvider()
	return &Provider{
		configProvider: configProvider,
		region:         region,
		logger:         log,
	}, nil
}

// GetNamespace retrieves the Object Storage namespace for the tenancy.
func (p *Provider) GetNamespace(ctx context.Context) (string, error) {
	client, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(p.configProvider)
	if err != nil {
		return "", fmt.Errorf("failed to create object storage client: %w", err)
	}
	req := objectstorage.GetNamespaceRequest{}
	resp, err := client.GetNamespace(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get namespace: %w", err)
	}
	return *resp.Value, nil
}

// CheckBucketExists checks if a bucket exists.
func (p *Provider) CheckBucketExists(ctx context.Context, namespace, bucketName string) (bool, error) {
	client, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(p.configProvider)
	if err != nil {
		return false, fmt.Errorf("failed to create object storage client: %w", err)
	}
	req := objectstorage.HeadBucketRequest{
		NamespaceName: &namespace,
		BucketName:    &bucketName,
	}
	_, err = client.HeadBucket(ctx, req)
	if err != nil {
		if serviceErr, ok := common.IsServiceError(err); ok && serviceErr.GetHTTPStatusCode() == 404 {
			return false, nil
		}
		return false, fmt.Errorf("failed to check bucket: %w", err)
	}
	return true, nil
}

// CreateBucket creates a new bucket.
func (p *Provider) CreateBucket(ctx context.Context, namespace, compartmentID, bucketName string) error {
	client, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(p.configProvider)
	if err != nil {
		return fmt.Errorf("failed to create object storage client: %w", err)
	}
	req := objectstorage.CreateBucketRequest{
		NamespaceName: &namespace,
		CreateBucketDetails: objectstorage.CreateBucketDetails{
			Name:          &bucketName,
			CompartmentId: &compartmentID,
		},
	}
	_, err = client.CreateBucket(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create bucket: %w", err)
	}
	p.logger.Successf("Created bucket: %s", bucketName)
	return nil
}

// CheckCompartmentExists checks if a compartment is accessible.
func (p *Provider) CheckCompartmentExists(ctx context.Context, compartmentID string) error {
	client, err := identity.NewIdentityClientWithConfigurationProvider(p.configProvider)
	if err != nil {
		return fmt.Errorf("failed to create identity client: %w", err)
	}
	req := identity.GetCompartmentRequest{
		CompartmentId: &compartmentID,
	}
	_, err = client.GetCompartment(ctx, req)
	if err != nil {
		return fmt.Errorf("compartment not accessible: %w", err)
	}
	return nil
}

// CheckSubnetExists checks if a subnet is accessible.
func (p *Provider) CheckSubnetExists(ctx context.Context, subnetID string) error {
	client, err := core.NewVirtualNetworkClientWithConfigurationProvider(p.configProvider)
	if err != nil {
		return fmt.Errorf("failed to create virtual network client: %w", err)
	}
	req := core.GetSubnetRequest{
		SubnetId: &subnetID,
	}
	_, err = client.GetSubnet(ctx, req)
	if err != nil {
		return fmt.Errorf("subnet not accessible: %w", err)
	}
	return nil
}

// GetFirstAvailabilityDomain retrieves the first availability domain in a compartment.
func (p *Provider) GetFirstAvailabilityDomain(ctx context.Context, compartmentID string) (string, error) {
	client, err := identity.NewIdentityClientWithConfigurationProvider(p.configProvider)
	if err != nil {
		return "", fmt.Errorf("failed to create identity client: %w", err)
	}
	req := identity.ListAvailabilityDomainsRequest{
		CompartmentId: &compartmentID,
	}
	resp, err := client.ListAvailabilityDomains(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to list availability domains: %w", err)
	}
	if len(resp.Items) == 0 {
		return "", fmt.Errorf("no availability domains found")
	}
	return *resp.Items[0].Name, nil
}

// GetLocalAvailabilityDomain retrieves the availability domain of the local instance.
func (p *Provider) GetLocalAvailabilityDomain(ctx context.Context, instanceID string) (string, error) {
	client, err := core.NewComputeClientWithConfigurationProvider(p.configProvider)
	if err != nil {
		return "", fmt.Errorf("failed to create compute client: %w", err)
	}
	req := core.GetInstanceRequest{
		InstanceId: &instanceID,
	}
	resp, err := client.GetInstance(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get instance details: %w", err)
	}
	if resp.AvailabilityDomain == nil {
		return "", fmt.Errorf("instance has no availability domain")
	}
	return *resp.AvailabilityDomain, nil
}

// UploadToObjectStorage uploads a file to OCI Object Storage.
func (p *Provider) UploadToObjectStorage(ctx context.Context, namespace, bucketName, objectName, filePath string) error {
	client, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(p.configProvider)
	if err != nil {
		return fmt.Errorf("failed to create object storage client: %w", err)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	contentLength := fileInfo.Size()
	req := objectstorage.PutObjectRequest{
		NamespaceName: &namespace,
		BucketName:    &bucketName,
		ObjectName:    &objectName,
		PutObjectBody: file,
		ContentLength: &contentLength,
	}
	_, err = client.PutObject(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}
	p.logger.Successf("Uploaded %s to bucket %s", objectName, bucketName)
	return nil
}

// ImportCustomImage imports a custom image from Object Storage.
func (p *Provider) ImportCustomImage(ctx context.Context, compartmentID, displayName, namespace, bucketName, objectName, operatingSystem string) (string, error) {
	client, err := core.NewComputeClientWithConfigurationProvider(p.configProvider)
	if err != nil {
		return "", fmt.Errorf("failed to create compute client: %w", err)
	}
	encodedObjectName := url.PathEscape(objectName)
	sourceURL := fmt.Sprintf("https://objectstorage.%s.oraclecloud.com/n/%s/b/%s/o/%s",
		p.region, namespace, bucketName, encodedObjectName)
	if operatingSystem == "" {
		operatingSystem = "Generic Linux"
	}
	osPtr := common.String(operatingSystem)
	req := core.CreateImageRequest{
		CreateImageDetails: core.CreateImageDetails{
			CompartmentId: &compartmentID,
			DisplayName:   &displayName,
			LaunchMode:    core.CreateImageDetailsLaunchModeParavirtualized,
			ImageSourceDetails: core.ImageSourceViaObjectStorageUriDetails{
				SourceUri:       &sourceURL,
				OperatingSystem: osPtr,
			},
		},
	}
	resp, err := client.CreateImage(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to create image: %w", err)
	}
	imageID := *resp.Image.Id
	p.logger.Successf("Custom image import started: %s", imageID)
	p.logger.Info("Image import can take a while to complete")
	return imageID, nil
}

// WaitForImageAvailable polls the image status until it reaches "AVAILABLE" state.
func (p *Provider) WaitForImageAvailable(ctx context.Context, imageID string) error {
	client, err := core.NewComputeClientWithConfigurationProvider(p.configProvider)
	if err != nil {
		return fmt.Errorf("failed to create compute client: %w", err)
	}
	p.logger.Info("Waiting for image import to complete...")
	p.logger.Infof("Image ID: %s", imageID)
	pollInterval := 30 * time.Second
	timeout := 2 * time.Hour
	startTime := time.Now()
	for {
		if time.Since(startTime) > timeout {
			return fmt.Errorf("timeout waiting for image to become available after %v", timeout)
		}
		req := core.GetImageRequest{
			ImageId: &imageID,
		}
		resp, err := client.GetImage(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to get image status: %w", err)
		}
		lifecycleState := resp.Image.LifecycleState
		p.logger.Debugf("Current image lifecycle state: %s", lifecycleState)
		if lifecycleState == core.ImageLifecycleStateAvailable {
			p.logger.Successf("Image import completed successfully and is now AVAILABLE")
			return nil
		}
		if lifecycleState == core.ImageLifecycleStateDeleted ||
			lifecycleState == core.ImageLifecycleStateDisabled {
			return fmt.Errorf("image import failed with state: %s", lifecycleState)
		}
		p.logger.Infof("Image status: %s - waiting %v before next check...", lifecycleState, pollInterval)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// GetLocalInstanceID retrieves the OCID of the local OCI instance.
func (p *Provider) GetLocalInstanceID(ctx context.Context) (string, error) {
	cmd := exec.Command("oci-metadata", "--get", "/instance/id", "--value-only")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get instance ID from metadata service: %w", err)
	}
	instanceID := strings.TrimSpace(string(output))
	if instanceID == "" {
		return "", fmt.Errorf("empty instance ID returned from metadata service")
	}
	return instanceID, nil
}

// CreateBlockVolume creates a new block volume.
func (p *Provider) CreateBlockVolume(ctx context.Context, compartmentID, availabilityDomain, displayName string, sizeInGBs int64) (string, error) {
	client, err := core.NewBlockstorageClientWithConfigurationProvider(p.configProvider)
	if err != nil {
		return "", fmt.Errorf("failed to create block storage client: %w", err)
	}
	req := core.CreateVolumeRequest{
		CreateVolumeDetails: core.CreateVolumeDetails{
			CompartmentId:      &compartmentID,
			AvailabilityDomain: &availabilityDomain,
			DisplayName:        &displayName,
			SizeInGBs:          &sizeInGBs,
		},
	}
	resp, err := client.CreateVolume(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to create volume: %w", err)
	}
	volumeID := *resp.Id
	p.logger.Info("Waiting for volume to become available...")
	err = p.WaitForVolumeState(ctx, volumeID, core.VolumeLifecycleStateAvailable)
	if err != nil {
		return "", fmt.Errorf("volume did not become available: %w", err)
	}
	return volumeID, nil
}

// WaitForVolumeState waits for a volume to reach the specified state.
func (p *Provider) WaitForVolumeState(ctx context.Context, volumeID string, targetState core.VolumeLifecycleStateEnum) error {
	client, err := core.NewBlockstorageClientWithConfigurationProvider(p.configProvider)
	if err != nil {
		return fmt.Errorf("failed to create block storage client: %w", err)
	}
	maxAttempts := 60
	for i := 0; i < maxAttempts; i++ {
		req := core.GetVolumeRequest{
			VolumeId: &volumeID,
		}
		resp, err := client.GetVolume(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to get volume state: %w", err)
		}
		if resp.LifecycleState == targetState {
			return nil
		}
		if resp.LifecycleState == core.VolumeLifecycleStateFaulty {
			return fmt.Errorf("volume entered faulty state")
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timeout waiting for volume to reach state %s", targetState)
}

// AttachVolume attaches a volume to an instance.
func (p *Provider) AttachVolume(ctx context.Context, instanceID, volumeID string) (string, error) {
	client, err := core.NewComputeClientWithConfigurationProvider(p.configProvider)
	if err != nil {
		return "", fmt.Errorf("failed to create compute client: %w", err)
	}
	req := core.AttachVolumeRequest{
		AttachVolumeDetails: core.AttachParavirtualizedVolumeDetails{
			InstanceId: &instanceID,
			VolumeId:   &volumeID,
		},
	}
	resp, err := client.AttachVolume(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to attach volume: %w", err)
	}
	attachmentID := *resp.VolumeAttachment.GetId()
	p.logger.Info("Waiting for volume attachment to complete...")
	err = p.WaitForVolumeAttachmentState(ctx, attachmentID, core.VolumeAttachmentLifecycleStateAttached)
	if err != nil {
		return "", fmt.Errorf("volume attachment failed: %w", err)
	}
	return attachmentID, nil
}

// WaitForVolumeAttachmentState waits for a volume attachment to reach the specified state.
func (p *Provider) WaitForVolumeAttachmentState(ctx context.Context, attachmentID string, targetState core.VolumeAttachmentLifecycleStateEnum) error {
	client, err := core.NewComputeClientWithConfigurationProvider(p.configProvider)
	if err != nil {
		return fmt.Errorf("failed to create compute client: %w", err)
	}
	maxAttempts := 60
	for i := 0; i < maxAttempts; i++ {
		req := core.GetVolumeAttachmentRequest{
			VolumeAttachmentId: &attachmentID,
		}
		resp, err := client.GetVolumeAttachment(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to get volume attachment state: %w", err)
		}
		if resp.VolumeAttachment.GetLifecycleState() == targetState {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timeout waiting for volume attachment to reach state %s", targetState)
}

// DetachVolume detaches a volume from an instance.
func (p *Provider) DetachVolume(ctx context.Context, attachmentID string) error {
	client, err := core.NewComputeClientWithConfigurationProvider(p.configProvider)
	if err != nil {
		return fmt.Errorf("failed to create compute client: %w", err)
	}
	req := core.DetachVolumeRequest{
		VolumeAttachmentId: &attachmentID,
	}
	_, err = client.DetachVolume(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to detach volume: %w", err)
	}
	p.logger.Info("Waiting for volume detachment to complete...")
	err = p.WaitForVolumeAttachmentState(ctx, attachmentID, core.VolumeAttachmentLifecycleStateDetached)
	if err != nil {
		return fmt.Errorf("volume detachment failed: %w", err)
	}
	return nil
}

// CreateVolumeSnapshot creates a snapshot (backup) of a block volume.
func (p *Provider) CreateVolumeSnapshot(ctx context.Context, volumeID, displayName string) (string, error) {
	client, err := core.NewBlockstorageClientWithConfigurationProvider(p.configProvider)
	if err != nil {
		return "", fmt.Errorf("failed to create block storage client: %w", err)
	}
	backupType := core.CreateVolumeBackupDetailsTypeFull
	req := core.CreateVolumeBackupRequest{
		CreateVolumeBackupDetails: core.CreateVolumeBackupDetails{
			VolumeId:    &volumeID,
			DisplayName: &displayName,
			Type:        backupType,
		},
	}
	resp, err := client.CreateVolumeBackup(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to create volume backup: %w", err)
	}
	snapshotID := *resp.Id
	p.logger.Info("Waiting for snapshot to become available...")
	err = p.WaitForSnapshotState(ctx, snapshotID, core.VolumeBackupLifecycleStateAvailable)
	if err != nil {
		return "", fmt.Errorf("snapshot did not become available: %w", err)
	}
	return snapshotID, nil
}

// WaitForSnapshotState waits for a volume snapshot to reach the specified state.
func (p *Provider) WaitForSnapshotState(ctx context.Context, snapshotID string, targetState core.VolumeBackupLifecycleStateEnum) error {
	client, err := core.NewBlockstorageClientWithConfigurationProvider(p.configProvider)
	if err != nil {
		return fmt.Errorf("failed to create block storage client: %w", err)
	}
	maxAttempts := 120
	for i := 0; i < maxAttempts; i++ {
		req := core.GetVolumeBackupRequest{
			VolumeBackupId: &snapshotID,
		}
		resp, err := client.GetVolumeBackup(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to get snapshot state: %w", err)
		}
		if resp.LifecycleState == targetState {
			return nil
		}
		if resp.LifecycleState == core.VolumeBackupLifecycleStateFaulty {
			return fmt.Errorf("snapshot entered faulty state")
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timeout waiting for snapshot to reach state %s", targetState)
}

// DeleteVolume deletes a block volume.
func (p *Provider) DeleteVolume(ctx context.Context, volumeID string) error {
	client, err := core.NewBlockstorageClientWithConfigurationProvider(p.configProvider)
	if err != nil {
		return fmt.Errorf("failed to create block storage client: %w", err)
	}
	req := core.DeleteVolumeRequest{
		VolumeId: &volumeID,
	}
	_, err = client.DeleteVolume(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete volume: %w", err)
	}
	err = p.WaitForVolumeState(ctx, volumeID, core.VolumeLifecycleStateTerminated)
	if err != nil {
		p.logger.Warning(fmt.Sprintf("Could not verify volume deletion: %v", err))
	}
	return nil
}

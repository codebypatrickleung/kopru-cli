// Package workflow provides workflow handlers for specific migration paths.
package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/codebypatrickleung/kopru-cli/internal/cloud/azure"
	"github.com/codebypatrickleung/kopru-cli/internal/cloud/oci"
	"github.com/codebypatrickleung/kopru-cli/internal/common"
	"github.com/codebypatrickleung/kopru-cli/internal/config"
	"github.com/codebypatrickleung/kopru-cli/internal/logger"
	ospackage "github.com/codebypatrickleung/kopru-cli/internal/os"
	"github.com/codebypatrickleung/kopru-cli/internal/template"
)

const (
	// DefaultOSType is the default operating system type for image configurations
	DefaultOSType = "Ubuntu"
)

// AzureToOCIHandler implements the workflow for migrating Compute instances from Azure to OCI.
type AzureToOCIHandler struct {
	config                *config.Config
	logger                *logger.Logger
	azureProvider         *azure.Provider
	ociProvider           *oci.Provider
	importedImageID       string   // Store the imported image ID for use in template
	dataDiskSnapshotIDs   []string // Store snapshot IDs for data disks
	dataDiskSnapshotNames []string // Store snapshot names for data disks
}

// NewAzureToOCIHandler creates a new Azure to OCI workflow handler.
func NewAzureToOCIHandler() *AzureToOCIHandler {
	return &AzureToOCIHandler{}
}

// Name returns the name of this workflow handler.
func (h *AzureToOCIHandler) Name() string {
	return "Azure to OCI Migration"
}

// SourcePlatform returns the source platform identifier.
func (h *AzureToOCIHandler) SourcePlatform() string {
	return "azure"
}

// TargetPlatform returns the target platform identifier.
func (h *AzureToOCIHandler) TargetPlatform() string {
	return "oci"
}

// Initialize prepares the workflow handler with configuration and logger.
func (h *AzureToOCIHandler) Initialize(cfg *config.Config, log *logger.Logger) error {
	h.config = cfg
	h.logger = log

	// Initialize Azure provider
	azureProvider, err := azure.NewProvider(cfg.AzureSubscriptionID, log)
	if err != nil {
		return fmt.Errorf("failed to initialize Azure provider: %w", err)
	}
	h.azureProvider = azureProvider

	// Initialize OCI provider
	ociProvider, err := oci.NewProvider(cfg.OCIRegion, log)
	if err != nil {
		return fmt.Errorf("failed to initialize OCI provider: %w", err)
	}
	h.ociProvider = ociProvider

	return nil
}

// Execute runs the complete Azure to OCI migration workflow.
func (h *AzureToOCIHandler) Execute(ctx context.Context) error {
	h.logger.Info("=========================================")
	h.logger.Infof("Executing: %s", h.Name())
	h.logger.Info("=========================================")

	// Step 1: Prerequisites check
	if !h.config.SkipPrereq {
		if err := h.runPrerequisites(ctx); err != nil {
			return fmt.Errorf("prerequisite checks failed: %w", err)
		}
	} else {
		h.logger.Warning("Skipping prerequisite checks (SKIP_PREREQ=true)")
	}

	// Step 2: Export OS disk
	if !h.config.SkipExport {
		if err := h.exportOSDisk(ctx); err != nil {
			return fmt.Errorf("OS disk export failed: %w", err)
		}
	} else {
		h.logger.Warning("Skipping OS disk export (SKIP_OS_EXPORT=true)")
	}

	// Step 3: Convert VHD to QCOW2
	if !h.config.SkipConvert {
		if err := h.convertDisk(ctx); err != nil {
			return fmt.Errorf("disk conversion failed: %w", err)
		}
	} else {
		h.logger.Warning("Skipping disk conversion (SKIP_OS_CONVERT=true)")
	}

	// Step 4: Configure image for OCI
	if !h.config.SkipConfigure {
		if err := h.configureImage(ctx); err != nil {
			return fmt.Errorf("image configuration failed: %w", err)
		}
	} else {
		h.logger.Warning("Skipping image configuration (SKIP_OS_CONFIGURE=true)")
	}

	// Step 5: Upload image to OCI
	if !h.config.SkipUpload {
		if err := h.uploadImage(ctx); err != nil {
			return fmt.Errorf("image upload failed: %w", err)
		}
	} else {
		h.logger.Warning("Skipping image upload (SKIP_OS_UPLOAD=true)")
	}

	// Step 6: Import custom image
	if !h.config.SkipImport {
		if err := h.importImage(ctx); err != nil {
			return fmt.Errorf("image import failed: %w", err)
		}
	} else {
		h.logger.Warning("Skipping image import (SKIP_OS_IMPORT=true)")
	}

	// Step 7: Export data disks
	if !h.config.SkipDDExport {
		if err := h.exportDataDisks(ctx); err != nil {
			return fmt.Errorf("data disk export failed: %w", err)
		}
	} else {
		h.logger.Warning("Skipping data disk export (SKIP_DD_EXPORT=true)")
	}

	// Step 8: Import data disks
	if !h.config.SkipDDImport {
		if err := h.importDataDisks(ctx); err != nil {
			return fmt.Errorf("data disk import failed: %w", err)
		}
	} else {
		h.logger.Warning("Skipping data disk import (SKIP_DD_IMPORT=true)")
	}

	// Step 9: Generate template
	if !h.config.SkipTemplate {
		if err := h.generateTemplate(ctx); err != nil {
			return fmt.Errorf("template generation failed: %w", err)
		}
	} else {
		h.logger.Warning("Skipping template generation (SKIP_TEMPLATE=true)")
	}

	// Step 10: Wait for image import to complete
	if !h.config.SkipImport && h.importedImageID != "" {
		if err := h.waitForImageAvailable(ctx); err != nil {
			return fmt.Errorf("image availability check failed: %w", err)
		}
	}

	// Step 11: Deploy with template
	if !h.config.SkipTemplateDeploy {
		if err := h.deployTemplate(ctx); err != nil {
			return fmt.Errorf("template deployment failed: %w", err)
		}
	} else {
		h.logger.Warning("Skipping template deployment (SKIP_TEMPLATE_DEPLOY=true)")
		h.logger.Infof("To deploy manually, run: cd %s && tofu init && tofu apply", h.config.TemplateOutputDir)
	}

	// Step 12: Verify workflow
	if !h.config.SkipVerify {
		if err := h.verifyWorkflow(ctx); err != nil {
			return fmt.Errorf("workflow verification failed: %w", err)
		}
	} else {
		h.logger.Warning("Skipping workflow verification (SKIP_VERIFY=true)")
	}

	h.logger.Success("=========================================")
	h.logger.Success("Azure to OCI migration completed successfully!")
	h.logger.Success("=========================================")

	return nil
}

// Workflow step implementations

func (h *AzureToOCIHandler) runPrerequisites(ctx context.Context) error {
	h.logger.Step(1, "Running Prerequisite Checks")

	// Check required commands
	h.logger.Info("Checking required tools...")
	requiredTools := []string{"qemu-img"}
	for _, tool := range requiredTools {
		if err := common.CheckCommand(tool); err != nil {
			return fmt.Errorf("required tool missing: %w", err)
		}
		h.logger.Successf("✓ %s is installed", tool)
	}

	// Check disk space (at least 100GB recommended)
	h.logger.Info("Checking disk space...")
	if err := common.CheckDiskSpace(".", common.MinDiskSpaceGB); err != nil {
		h.logger.Warning(fmt.Sprintf("Disk space check: %v", err))
	}

	// Azure prerequisites
	h.logger.Info("Checking Azure Compute instance...")
	if err := h.azureProvider.CheckComputeExists(ctx, h.config.AzureResourceGroup, h.config.AzureComputeName); err != nil {
		return fmt.Errorf("Azure Compute instance check failed: %w", err)
	}
	h.logger.Successf("✓ Azure Compute instance '%s' is accessible", h.config.AzureComputeName)

	// Check Compute instance OS type
	osType, err := h.azureProvider.GetComputeOSType(ctx, h.config.AzureResourceGroup, h.config.AzureComputeName)
	if err != nil {
		return fmt.Errorf("failed to get Compute instance OS type: %w", err)
	}
	if osType != "Linux" {
		return fmt.Errorf("only Linux Compute instances are currently supported, found: %s", osType)
	}
	h.logger.Successf("✓ Compute instance OS type: %s", osType)

	// Check Compute instance is stopped
	isStopped, err := h.azureProvider.CheckComputeIsStopped(ctx, h.config.AzureResourceGroup, h.config.AzureComputeName)
	if err != nil {
		return fmt.Errorf("failed to check Compute instance state: %w", err)
	}
	if !isStopped {
		h.logger.Warning("Compute instance is running - it's recommended to stop the instance before export to ensure data consistency")
	} else {
		h.logger.Success("✓ Compute instance is stopped")
	}

	// OCI prerequisites
	h.logger.Info("Checking OCI compartment...")
	if err := h.ociProvider.CheckCompartmentExists(ctx, h.config.OCICompartmentID); err != nil {
		return fmt.Errorf("OCI compartment check failed: %w", err)
	}
	h.logger.Success("✓ OCI compartment is accessible")

	h.logger.Info("Checking OCI subnet...")
	if err := h.ociProvider.CheckSubnetExists(ctx, h.config.OCISubnetID); err != nil {
		return fmt.Errorf("OCI subnet check failed: %w", err)
	}
	h.logger.Success("✓ OCI subnet is accessible")

	// Get or create bucket
	namespace, err := h.ociProvider.GetNamespace(ctx)
	if err != nil {
		return fmt.Errorf("failed to get OCI namespace: %w", err)
	}
	h.logger.Successf("✓ OCI namespace: %s", namespace)

	bucketExists, err := h.ociProvider.CheckBucketExists(ctx, namespace, h.config.OCIBucketName)
	if err != nil {
		return fmt.Errorf("failed to check bucket: %w", err)
	}
	if !bucketExists {
		h.logger.Infof("Bucket '%s' does not exist, it will be created during upload", h.config.OCIBucketName)
	} else {
		h.logger.Successf("✓ Bucket '%s' exists", h.config.OCIBucketName)
	}

	h.logger.Success("Prerequisite checks passed")
	return nil
}

func (h *AzureToOCIHandler) exportOSDisk(ctx context.Context) error {
	h.logger.Step(2, "Exporting OS Disk")

	// Create export directory
	exportDir := fmt.Sprintf("./%s-os-disk-export", common.SanitizeName(h.config.AzureComputeName))
	if err := common.EnsureDir(exportDir); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}

	h.logger.Infof("Export directory: %s", exportDir)

	// Get OS disk name using Azure SDK
	h.logger.Info("Getting OS disk name...")
	osDiskName, err := h.azureProvider.GetComputeOSDiskName(ctx, h.config.AzureResourceGroup, h.config.AzureComputeName)
	if err != nil {
		return fmt.Errorf("failed to get OS disk name: %w", err)
	}
	h.logger.Infof("OS disk name: %s", osDiskName)

	// Export the disk using Azure provider
	vhdFile, err := h.azureProvider.ExportAzureDisk(ctx, osDiskName, h.config.AzureResourceGroup, exportDir)
	if err != nil {
		return fmt.Errorf("failed to export OS disk: %w", err)
	}

	h.logger.Successf("OS disk exported to: %s", vhdFile)
	return nil
}

func (h *AzureToOCIHandler) convertDisk(ctx context.Context) error {
	h.logger.Step(3, "Converting VHD to QCOW2")

	// Find the VHD file
	exportDir := fmt.Sprintf("./%s-os-disk-export", common.SanitizeName(h.config.AzureComputeName))
	vhdFile, err := common.FindVHDFile(exportDir)
	if err != nil {
		return fmt.Errorf("failed to find VHD file: %w", err)
	}

	h.logger.Infof("Converting VHD file: %s", vhdFile)

	// Generate QCOW2 filename
	qcow2File := strings.TrimSuffix(vhdFile, ".vhd") + ".qcow2"

	// Convert using qemu-img
	h.logger.Info("Running qemu-img convert (this may take a while)...")
	if err := common.ConvertVHDToQCOW2(vhdFile, qcow2File, !h.config.KeepVHD); err != nil {
		return err
	}

	h.logger.Successf("Disk converted to QCOW2: %s", qcow2File)

	if !h.config.KeepVHD {
		h.logger.Success("VHD file removed")
	}

	return nil
}

func (h *AzureToOCIHandler) configureImage(ctx context.Context) error {
	h.logger.Step(4, "Configureing Image for OCI")

	// Find the QCOW2 file
	exportDir := fmt.Sprintf("./%s-os-disk-export", common.SanitizeName(h.config.AzureComputeName))
	qcow2File, err := common.FindQCOW2File(exportDir)
	if err != nil {
		return fmt.Errorf("failed to find QCOW2 file: %w", err)
	}

	h.logger.Infof("Configureing QCOW2 file: %s", qcow2File)

	// Determine OS type
	osType := h.config.OCIImageOS
	if osType == "" {
		osType = DefaultOSType
		h.logger.Infof("Using default OS type: %s", osType)
	}

	// Get OS configurator for the source platform and target platform
	configurator, err := ospackage.GetConfigurator(h.SourcePlatform(), h.TargetPlatform(), osType)
	if err != nil {
		return fmt.Errorf("failed to get OS configurator: %w", err)
	}

	// Apply configurations
	if err := configurator.ConfigureImage(ctx, qcow2File, h.logger, h.config); err != nil {
		return fmt.Errorf("failed to configure image: %w", err)
	}

	h.logger.Success("Image configurations complete")
	return nil
}

func (h *AzureToOCIHandler) uploadImage(ctx context.Context) error {
	h.logger.Step(5, "Uploading Image to OCI")

	// Find the QCOW2 file
	exportDir := fmt.Sprintf("./%s-os-disk-export", common.SanitizeName(h.config.AzureComputeName))
	qcow2File, err := common.FindQCOW2File(exportDir)
	if err != nil {
		return fmt.Errorf("failed to find QCOW2 file: %w", err)
	}

	// Get namespace
	namespace, err := h.ociProvider.GetNamespace(ctx)
	if err != nil {
		return fmt.Errorf("failed to get namespace: %w", err)
	}

	// Check if bucket exists, create if not
	bucketExists, err := h.ociProvider.CheckBucketExists(ctx, namespace, h.config.OCIBucketName)
	if err != nil {
		return fmt.Errorf("failed to check bucket: %w", err)
	}

	if !bucketExists {
		h.logger.Infof("Creating bucket '%s'...", h.config.OCIBucketName)
		if err := h.ociProvider.CreateBucket(ctx, namespace, h.config.OCICompartmentID, h.config.OCIBucketName); err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	// Upload the QCOW2 file
	objectName := filepath.Base(qcow2File)
	h.logger.Infof("Uploading %s to bucket %s (this may take a while)...", objectName, h.config.OCIBucketName)

	if err := h.ociProvider.UploadToObjectStorage(ctx, namespace, h.config.OCIBucketName, objectName, qcow2File); err != nil {
		return fmt.Errorf("failed to upload to Object Storage: %w", err)
	}

	h.logger.Success("Image uploaded to OCI")
	return nil
}

func (h *AzureToOCIHandler) importImage(ctx context.Context) error {
	h.logger.Step(6, "Importing Custom Image")

	// Find the QCOW2 file to get object name
	exportDir := fmt.Sprintf("./%s-os-disk-export", common.SanitizeName(h.config.AzureComputeName))
	qcow2File, err := common.FindQCOW2File(exportDir)
	if err != nil {
		return fmt.Errorf("failed to find QCOW2 file: %w", err)
	}

	objectName := filepath.Base(qcow2File)

	// Get namespace
	namespace, err := h.ociProvider.GetNamespace(ctx)
	if err != nil {
		return fmt.Errorf("failed to get namespace: %w", err)
	}

	// Import custom image
	h.logger.Infof("Importing custom image '%s'...", h.config.OCIImageName)
	imageID, err := h.ociProvider.ImportCustomImage(ctx,
		h.config.OCICompartmentID,
		h.config.OCIImageName,
		namespace,
		h.config.OCIBucketName,
		objectName,
		h.config.OCIImageOS)
	if err != nil {
		return fmt.Errorf("failed to import custom image: %w", err)
	}

	h.logger.Successf("Custom image imported: %s", imageID)
	h.logger.Info("Note: Image import will continue in the background")

	// Store the image ID for use in Terraform template generation
	h.importedImageID = imageID

	return nil
}

func (h *AzureToOCIHandler) exportDataDisks(ctx context.Context) error {
	h.logger.Step(7, "Exporting Data Disks")

	// Create export directory
	exportDir := fmt.Sprintf("./%s-data-disk-exports", common.SanitizeName(h.config.AzureComputeName))
	if err := common.EnsureDir(exportDir); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}

	h.logger.Infof("Export directory: %s", exportDir)

	// Get data disk names using Azure SDK
	h.logger.Info("Getting data disk names...")
	diskNames, err := h.azureProvider.GetComputeDataDiskNames(ctx, h.config.AzureResourceGroup, h.config.AzureComputeName)
	if err != nil {
		return fmt.Errorf("failed to get data disk names: %w", err)
	}

	if len(diskNames) == 0 {
		h.logger.Info("No data disks found for Compute instance")
		return nil
	}

	h.logger.Infof("Found %d data disk(s) to export", len(diskNames))

	// Export each data disk
	for _, diskName := range diskNames {
		h.logger.Infof("Exporting data disk: %s", diskName)
		_, err := h.azureProvider.ExportAzureDisk(ctx, diskName, h.config.AzureResourceGroup, exportDir)
		if err != nil {
			h.logger.Warning(fmt.Sprintf("Failed to export data disk %s: %v", diskName, err))
			continue
		}
		h.logger.Successf("✓ Exported: %s", diskName)
	}

	h.logger.Success("Data disks exported")
	return nil
}

func (h *AzureToOCIHandler) importDataDisks(ctx context.Context) error {
	h.logger.Step(8, "Importing Data Disks")

	// Initialize tracking arrays
	h.dataDiskSnapshotIDs = []string{}
	h.dataDiskSnapshotNames = []string{}

	// Find data disk VHD files
	exportDir := fmt.Sprintf("./%s-data-disk-exports", common.SanitizeName(h.config.AzureComputeName))

	// Check if export directory exists
	if _, err := os.Stat(exportDir); os.IsNotExist(err) {
		h.logger.Info("No data disk export directory found - skipping data disk import")
		return nil
	}

	// Find VHD files in export directory
	vhdFiles, err := filepath.Glob(filepath.Join(exportDir, "*.vhd"))
	if err != nil {
		return fmt.Errorf("failed to find VHD files: %w", err)
	}

	if len(vhdFiles) == 0 {
		h.logger.Info("No data disk VHD files found - skipping data disk import")
		return nil
	}

	h.logger.Infof("Found %d data disk(s) to import", len(vhdFiles))

	// Cleanup previous mounts if any
	common.CleanupNBDMount("/dev/nbd0", "")

	// Load NBD kernel module
	h.logger.Info("Loading NBD kernel module...")
	if err := common.LoadNBDModule(); err != nil {
		return fmt.Errorf("failed to load NBD module: %w", err)
	}

	// Get local instance information
	h.logger.Info("Retrieving local instance information...")
	localInstanceID, err := h.ociProvider.GetLocalInstanceID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get local instance ID: %w", err)
	}

	localAvailabilityDomain, err := h.ociProvider.GetLocalAvailabilityDomain(ctx, localInstanceID)
	if err != nil {
		return fmt.Errorf("failed to get availability domain: %w", err)
	}

	h.logger.Infof("Local instance: %s", localInstanceID)
	h.logger.Infof("Availability domain: %s", localAvailabilityDomain)

	// Track created volumes for cleanup
	createdVolumes := []string{}
	failedCount := 0

	// Process each VHD file
	for _, vhdFile := range vhdFiles {
		h.logger.Info("=========================================")
		h.logger.Infof("Processing: %s", filepath.Base(vhdFile))
		h.logger.Info("=========================================")

		nbdDevice := "/dev/nbd0"
		mountDir := ""

		// Mount the VHD file using NBD
		h.logger.Infof("Connecting VHD image to %s...", nbdDevice)
		if err := common.ConnectVHDToNBD(vhdFile, nbdDevice); err != nil {
			h.logger.Warning(fmt.Sprintf("Failed to connect VHD to NBD: %v", err))
			failedCount++
			continue
		}

		// List partitions for debugging
		h.logger.Infof("Detecting partitions on %s...", nbdDevice)

		// Find a mountable partition
		targetPartition, err := common.FindMountablePartition(nbdDevice)
		if err != nil {
			h.logger.Warning(fmt.Sprintf("Could not find mountable partition: %v", err))
			common.CleanupNBDMount(nbdDevice, "")
			failedCount++
			continue
		}

		// Create temporary mount directory
		mountDir, err = os.MkdirTemp("", "kopru-mount-*")
		if err != nil {
			h.logger.Warning(fmt.Sprintf("Failed to create mount directory: %v", err))
			common.CleanupNBDMount(nbdDevice, "")
			failedCount++
			continue
		}

		// Mount the partition
		h.logger.Infof("Mounting %s to %s...", targetPartition, mountDir)
		if err := common.MountPartition(targetPartition, mountDir); err != nil {
			h.logger.Warning(fmt.Sprintf("Failed to mount partition: %v", err))
			common.CleanupNBDMount(nbdDevice, mountDir)
			failedCount++
			continue
		}
		h.logger.Success("Mounted partition successfully")

		// Calculate disk size
		diskSizeGB, err := common.GetFileSizeGB(vhdFile)
		if err != nil {
			h.logger.Warning(fmt.Sprintf("Failed to get disk size: %v", err))
			common.CleanupNBDMount(nbdDevice, mountDir)
			failedCount++
			continue
		}
		h.logger.Infof("Creating OCI volume of size %d GB...", diskSizeGB)

		// Create volume name based on VHD file name
		baseDiskName := strings.TrimSuffix(filepath.Base(vhdFile), ".vhd")
		volumeName := fmt.Sprintf("imported-%s", baseDiskName)
		h.logger.Infof("Volume name: %s", volumeName)

		// Create OCI block volume
		volumeID, err := h.ociProvider.CreateBlockVolume(ctx, h.config.OCICompartmentID, localAvailabilityDomain, volumeName, diskSizeGB)
		if err != nil {
			h.logger.Warning(fmt.Sprintf("Failed to create OCI volume: %v", err))
			common.CleanupNBDMount(nbdDevice, mountDir)
			failedCount++
			continue
		}
		h.logger.Successf("Created volume: %s", volumeID)
		createdVolumes = append(createdVolumes, volumeID)

		// Get block device list before attachment
		beforeDevices, err := common.ListBlockDevices()
		if err != nil {
			h.logger.Warning(fmt.Sprintf("Failed to list block devices: %v", err))
			common.CleanupNBDMount(nbdDevice, mountDir)
			failedCount++
			continue
		}

		// Attach the volume to this instance
		h.logger.Info("Attaching volume to local instance...")
		attachmentID, err := h.ociProvider.AttachVolume(ctx, localInstanceID, volumeID)
		if err != nil {
			h.logger.Warning(fmt.Sprintf("Failed to attach volume: %v", err))
			common.CleanupNBDMount(nbdDevice, mountDir)
			failedCount++
			continue
		}
		h.logger.Infof("Volume attached (attachment: %s)", attachmentID)

		// Detect the newly attached device
		attachedDevice, err := common.DetectNewBlockDevice(beforeDevices)
		if err != nil {
			h.logger.Warning(fmt.Sprintf("Could not detect attached device: %v", err))
			h.ociProvider.DetachVolume(ctx, attachmentID)
			common.CleanupNBDMount(nbdDevice, mountDir)
			failedCount++
			continue
		}
		h.logger.Infof("Attached device: %s", attachedDevice)

		// Copy data from VHD partition to the attached volume
		h.logger.Infof("Copying data from %s to %s...", targetPartition, attachedDevice)
		h.logger.Info("  This may take several minutes depending on disk size...")
		if err := common.CopyDataWithDD(targetPartition, attachedDevice); err != nil {
			h.logger.Warning(fmt.Sprintf("Failed to copy data: %v", err))
			h.ociProvider.DetachVolume(ctx, attachmentID)
			common.CleanupNBDMount(nbdDevice, mountDir)
			failedCount++
			continue
		}
		h.logger.Success("Data copy completed")

		// Unmount and cleanup NBD
		common.CleanupNBDMount(nbdDevice, mountDir)

		// Detach the volume
		h.logger.Info("Detaching volume...")
		if err := h.ociProvider.DetachVolume(ctx, attachmentID); err != nil {
			h.logger.Warning(fmt.Sprintf("Failed to detach volume: %v", err))
		} else {
			h.logger.Info("Volume detached")
		}

		// Create snapshot for this volume
		snapshotName := fmt.Sprintf("%s-snapshot", baseDiskName)
		h.logger.Infof("Creating snapshot: %s...", snapshotName)
		snapshotID, err := h.ociProvider.CreateVolumeSnapshot(ctx, volumeID, snapshotName)
		if err != nil {
			h.logger.Warning(fmt.Sprintf("Failed to create snapshot: %v", err))
			failedCount++
			continue
		}

		h.logger.Successf("Created snapshot: %s", snapshotID)
		h.dataDiskSnapshotIDs = append(h.dataDiskSnapshotIDs, snapshotID)
		h.dataDiskSnapshotNames = append(h.dataDiskSnapshotNames, snapshotName)

		h.logger.Successf("Processed: %s", filepath.Base(vhdFile))
	}

	// Delete all created volumes since only snapshots are needed
	h.logger.Info("Cleaning up temporary block volumes...")
	for _, volumeID := range createdVolumes {
		h.logger.Infof("Deleting volume %s...", volumeID)
		if err := h.ociProvider.DeleteVolume(ctx, volumeID); err != nil {
			h.logger.Warning(fmt.Sprintf("Failed to delete volume %s: %v", volumeID, err))
		} else {
			h.logger.Success("Volume deleted")
		}
	}

	h.logger.Info("=========================================")
	h.logger.Success("Data disk import completed")
	h.logger.Infof("  Snapshots created: %d", len(h.dataDiskSnapshotIDs))
	h.logger.Infof("  Failed: %d", failedCount)
	if len(h.dataDiskSnapshotIDs) > 0 {
		h.logger.Infof("  Snapshot OCIDs: %v", h.dataDiskSnapshotIDs)
		h.logger.Infof("  Snapshot Names: %v", h.dataDiskSnapshotNames)
	}
	h.logger.Info("=========================================")

	return nil
}

func (h *AzureToOCIHandler) generateTemplate(ctx context.Context) error {
	h.logger.Step(9, "Generating Template")

	// Create template generator
	tfGen := template.NewOCIGenerator(h.config, h.logger, h.importedImageID, h.dataDiskSnapshotIDs, h.dataDiskSnapshotNames)

	// Generate template
	if err := tfGen.GenerateTemplate(); err != nil {
		return err
	}

	return nil
}

func (h *AzureToOCIHandler) waitForImageAvailable(ctx context.Context) error {
	h.logger.Step(10, "Verifying Image Import Status")

	if h.importedImageID == "" {
		return fmt.Errorf("no imported image ID available")
	}

	// Call the OCI provider's WaitForImageAvailable function
	if err := h.ociProvider.WaitForImageAvailable(ctx, h.importedImageID); err != nil {
		return fmt.Errorf("failed to wait for image availability: %w", err)
	}

	return nil
}

func (h *AzureToOCIHandler) deployTemplate(ctx context.Context) error {
	h.logger.Step(11, "Deploying the template")

	// Create template generator
	tfGen := template.NewOCIGenerator(h.config, h.logger, h.importedImageID, h.dataDiskSnapshotIDs, h.dataDiskSnapshotNames)

	// Deploy the template
	if err := tfGen.DeployTemplate(); err != nil {
		return err
	}

	return nil
}

func (h *AzureToOCIHandler) verifyWorkflow(ctx context.Context) error {
	h.logger.Step(12, "Verifying Workflow")

	// Verify exported files exist
	exportDir := fmt.Sprintf("./%s-os-disk-export", common.SanitizeName(h.config.AzureComputeName))

	if !h.config.SkipExport {
		vhdFile, err := common.FindVHDFile(exportDir)
		if err == nil {
			h.logger.Successf("✓ VHD file exists: %s", filepath.Base(vhdFile))
		}
	}

	if !h.config.SkipConvert {
		qcow2File, err := common.FindQCOW2File(exportDir)
		if err == nil {
			h.logger.Successf("✓ QCOW2 file exists: %s", filepath.Base(qcow2File))
		}
	}

	if !h.config.SkipTemplate {
		if _, err := os.Stat(h.config.TemplateOutputDir); err == nil {
			h.logger.Successf("✓ Template files exist in: %s", h.config.TemplateOutputDir)
		}
	}

	h.logger.Success("Workflow verification complete")
	h.logger.Info("=========================================")
	h.logger.Info("Next Steps:")
	if !h.config.SkipTemplateDeploy {
		h.logger.Info("1. Check the OCI console for the deployed instance")
		h.logger.Info("2. Verify the instance is running as expected")
	} else {
		h.logger.Infof("1. Navigate to: %s", h.config.TemplateOutputDir)
		h.logger.Info("2. Run: tofu init && tofu apply")
		h.logger.Info("3. Check the OCI console for the deployed instance")
	}
	h.logger.Info("=========================================")

	return nil
}

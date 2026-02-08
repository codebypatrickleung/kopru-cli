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
	"github.com/codebypatrickleung/kopru-cli/internal/template"
)

// AzureToOCIHandler implements the workflow for migrating Compute instances from Azure to OCI.
type AzureToOCIHandler struct {
	config                *config.Config
	logger                *logger.Logger
	azureProvider         *azure.Provider
	ociProvider           *oci.Provider
	dataDiskSnapshotIDs   []string
	dataDiskSnapshotNames []string
	azureOSDiskSizeGB     int64
	azureVMCPUs           int32
	azureVMMemoryGB       int32
	azureVMArchitecture   string
}

func NewAzureToOCIHandler() *AzureToOCIHandler      { return &AzureToOCIHandler{} }
func (h *AzureToOCIHandler) Name() string           { return "Azure to OCI Migration" }
func (h *AzureToOCIHandler) SourcePlatform() string { return "azure" }
func (h *AzureToOCIHandler) TargetPlatform() string { return "oci" }

func (h *AzureToOCIHandler) Initialize(cfg *config.Config, log *logger.Logger) error {
	h.config, h.logger = cfg, log
	var err error
	if h.azureProvider, err = azure.NewProvider(cfg.AzureSubscriptionID, log); err != nil {
		return fmt.Errorf("failed to initialize Azure provider: %w", err)
	}
	if h.ociProvider, err = oci.NewProvider(cfg.OCIRegion, log); err != nil {
		return fmt.Errorf("failed to initialize OCI provider: %w", err)
	}
	return nil
}

func (h *AzureToOCIHandler) Execute(ctx context.Context) error {
	h.logger.Info("=========================================")
	h.logger.Infof("Executing: %s", h.Name())
	h.logger.Info("=========================================")

	steps := []struct {
		skip    bool
		skipMsg string
		errMsg  string
		fn      func(context.Context) error
	}{
		{h.config.SkipPrereq, "Skipping prerequisite checks (SKIP_PREREQ=true)", "prerequisite checks failed", h.runPrerequisites},
		{h.config.SkipExport, "Skipping OS disk export (SKIP_OS_EXPORT=true)", "OS disk export failed", h.exportOSDisk},
		{h.config.SkipConvert, "Skipping disk conversion (SKIP_OS_CONVERT=true)", "disk conversion failed", h.convertDisk},
		{h.config.SkipConfigure, "Skipping image configuration (SKIP_OS_CONFIGURE=true)", "image configuration failed", h.configureImage},
		{h.config.SkipUpload, "Skipping image upload (SKIP_OS_UPLOAD=true)", "image upload failed", h.uploadImage},
		{h.config.SkipDDExport, "Skipping data disk export (SKIP_DD_EXPORT=true)", "data disk export failed", h.exportDataDisks},
		{h.config.SkipDDImport, "Skipping data disk import (SKIP_DD_IMPORT=true)", "data disk import failed", h.importDataDisks},
		{h.config.SkipTemplate, "Skipping template generation (SKIP_TEMPLATE=true)", "template generation failed", h.generateTemplate},
	}
	for _, step := range steps {
		if step.skip {
			h.logger.Warning(step.skipMsg)
			continue
		}
		if err := step.fn(ctx); err != nil {
			return fmt.Errorf("%s: %w", step.errMsg, err)
		}
	}

	if !h.config.SkipTemplateDeploy {
		if err := h.deployTemplate(ctx); err != nil {
			return fmt.Errorf("template deployment failed: %w", err)
		}
	} else {
		h.logger.Warning("Skipping template deployment (SKIP_TEMPLATE_DEPLOY=true)")
		h.logger.Infof("To deploy manually, run: cd %s && tofu init && tofu apply", h.config.TemplateOutputDir)
	}

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

func (h *AzureToOCIHandler) runPrerequisites(ctx context.Context) error {
	h.logger.Step(1, "Reviewing Migration Configuration")
	h.logger.Infof("Azure Resource Group: %s", h.config.AzureResourceGroup)
	h.logger.Infof("Azure Compute Name: %s", h.config.AzureComputeName)
	h.logger.Infof("OCI Compartment ID: %s", h.config.OCICompartmentID)
	h.logger.Infof("OCI Subnet ID: %s", h.config.OCISubnetID)
	h.logger.Infof("OCI Region: %s", h.config.OCIRegion)
	h.logger.Infof("OCI Bucket Name: %s", h.config.OCIBucketName)
	h.logger.Infof("OCI Image Name: %s", h.config.OCIImageName)
	h.logger.Infof("OCI Image OS: %s", h.config.OCIImageOS)
	h.logger.Infof("OCI Image OS Version: %s", h.config.OCIImageOSVersion)
	h.logger.Infof("OCI Image UEFI Enabled: %t", h.config.OCIImageEnableUEFI)
	h.logger.Infof("Template Output Dir: %s", h.config.TemplateOutputDir)
	h.logger.Infof("SSH Key File Path: %s", h.config.SSHKeyFilePath)
	h.logger.Step(2, "Running Prerequisite Checks")
	for _, tool := range []string{"qemu-img", "virt-customize"} {
		if err := common.CheckCommand(tool); err != nil {
			return fmt.Errorf("required tool missing: %w", err)
		}
		h.logger.Successf("✓ %s is installed", tool)
	}
	availableBytes, err := common.GetAvailableDiskSpace(".", common.MinDiskSpaceGB)
	if err != nil {
		h.logger.Warning(fmt.Sprintf("Disk space check: %v", err))
	} else {
		h.logger.Successf("✓ Available disk space: %d GB", availableBytes/(1024*1024*1024))
	}
	h.logger.Warning("Ignore this warning if your available disk space exceeds 2x the VM disks plus 50 GB.")
	if err := h.azureProvider.CheckComputeExists(ctx, h.config.AzureResourceGroup, h.config.AzureComputeName); err != nil {
		return fmt.Errorf("azure Compute instance check failed: %w", err)
	}
	h.logger.Successf("✓ Azure Compute instance '%s' is accessible", h.config.AzureComputeName)
	osType, err := h.azureProvider.GetComputeOSType(ctx, h.config.AzureResourceGroup, h.config.AzureComputeName)
	if err != nil {
		return fmt.Errorf("failed to get Compute instance OS type: %w", err)
	}
	h.logger.Successf("✓ Compute instance OS type: %s", osType)
	cpus, memoryGB, err := h.azureProvider.GetComputeCPUAndMemory(ctx, h.config.AzureResourceGroup, h.config.AzureComputeName)
	if err != nil {
		h.logger.Warning(fmt.Sprintf("Failed to get VM CPU/memory configuration: %v", err))
		h.logger.Warning("Will use default configuration (1 OCPU, 12 GB) for OCI instance")
		h.azureVMCPUs = 0
		h.azureVMMemoryGB = 0
	} else {
		h.azureVMCPUs = cpus
		h.azureVMMemoryGB = memoryGB
		h.logger.Successf("✓ Source VM configuration: %d vCPUs, %d GB memory", cpus, memoryGB)
	}
	architecture, err := h.azureProvider.GetComputeArchitecture(ctx, h.config.AzureResourceGroup, h.config.AzureComputeName)
	if err != nil {
		h.logger.Warning(fmt.Sprintf("Failed to get VM architecture: %v", err))
		h.logger.Warning("Will assume x86_64 architecture for OCI instance")
		h.azureVMArchitecture = "x86_64"
	} else {
		h.azureVMArchitecture = architecture
		h.logger.Successf("✓ Source VM CPU architecture: %s", architecture)
	}
	if h.config.OCIImageOS == "" {
		return fmt.Errorf("operating system (OCI_IMAGE_OS) is required when migrating a Compute instance. Allowed values: 'Oracle Linux', 'AlmaLinux', 'CentOS', 'Debian', 'RHEL', 'Rocky Linux', 'SUSE', 'Ubuntu', 'Windows'")
	}
	allowedOS := map[string]struct{}{
		"Oracle Linux": {}, "AlmaLinux": {}, "CentOS": {}, "Debian": {}, "RHEL": {},
		"Rocky Linux": {}, "SUSE": {}, "Ubuntu": {}, "Windows": {}, "Generic Linux": {},
	}
	if _, ok := allowedOS[h.config.OCIImageOS]; !ok {
		return fmt.Errorf("invalid OCI_IMAGE_OS: '%s'. Allowed values: 'Oracle Linux', 'AlmaLinux', 'CentOS', 'Debian', 'RHEL', 'Rocky Linux', 'SUSE', 'Ubuntu', 'Windows'", h.config.OCIImageOS)
	}
	if strings.ToLower(osType) == "windows" && strings.ToLower(h.config.OCIImageOS) != "windows" {
		return fmt.Errorf("detected OS type is 'Windows', but OCI_IMAGE_OS is set to '%s'. Please set OCI_IMAGE_OS to 'Windows'", h.config.OCIImageOS)
	}
	h.logger.Successf("✓ Detected OS type '%s' matches OCI_IMAGE_OS '%s'", osType, h.config.OCIImageOS)
	h.logger.Successf("✓ Operating system configured for OCI: %s", h.config.OCIImageOS)
	if common.IsWindowsOS(h.config.OCIImageOS) && h.config.OCIImageOSVersion == "" {
		return fmt.Errorf("operating system version (OCI_IMAGE_OS_VERSION) is required when migrating a Windows instance")
	}
	h.logger.Successf("✓ Compute instance OS version: %s", h.config.OCIImageOSVersion)
	if h.config.OCIRegion == "" {
		return fmt.Errorf("OCI region (OCI_REGION) is required")
	}
	h.logger.Successf("✓ OCI region configured: %s", h.config.OCIRegion)
	isStopped, err := h.azureProvider.CheckComputeIsStopped(ctx, h.config.AzureResourceGroup, h.config.AzureComputeName)
	if err != nil {
		return fmt.Errorf("failed to check Compute instance state: %w", err)
	}
	if !isStopped {
		h.logger.Warning("Compute instance is running - it's recommended to stop the instance before export to ensure data consistency")
	} else {
		h.logger.Success("✓ Compute instance is stopped")
	}
	if err := h.ociProvider.CheckCompartmentExists(ctx, h.config.OCICompartmentID); err != nil {
		return fmt.Errorf("OCI compartment check failed: %w", err)
	}
	h.logger.Success("✓ OCI compartment is accessible")
	if err := h.ociProvider.CheckSubnetExists(ctx, h.config.OCISubnetID); err != nil {
		return fmt.Errorf("OCI subnet check failed: %w", err)
	}
	h.logger.Success("✓ OCI subnet is accessible")
	namespace, err := h.ociProvider.GetNamespace(ctx)
	if err != nil {
		return fmt.Errorf("failed to get OCI namespace: %w", err)
	}
	h.logger.Successf("✓ OCI namespace retrieved: %s", namespace)
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
	h.logger.Step(3, "Exporting OS Disk")
	exportDir := fmt.Sprintf("./%s-os-disk-export", common.SanitizeName(h.config.AzureComputeName))
	if err := common.EnsureDir(exportDir); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}
	h.logger.Infof("Export directory: %s", exportDir)
	osDiskName, err := h.azureProvider.GetComputeOSDiskName(ctx, h.config.AzureResourceGroup, h.config.AzureComputeName)
	if err != nil {
		return fmt.Errorf("failed to get OS disk name: %w", err)
	}
	h.logger.Infof("OS disk name: %s", osDiskName)
	vhdFile, err := h.azureProvider.ExportAzureDisk(ctx, osDiskName, h.config.AzureResourceGroup, exportDir)
	if err != nil {
		return fmt.Errorf("failed to export OS disk: %w", err)
	}
	h.logger.Successf("OS disk exported to: %s", vhdFile)
	return nil
}

func (h *AzureToOCIHandler) convertDisk(ctx context.Context) error {
	h.logger.Step(4, "Converting VHD to QCOW2")
	exportDir := fmt.Sprintf("./%s-os-disk-export", common.SanitizeName(h.config.AzureComputeName))
	vhdFile, err := common.FindDiskFile(exportDir, ".vhd")
	if err != nil {
		return fmt.Errorf("failed to find VHD file: %w", err)
	}
	h.logger.Infof("Converting VHD file: %s", vhdFile)
	qcow2File := strings.TrimSuffix(vhdFile, ".vhd") + ".qcow2"
	h.logger.Info("Running qemu-img convert (this may take a while)...")
	if err := common.ConvertVHDToQCOW2(vhdFile, qcow2File); err != nil {
		return err
	}
	h.logger.Successf("Disk converted to QCOW2: %s", qcow2File)
	return nil
}

func (h *AzureToOCIHandler) configureImage(ctx context.Context) error {
	h.logger.Step(5, "Configuring Image for OCI")
	exportDir := fmt.Sprintf("./%s-os-disk-export", common.SanitizeName(h.config.AzureComputeName))
	qcow2File, err := common.FindDiskFile(exportDir, ".qcow2")
	if err != nil {
		return fmt.Errorf("failed to find QCOW2 file: %w", err)
	}
	h.logger.Infof("Configuring QCOW2 file: %s", qcow2File)
	osType := h.config.OCIImageOS
	if common.IsLinuxOS(osType) {
		h.logger.Info("Applying OS configurations ...")
		if err := common.ExecuteOSConfigScript(qcow2File, osType, h.SourcePlatform(), h.logger); err != nil {
			return fmt.Errorf("failed to execute OS configuration script: %w", err)
		}
		h.logger.Success("Image configurations complete")
	} else {
		h.logger.Infof("Skipping image configuration for %s OS", osType)
	}
	return nil
}

func (h *AzureToOCIHandler) uploadImage(ctx context.Context) error {
	h.logger.Step(6, "Uploading Image to OCI")
	exportDir := fmt.Sprintf("./%s-os-disk-export", common.SanitizeName(h.config.AzureComputeName))
	qcow2File, err := common.FindDiskFile(exportDir, ".qcow2")
	if err != nil {
		return fmt.Errorf("failed to find QCOW2 file: %w", err)
	}
	namespace, err := h.ociProvider.GetNamespace(ctx)
	if err != nil {
		return fmt.Errorf("failed to get namespace: %w", err)
	}
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
	objectName := filepath.Base(qcow2File)
	h.logger.Infof("Uploading %s to bucket %s (this may take a while)...", objectName, h.config.OCIBucketName)
	if err := h.ociProvider.UploadToObjectStorage(ctx, namespace, h.config.OCIBucketName, objectName, qcow2File); err != nil {
		return fmt.Errorf("failed to upload to Object Storage: %w", err)
	}
	h.logger.Success("Image uploaded to OCI")
	return nil
}

func (h *AzureToOCIHandler) exportDataDisks(ctx context.Context) error {
	h.logger.Step(7, "Exporting Data Disks")
	exportDir := fmt.Sprintf("./%s-data-disk-exports", common.SanitizeName(h.config.AzureComputeName))
	if err := common.EnsureDir(exportDir); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}
	h.logger.Infof("Export directory: %s", exportDir)
	diskNames, err := h.azureProvider.GetComputeDataDiskNames(ctx, h.config.AzureResourceGroup, h.config.AzureComputeName)
	if err != nil {
		return fmt.Errorf("failed to get data disk names: %w", err)
	}
	if len(diskNames) == 0 {
		h.logger.Info("No data disks found for Compute instance")
		return nil
	}
	h.logger.Infof("Found %d data disk(s) to export", len(diskNames))
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
	h.dataDiskSnapshotIDs, h.dataDiskSnapshotNames = []string{}, []string{}
	exportDir := fmt.Sprintf("./%s-data-disk-exports", common.SanitizeName(h.config.AzureComputeName))
	if _, err := os.Stat(exportDir); os.IsNotExist(err) {
		h.logger.Info("No data disk export directory found - skipping data disk import")
		return nil
	}
	vhdFiles, err := filepath.Glob(filepath.Join(exportDir, "*.vhd"))
	if err != nil {
		return fmt.Errorf("failed to find VHD files: %w", err)
	}
	if len(vhdFiles) == 0 {
		h.logger.Info("No data disk VHD files found - skipping data disk import")
		return nil
	}
	if common.IsWindowsOS(h.config.OCIImageOS) && h.config.OCIImageOSVersion == "" {
		return fmt.Errorf("operating system version (OCI_IMAGE_OS_VERSION) is required when migrating a Windows instance")
	}
	h.logger.Infof("Compute instance OS type: %s", h.config.OCIImageOS)
	h.logger.Infof("Compute instance OS version: %s", h.config.OCIImageOSVersion)
	h.logger.Infof("Found %d data disk(s) to import", len(vhdFiles))
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

	var (
		createdVolumes []string
		snapshotIDs    []string
		snapshotNames  []string
		failedCount    int
	)

	for _, vhdFile := range vhdFiles {
		h.logger.Info("=========================================")
		h.logger.Infof("Processing: %s", filepath.Base(vhdFile))
		h.logger.Info("=========================================")

		baseDiskName := strings.TrimSuffix(filepath.Base(vhdFile), ".vhd")
		rawFile := strings.TrimSuffix(vhdFile, ".vhd") + ".raw"
		h.logger.Infof("Converting VHD to RAW format: %s", filepath.Base(rawFile))
		if err := common.ConvertVHDToRAW(vhdFile, rawFile); err != nil {
			h.logger.Warning(fmt.Sprintf("Failed to convert VHD to RAW: %v", err))
			failedCount++
			continue
		}
		h.logger.Success("VHD converted to RAW format")

		diskSizeGB, err := common.GetFileSizeGB(rawFile)
		if err != nil {
			h.logger.Warning(fmt.Sprintf("Failed to get disk size: %v", err))
			failedCount++
			continue
		}

		h.logger.Infof("Creating OCI volume of size %d GB...", diskSizeGB)
		volumeName := fmt.Sprintf("bv-%s", baseDiskName)
		h.logger.Infof("Volume name: %s", volumeName)

		volumeID, err := h.ociProvider.CreateBlockVolume(ctx, h.config.OCICompartmentID, localAvailabilityDomain, volumeName, diskSizeGB)
		if err != nil {
			h.logger.Warning(fmt.Sprintf("Failed to create OCI volume: %v", err))
			failedCount++
			continue
		}
		h.logger.Successf("Created volume: %s", volumeID)

		createdVolumes = append(createdVolumes, volumeID)

		beforeDevices, err := common.ListBlockDevices()
		if err != nil {
			h.logger.Warning(fmt.Sprintf("Failed to list block devices: %v", err))
			failedCount++
			continue
		}

		h.logger.Info("Attaching volume to local instance...")
		attachmentID, err := h.ociProvider.AttachVolume(ctx, localInstanceID, volumeID)
		if err != nil {
			h.logger.Warning(fmt.Sprintf("Failed to attach volume: %v", err))
			failedCount++
			continue
		}
		h.logger.Infof("Volume attached (attachment: %s)", attachmentID)

		attachedDevice, err := common.DetectNewBlockDevice(beforeDevices)
		if err != nil {
			h.logger.Warning(fmt.Sprintf("Could not detect attached device: %v", err))
			if detachErr := h.ociProvider.DetachVolume(ctx, attachmentID); detachErr != nil {
				h.logger.Warning(fmt.Sprintf("Failed to detach volume during cleanup: %v", detachErr))
			}
			failedCount++
			continue
		}
		h.logger.Infof("Attached device: %s", attachedDevice)

		h.logger.Infof("Copying data from RAW file to %s...", attachedDevice)
		h.logger.Info("  This may take a while depending on disk size...")
		if err := common.CopyDataWithDD(rawFile, attachedDevice); err != nil {
			h.logger.Warning(fmt.Sprintf("Failed to copy data: %v", err))
			if detachErr := h.ociProvider.DetachVolume(ctx, attachmentID); detachErr != nil {
				h.logger.Warning(fmt.Sprintf("Failed to detach volume during cleanup: %v", detachErr))
			}
			failedCount++
			continue
		}
		h.logger.Success("Data copy completed")

		h.logger.Info("Detaching volume...")
		if err := h.ociProvider.DetachVolume(ctx, attachmentID); err != nil {
			h.logger.Warning(fmt.Sprintf("Failed to detach volume: %v", err))
		} else {
			h.logger.Info("Volume detached")
		}

		snapshotName := fmt.Sprintf("ss-%s", baseDiskName)
		h.logger.Infof("Creating snapshot: %s...", snapshotName)
		snapshotID, err := h.ociProvider.CreateVolumeSnapshot(ctx, volumeID, snapshotName)
		if err != nil {
			h.logger.Warning(fmt.Sprintf("Failed to create snapshot: %v", err))
			failedCount++
			continue
		}
		h.logger.Successf("Created snapshot: %s", snapshotID)

		snapshotIDs = append(snapshotIDs, snapshotID)
		snapshotNames = append(snapshotNames, snapshotName)

		h.logger.Successf("Processed: %s", filepath.Base(vhdFile))
	}

	h.dataDiskSnapshotIDs = snapshotIDs
	h.dataDiskSnapshotNames = snapshotNames

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

// getImageImportDetails retrieves namespace and object name for image import
func (h *AzureToOCIHandler) getImageImportDetails(ctx context.Context) (namespace, objectName string, err error) {
	exportDir := fmt.Sprintf("./%s-os-disk-export", common.SanitizeName(h.config.AzureComputeName))
	qcow2File, err := common.FindDiskFile(exportDir, ".qcow2")
	if err != nil {
		return "", "", fmt.Errorf("failed to find QCOW2 file: %w", err)
	}
	objectName = filepath.Base(qcow2File)
	namespace, err = h.ociProvider.GetNamespace(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to get namespace: %w", err)
	}
	return namespace, objectName, nil
}

func (h *AzureToOCIHandler) generateTemplate(ctx context.Context) error {
	h.logger.Step(9, "Generating Template")
	if h.azureOSDiskSizeGB == 0 {
		h.logger.Info("Reading OS disk size from QCOW2 file...")
		exportDir := fmt.Sprintf("./%s-os-disk-export", common.SanitizeName(h.config.AzureComputeName))
		qcow2File, err := common.FindDiskFile(exportDir, ".qcow2")
		if err != nil {
			return fmt.Errorf("failed to find QCOW2 file: %w", err)
		}
		osDiskSizeGB, err := common.GetComputeOSDiskSizeGB(qcow2File)
		if err != nil {
			return fmt.Errorf("failed to get OS disk size from QCOW2: %w", err)
		}
		h.azureOSDiskSizeGB = osDiskSizeGB
		h.logger.Successf("✓ OS disk size from QCOW2: %d GB", osDiskSizeGB)
		if h.azureOSDiskSizeGB < common.OCIMinVolumeSizeGB {
			h.logger.Infof("OS disk size (%d GB) is less than OCI minimum (%d GB)", h.azureOSDiskSizeGB, common.OCIMinVolumeSizeGB)
			h.logger.Infof("Boot volume will be created with minimum size of %d GB", common.OCIMinVolumeSizeGB)
		}
	}
	namespace, objectName, err := h.getImageImportDetails(ctx)
	if err != nil {
		return err
	}
	tfGen := template.NewOCIGenerator(
		h.config, h.logger, namespace, objectName,
		h.dataDiskSnapshotIDs, h.dataDiskSnapshotNames,
		h.azureOSDiskSizeGB, h.azureVMCPUs, h.azureVMMemoryGB, h.azureVMArchitecture,
	)
	return tfGen.GenerateTemplate()
}

func (h *AzureToOCIHandler) deployTemplate(ctx context.Context) error {
	h.logger.Step(10, "Deploying the template")
	namespace, objectName, err := h.getImageImportDetails(ctx)
	if err != nil {
		return err
	}
	tfGen := template.NewOCIGenerator(
		h.config, h.logger, namespace, objectName,
		h.dataDiskSnapshotIDs, h.dataDiskSnapshotNames,
		h.azureOSDiskSizeGB, h.azureVMCPUs, h.azureVMMemoryGB, h.azureVMArchitecture,
	)
	return tfGen.DeployTemplate()
}

func (h *AzureToOCIHandler) verifyWorkflow(ctx context.Context) error {
	h.logger.Step(11, "Verifying Workflow")
	exportDir := fmt.Sprintf("./%s-os-disk-export", common.SanitizeName(h.config.AzureComputeName))
	if !h.config.SkipExport {
		if vhdFile, err := common.FindDiskFile(exportDir, ".vhd"); err == nil {
			h.logger.Successf("✓ VHD file exists: %s", filepath.Base(vhdFile))
		}
	}
	if !h.config.SkipConvert {
		if qcow2File, err := common.FindDiskFile(exportDir, ".qcow2"); err == nil {
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

// Package workflow provides workflow handlers for specific migration paths.
package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/codebypatrickleung/kopru-cli/internal/cloud/azure"
	"github.com/codebypatrickleung/kopru-cli/internal/cloud/oci"
	"github.com/codebypatrickleung/kopru-cli/internal/common"
	"github.com/codebypatrickleung/kopru-cli/internal/config"
	"github.com/codebypatrickleung/kopru-cli/internal/logger"
	"github.com/codebypatrickleung/kopru-cli/internal/template"
	"github.com/oracle/oci-go-sdk/v65/core"
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
	osExportDir           string
	dataExportDir         string
	templateOutputDir     string
	importedImageID       string
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

	// Set export and template output directories based on Azure compute name
	sanitizedName := common.SanitizeName(cfg.AzureComputeName)
	h.osExportDir = fmt.Sprintf("./%s-os-disk-export", sanitizedName)
	h.dataExportDir = fmt.Sprintf("./%s-data-disk-exports", sanitizedName)
	h.templateOutputDir = fmt.Sprintf("./%s-template-output", sanitizedName)

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
		{h.config.SkipExport, "Skipping OS disk export (SKIP_OS_EXPORT=true)", "OS disk export failed", h.exportOSDisk},
	}
	
	// Run prerequisite checks
	if err := h.runPrerequisites(ctx); err != nil {
		return fmt.Errorf("prerequisite checks failed: %w", err)
	}
	
	// Run steps with skip logic
	for _, step := range steps {
		if step.skip {
			h.logger.Warning(step.skipMsg)
			continue
		}
		if err := step.fn(ctx); err != nil {
			return fmt.Errorf("%s: %w", step.errMsg, err)
		}
	}
	
	if err := h.convertDisk(ctx); err != nil {
		return fmt.Errorf("disk conversion failed: %w", err)
	}
	if err := h.configureImage(ctx); err != nil {
		return fmt.Errorf("image configuration failed: %w", err)
	}
	if err := h.uploadImage(ctx); err != nil {
		return fmt.Errorf("image upload failed: %w", err)
	}
	if err := h.importOSImage(ctx); err != nil {
		return fmt.Errorf("image import failed: %w", err)
	}
	if err := h.exportDataDisks(ctx); err != nil {
		return fmt.Errorf("data disk export failed: %w", err)
	}
	if err := h.importDataDisks(ctx); err != nil {
		return fmt.Errorf("data disk import failed: %w", err)
	}
	if err := h.generateTemplate(ctx); err != nil {
		return fmt.Errorf("template generation failed: %w", err)
	}

	if !h.config.SkipTemplateDeploy {
		if err := h.deployTemplate(ctx); err != nil {
			return fmt.Errorf("template deployment failed: %w", err)
		}
	} else {
		h.logger.Warning("Skipping template deployment (SKIP_TEMPLATE_DEPLOY=true)")
		h.logger.Infof("To deploy manually, run: cd %s && tofu init && tofu apply", h.templateOutputDir)
	}

	if err := h.verifyWorkflow(ctx); err != nil {
		return fmt.Errorf("workflow verification failed: %w", err)
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
	h.logger.Infof("Template Output Dir: %s", h.templateOutputDir)
	h.logger.Infof("SSH Key File Path: %s", h.config.SSHKeyFilePath)
	h.logger.Infof("Data Disk Parallelism: %d", h.config.DataDiskParallelism)
	h.logger.Step(2, "Running Prerequisite Checks")
	for _, tool := range []string{"qemu-img", "virt-customize"} {
		if err := common.CheckCommand(tool); err != nil {
			return fmt.Errorf("required tool missing: %w", err)
		}
		h.logger.Successf("✓ %s is installed", tool)
	}
	availableBytes, err := common.GetAvailableDiskSpace(".", common.MinDiskSpaceGB)
	if err != nil {
		h.logger.Warningf("Disk space check: %v", err)
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
		h.logger.Warningf("Failed to get VM CPU/memory configuration: %v", err)
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
		h.logger.Warningf("Failed to get VM architecture: %v", err)
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
	if h.config.OCIImageOSVersion == "" {
		return fmt.Errorf("operating system version (OCI_IMAGE_OS_VERSION) is required")
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
	if err := common.EnsureDir(h.osExportDir); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}
	h.logger.Infof("Export directory: %s", h.osExportDir)
	osDiskName, err := h.azureProvider.GetComputeOSDiskName(ctx, h.config.AzureResourceGroup, h.config.AzureComputeName)
	if err != nil {
		return fmt.Errorf("failed to get OS disk name: %w", err)
	}
	h.logger.Infof("OS disk name: %s", osDiskName)
	vhdFile, err := h.azureProvider.ExportAzureDisk(ctx, osDiskName, h.config.AzureResourceGroup, h.osExportDir)
	if err != nil {
		return fmt.Errorf("failed to export OS disk: %w", err)
	}
	h.logger.Successf("OS disk exported to: %s", vhdFile)
	return nil
}

func (h *AzureToOCIHandler) convertDisk(ctx context.Context) error {
	h.logger.Step(4, "Converting VHD to QCOW2")
	vhdFile, err := common.FindDiskFile(h.osExportDir, ".vhd")
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
	qcow2File, err := common.FindDiskFile(h.osExportDir, ".qcow2")
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
	qcow2File, err := common.FindDiskFile(h.osExportDir, ".qcow2")
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

func (h *AzureToOCIHandler) importOSImage(ctx context.Context) error {
	h.logger.Step(7, "Importing OS Image in OCI")
	
	namespace, objectName, err := h.getImageImportDetails(ctx)
	if err != nil {
		return err
	}

	imageName := fmt.Sprintf("%s-imported-image", common.SanitizeName(h.config.AzureComputeName))
	h.logger.Infof("Starting OS image import: %s", imageName)
	h.logger.Info("Image import will run in the background (10-20 minutes)")

	imageID, err := h.ociProvider.ImportImage(
		ctx,
		h.config.OCICompartmentID,
		namespace,
		h.config.OCIBucketName,
		objectName,
		imageName,
		h.config.OCIImageOS,
		h.config.OCIImageOSVersion,
	)
	if err != nil {
		return fmt.Errorf("failed to start image import: %w", err)
	}

	h.importedImageID = imageID
	h.logger.Successf("OS image import started with ID: %s", imageID)
	h.logger.Info("Continuing with data disk operations while image imports in background...")
	
	return nil
}

func (h *AzureToOCIHandler) exportDataDisks(ctx context.Context) error {
	h.logger.Step(8, "Exporting Data Disks")
	if err := common.EnsureDir(h.dataExportDir); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}
	h.logger.Infof("Export directory: %s", h.dataExportDir)
	diskNames, err := h.azureProvider.GetComputeDataDiskNames(ctx, h.config.AzureResourceGroup, h.config.AzureComputeName)
	if err != nil {
		return fmt.Errorf("failed to get data disk names: %w", err)
	}
	if len(diskNames) == 0 {
		h.logger.Info("No data disks found for Compute instance")
		return nil
	}
	h.logger.Infof("Found %d data disk(s) to export", len(diskNames))
	h.logger.Info("Exporting all data disks in parallel...")
	sem := make(chan struct{}, h.config.DataDiskParallelism)
	var wg sync.WaitGroup
	for _, diskName := range diskNames {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() {
				<-sem
				wg.Done()
			}()
			h.logger.Infof("Exporting data disk: %s", diskName)
			if _, err := h.azureProvider.ExportAzureDisk(ctx, diskName, h.config.AzureResourceGroup, h.dataExportDir); err != nil {
				h.logger.Warningf("Failed to export data disk %s: %v", diskName, err)
				return
			}
			h.logger.Successf("✓ Exported: %s", diskName)
		}()
	}
	wg.Wait()
	h.logger.Success("Data disks exported")
	return nil
}

func (h *AzureToOCIHandler) importDataDisks(ctx context.Context) error {
	h.logger.Step(9, "Importing Data Disks")
	h.dataDiskSnapshotIDs, h.dataDiskSnapshotNames = []string{}, []string{}
	if _, err := os.Stat(h.dataExportDir); os.IsNotExist(err) {
		h.logger.Info("No data disk export directory found - skipping data disk import")
		return nil
	}
	vhdFiles, err := filepath.Glob(filepath.Join(h.dataExportDir, "*.vhd"))
	if err != nil {
		return fmt.Errorf("failed to find VHD files: %w", err)
	}
	if len(vhdFiles) == 0 {
		h.logger.Info("No data disk VHD files found - skipping data disk import")
		return nil
	}
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

	n := len(vhdFiles)
	type diskInfo struct {
		vhdFile      string
		rawFile      string
		baseDiskName string
	}
	disks := make([]diskInfo, n)
	for i, vhdFile := range vhdFiles {
		disks[i] = diskInfo{
			vhdFile:      vhdFile,
			rawFile:      strings.TrimSuffix(vhdFile, ".vhd") + ".raw",
			baseDiskName: strings.TrimSuffix(filepath.Base(vhdFile), ".vhd"),
		}
	}

	// Phase 1: Convert all VHDs to RAW format in parallel
	h.logger.Info("Phase 1: Converting VHD files to RAW format in parallel...")
	convErrors := make([]error, n)
	sem := make(chan struct{}, h.config.DataDiskParallelism)
	var wg sync.WaitGroup
	for i, disk := range disks {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() {
				<-sem
				wg.Done()
			}()
			h.logger.Infof("[%s] Converting VHD to RAW format...", disk.baseDiskName)
			if err := common.ConvertVHDToRAW(disk.vhdFile, disk.rawFile); err != nil {
				convErrors[i] = err
				h.logger.Warningf("[%s] Failed to convert VHD to RAW: %v", disk.baseDiskName, err)
			} else {
				h.logger.Successf("[%s] VHD converted to RAW format", disk.baseDiskName)
			}
		}()
	}
	wg.Wait()

	// Phase 2: Copy data to OCI block volumes in parallel.
	h.logger.Info("Phase 2: Copying data to OCI block volumes in parallel...")
	volumeIDs := make([]string, n)
	ddErrors := make([]error, n)
	var attachMu sync.Mutex
	for i, disk := range disks {
		if convErrors[i] != nil {
			ddErrors[i] = fmt.Errorf("skipping due to conversion failure: %w", convErrors[i])
			continue
		}
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() {
				<-sem
				wg.Done()
			}()
			diskSizeGB, err := common.GetFileSizeGB(disk.rawFile)
			if err != nil {
				ddErrors[i] = fmt.Errorf("failed to get disk size: %w", err)
				h.logger.Warningf("[%s] Failed to get disk size: %v", disk.baseDiskName, err)
				return
			}
			volumeName := fmt.Sprintf("bv-%s", disk.baseDiskName)
			h.logger.Infof("[%s] Creating OCI volume '%s' of size %d GB...", disk.baseDiskName, volumeName, diskSizeGB)
			volumeID, err := h.ociProvider.CreateBlockVolume(ctx, h.config.OCICompartmentID, localAvailabilityDomain, volumeName, diskSizeGB)
			if err != nil {
				ddErrors[i] = fmt.Errorf("failed to create OCI volume: %w", err)
				h.logger.Warningf("[%s] Failed to create OCI volume: %v", disk.baseDiskName, err)
				return
			}
			h.logger.Successf("[%s] Created volume: %s", disk.baseDiskName, volumeID)
			volumeIDs[i] = volumeID

			// Serialize attach+detect to reliably identify the newly attached device
			attachMu.Lock()
			beforeDevices, err := common.ListBlockDevices()
			if err != nil {
				attachMu.Unlock()
				ddErrors[i] = fmt.Errorf("failed to list block devices: %w", err)
				h.logger.Warningf("[%s] Failed to list block devices: %v", disk.baseDiskName, err)
				return
			}
			h.logger.Infof("[%s] Attaching volume to local instance...", disk.baseDiskName)
			attachmentID, err := h.ociProvider.AttachVolume(ctx, localInstanceID, volumeID)
			if err != nil {
				attachMu.Unlock()
				ddErrors[i] = fmt.Errorf("failed to attach volume: %w", err)
				h.logger.Warningf("[%s] Failed to attach volume: %v", disk.baseDiskName, err)
				return
			}
			h.logger.Infof("[%s] Volume attached (attachment: %s)", disk.baseDiskName, attachmentID)
			attachedDevice, err := common.DetectNewBlockDevice(beforeDevices)
			attachMu.Unlock()
			if err != nil {
				h.logger.Warningf("[%s] Could not detect attached device: %v", disk.baseDiskName, err)
				if detachErr := h.ociProvider.DetachVolume(ctx, attachmentID); detachErr != nil {
					h.logger.Warningf("[%s] Failed to detach volume during cleanup: %v", disk.baseDiskName, detachErr)
				}
				ddErrors[i] = fmt.Errorf("failed to detect attached device: %w", err)
				return
			}
			h.logger.Infof("[%s] Attached device: %s", disk.baseDiskName, attachedDevice)

			h.logger.Infof("[%s] Copying data from RAW file to %s (this may take a while)...", disk.baseDiskName, attachedDevice)
			if err := common.CopyDataWithDD(disk.rawFile, attachedDevice); err != nil {
				h.logger.Warningf("[%s] Failed to copy data: %v", disk.baseDiskName, err)
				if detachErr := h.ociProvider.DetachVolume(ctx, attachmentID); detachErr != nil {
					h.logger.Warningf("[%s] Failed to detach volume during cleanup: %v", disk.baseDiskName, detachErr)
				}
				ddErrors[i] = fmt.Errorf("failed to copy data with dd: %w", err)
				return
			}
			h.logger.Successf("[%s] Data copy completed", disk.baseDiskName)

			h.logger.Infof("[%s] Detaching volume...", disk.baseDiskName)
			if err := h.ociProvider.DetachVolume(ctx, attachmentID); err != nil {
				h.logger.Warningf("[%s] Failed to detach volume: %v", disk.baseDiskName, err)
			} else {
				h.logger.Infof("[%s] Volume detached", disk.baseDiskName)
			}
		}()
	}
	wg.Wait()

	// Phase 3: Create snapshots in parallel
	h.logger.Info("Phase 3: Creating snapshots in parallel...")
	snapshotIDs := make([]string, n)
	snapshotNames := make([]string, n)
	snapshotErrors := make([]error, n)
	for i, disk := range disks {
		if ddErrors[i] != nil || volumeIDs[i] == "" {
			continue
		}
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() {
				<-sem
				wg.Done()
			}()
			snapshotName := fmt.Sprintf("ss-%s", disk.baseDiskName)
			h.logger.Infof("[%s] Creating snapshot: %s...", disk.baseDiskName, snapshotName)
			snapshotID, err := h.ociProvider.CreateVolumeSnapshot(ctx, volumeIDs[i], snapshotName)
			if err != nil {
				snapshotErrors[i] = err
				h.logger.Warningf("[%s] Failed to create snapshot: %v", disk.baseDiskName, err)
				return
			}
			snapshotIDs[i] = snapshotID
			snapshotNames[i] = snapshotName
			h.logger.Successf("[%s] Created snapshot: %s", disk.baseDiskName, snapshotID)
		}()
	}
	wg.Wait()

	// Collect snapshot results and count failures
	var failedCount int
	for i := range disks {
		if convErrors[i] != nil || ddErrors[i] != nil || snapshotErrors[i] != nil {
			failedCount++
		}
		if snapshotIDs[i] != "" {
			h.dataDiskSnapshotIDs = append(h.dataDiskSnapshotIDs, snapshotIDs[i])
			h.dataDiskSnapshotNames = append(h.dataDiskSnapshotNames, snapshotNames[i])
		}
	}

	// Cleanup: delete all created block volumes in parallel
	h.logger.Info("Cleaning up temporary block volumes...")
	for i, volumeID := range volumeIDs {
		if volumeID == "" {
			continue
		}
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() {
				<-sem
				wg.Done()
			}()
			h.logger.Infof("Deleting volume %s...", volumeID)
			if err := h.ociProvider.DeleteVolume(ctx, volumeID); err != nil {
				h.logger.Warningf("Failed to delete volume %s: %v", volumeID, err)
			} else {
				h.logger.Successf("[%s] Volume deleted", disks[i].baseDiskName)
			}
		}()
	}
	wg.Wait()

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

func (h *AzureToOCIHandler) getImageImportDetails(ctx context.Context) (namespace, objectName string, err error) {
	qcow2File, err := common.FindDiskFile(h.osExportDir, ".qcow2")
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
	h.logger.Step(10, "Generating Template")
	if h.azureOSDiskSizeGB == 0 {
		h.logger.Info("Reading OS disk size from QCOW2 file...")
		qcow2File, err := common.FindDiskFile(h.osExportDir, ".qcow2")
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
	tfGen := template.NewOCIGenerator(
		h.config, h.logger, h.importedImageID,
		h.dataDiskSnapshotIDs, h.dataDiskSnapshotNames,
		h.azureOSDiskSizeGB, h.azureVMCPUs, h.azureVMMemoryGB, h.azureVMArchitecture,
		h.templateOutputDir,
	)
	return tfGen.GenerateTemplate()
}

func (h *AzureToOCIHandler) waitForImageImportCompletion(ctx context.Context) error {
	if h.importedImageID == "" {
		h.logger.Info("No image import was started, skipping wait")
		return nil
	}

	h.logger.Info("Checking OS image import status before deployment...")
	if err := h.ociProvider.WaitForImageState(ctx, h.importedImageID, core.ImageLifecycleStateAvailable); err != nil {
		return fmt.Errorf("image import did not complete successfully: %w", err)
	}

	h.logger.Success("OS image import completed successfully")
	return nil
}

func (h *AzureToOCIHandler) deployTemplate(ctx context.Context) error {
	h.logger.Step(11, "Deploying the template")
	
	if err := h.waitForImageImportCompletion(ctx); err != nil {
		return fmt.Errorf("failed waiting for image import: %w", err)
	}
	
	tfGen := template.NewOCIGenerator(
		h.config, h.logger, h.importedImageID,
		h.dataDiskSnapshotIDs, h.dataDiskSnapshotNames,
		h.azureOSDiskSizeGB, h.azureVMCPUs, h.azureVMMemoryGB, h.azureVMArchitecture,
		h.templateOutputDir,
	)
	return tfGen.DeployTemplate()
}

func (h *AzureToOCIHandler) verifyWorkflow(ctx context.Context) error {
	h.logger.Step(12, "Verifying Workflow")
	if !h.config.SkipExport {
		if vhdFile, err := common.FindDiskFile(h.osExportDir, ".vhd"); err == nil {
			h.logger.Successf("✓ VHD file exists: %s", filepath.Base(vhdFile))
		}
		if qcow2File, err := common.FindDiskFile(h.osExportDir, ".qcow2"); err == nil {
			h.logger.Successf("✓ QCOW2 file exists: %s", filepath.Base(qcow2File))
		}
	}
	if _, err := os.Stat(h.templateOutputDir); err == nil {
		h.logger.Successf("✓ Template files exist in: %s", h.templateOutputDir)
	}
	h.logger.Success("Workflow verification complete")
	h.logger.Info("=========================================")
	h.logger.Info("Next Steps:")
	if !h.config.SkipTemplateDeploy {
		h.logger.Info("1. Check the OCI console for the deployed instance")
		h.logger.Info("2. Verify the instance is running as expected")
	} else {
		h.logger.Infof("1. Navigate to: %s", h.templateOutputDir)
		h.logger.Info("2. Run: tofu init && tofu apply")
		h.logger.Info("3. Check the OCI console for the deployed instance")
	}
	h.logger.Info("=========================================")
	return nil
}

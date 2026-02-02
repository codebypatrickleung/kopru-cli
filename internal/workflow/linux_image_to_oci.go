// Package workflow provides workflow handlers for specific migration paths.
package workflow

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/codebypatrickleung/kopru-cli/internal/cloud/oci"
	"github.com/codebypatrickleung/kopru-cli/internal/common"
	"github.com/codebypatrickleung/kopru-cli/internal/config"
	"github.com/codebypatrickleung/kopru-cli/internal/logger"
	"github.com/codebypatrickleung/kopru-cli/internal/template"
)

// LinuxImageToOCIHandler implements the workflow for creating OCI instances from Linux cloud images.
type LinuxImageToOCIHandler struct {
	config            *config.Config
	logger            *logger.Logger
	ociProvider       *oci.Provider
	osImageURL        string
	osDiskSizeGB      int64
	osArchitecture    string
}

const (
	imageExportDir    = "./linux-image-download"
)

func NewLinuxImageToOCIHandler() *LinuxImageToOCIHandler      { return &LinuxImageToOCIHandler{} }
func (h *LinuxImageToOCIHandler) Name() string           { return "Linux Image to OCI Deployment" }
func (h *LinuxImageToOCIHandler) SourcePlatform() string { return "linux_image" }
func (h *LinuxImageToOCIHandler) TargetPlatform() string { return "oci" }

func (h *LinuxImageToOCIHandler) Initialize(cfg *config.Config, log *logger.Logger) error {
	h.config, h.logger = cfg, log
	var err error
	if h.ociProvider, err = oci.NewProvider(cfg.OCIRegion, log); err != nil {
		return fmt.Errorf("failed to initialize OCI provider: %w", err)
	}
	
	if cfg.OSImageURL != "" {
		h.osImageURL = cfg.OSImageURL
	} else {
		return fmt.Errorf("OS image URL (OS_IMAGE_URL) is required for Linux Image to OCI workflow")
	}
	h.osArchitecture = "x86_64"
	
	return nil
}

func (h *LinuxImageToOCIHandler) Execute(ctx context.Context) error {
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
		{h.config.SkipExport, "Skipping OS image download (SKIP_OS_EXPORT=true)", "OS image download failed", h.downloadOSImage},
		{h.config.SkipConfigure, "Skipping image configuration (SKIP_OS_CONFIGURE=true)", "image configuration failed", h.configureImage},
		{h.config.SkipUpload, "Skipping image upload (SKIP_OS_UPLOAD=true)", "image upload failed", h.uploadImage},
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
	h.logger.Success("Linux Image to OCI deployment completed successfully!")
	h.logger.Success("=========================================")
	return nil
}

func (h *LinuxImageToOCIHandler) runPrerequisites(ctx context.Context) error {
	h.logger.Step(1, "Reviewing Deployment Configuration")
	h.logger.Infof("OS Image URL: %s", h.osImageURL)
	h.logger.Infof("OCI Compartment ID: %s", h.config.OCICompartmentID)
	h.logger.Infof("OCI Subnet ID: %s", h.config.OCISubnetID)
	h.logger.Infof("OCI Region: %s", h.config.OCIRegion)
	h.logger.Infof("OCI Bucket Name: %s", h.config.OCIBucketName)
	h.logger.Infof("OCI Image Name: %s", h.config.OCIImageName)
	h.logger.Infof("OCI Image OS: %s", h.config.OCIImageOS)
	h.logger.Infof("OCI Image OS Version: %s", h.config.OCIImageOSVersion)
	h.logger.Infof("Template Output Dir: %s", h.config.TemplateOutputDir)
	h.logger.Infof("SSH Key File Path: %s", h.config.SSHKeyFilePath)
	
	h.logger.Step(2, "Running Prerequisite Checks")
	for _, tool := range []string{"qemu-img", "virt-customize", "curl"} {
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
	h.logger.Warning("Ignore this warning if your available disk space exceeds 50 GB.")
	
	// Set default OS values if not provided
	if h.config.OCIImageOS == "" {
		h.config.OCIImageOS = "Generic Linux"
		h.logger.Infof("OCI_IMAGE_OS not set, using default: %s", h.config.OCIImageOS)
	}
	if h.config.OCIImageOSVersion == "" {
		h.config.OCIImageOSVersion = "1.0"
		h.logger.Infof("OCI_IMAGE_OS_VERSION not set, using default: %s", h.config.OCIImageOSVersion)
	}
	
	h.logger.Successf("✓ Operating system configured for OCI: %s %s", h.config.OCIImageOS, h.config.OCIImageOSVersion)
	
	// Set image and instance names if using defaults
	if h.config.OCIImageName == "kopru-image" {
		h.config.OCIImageName = fmt.Sprintf("%s-%s-image", strings.ReplaceAll(h.config.OCIImageOS, " ", "-"), h.config.OCIImageOSVersion)
		h.logger.Infof("Using image name: %s", h.config.OCIImageName)
	}
	if h.config.OCIInstanceName == "kopru-instance" {
		h.config.OCIInstanceName = fmt.Sprintf("%s-%s-instance", strings.ReplaceAll(h.config.OCIImageOS, " ", "-"), h.config.OCIImageOSVersion)
		h.logger.Infof("Using instance name: %s", h.config.OCIInstanceName)
	}
	
	if h.config.OCIRegion == "" {
		return fmt.Errorf("OCI region (OCI_REGION) is required")
	}
	h.logger.Successf("✓ OCI region configured: %s", h.config.OCIRegion)
	
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

func (h *LinuxImageToOCIHandler) downloadOSImage(ctx context.Context) error {
	h.logger.Step(3, "Downloading Linux Cloud Image")
	
	if err := common.EnsureDir(imageExportDir); err != nil {
		return fmt.Errorf("failed to create download directory: %w", err)
	}
	h.logger.Infof("Download directory: %s", imageExportDir)
	
	urlParts := strings.Split(h.osImageURL, "/")
	filename := urlParts[len(urlParts)-1]
	destPath := filepath.Join(imageExportDir, filename)
	
	if _, err := os.Stat(destPath); err == nil {
		h.logger.Infof("Image file already exists: %s", destPath)
		h.logger.Success("Skipping download")
		return nil
	}
	
	h.logger.Infof("Downloading from: %s", h.osImageURL)
	h.logger.Info("This may take a few minutes...")
	
	resp, err := http.Get(h.osImageURL)
	if err != nil {
		return fmt.Errorf("failed to download OS image: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download OS image: HTTP %d", resp.StatusCode)
	}
	
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer out.Close()
	
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write downloaded image: %w", err)
	}
	
	h.logger.Successf("Linux cloud image downloaded to: %s", destPath)
	return nil
}

func (h *LinuxImageToOCIHandler) configureImage(ctx context.Context) error {
	h.logger.Step(4, "Configuring Image for OCI")
	
	qcow2File, err := common.FindDiskFile(imageExportDir, ".qcow2")
	if err != nil {
		return fmt.Errorf("failed to find QCOW2 file: %w", err)
	}
	h.logger.Infof("Configuring QCOW2 file: %s", qcow2File)
	
	h.logger.Info("Applying Linux Image to OCI configurations using virt-customize...")
	if err := common.ExecuteOSConfigScript(qcow2File, h.config.OCIImageOS, h.SourcePlatform(), h.logger); err != nil {
		return fmt.Errorf("failed to execute OS configuration script: %w", err)
	}
	
	h.logger.Success("Image configurations complete")
	return nil
}

func (h *LinuxImageToOCIHandler) uploadImage(ctx context.Context) error {
	h.logger.Step(5, "Uploading Image to OCI")
	
	qcow2File, err := common.FindDiskFile(imageExportDir, ".qcow2")
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

// getImageImportDetails retrieves namespace and object name for image import
func (h *LinuxImageToOCIHandler) getImageImportDetails(ctx context.Context) (namespace, objectName string, err error) {
	qcow2File, err := common.FindDiskFile(imageExportDir, ".qcow2")
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

func (h *LinuxImageToOCIHandler) generateTemplate(ctx context.Context) error {
	h.logger.Step(6, "Generating Template")
	
	if h.osDiskSizeGB == 0 {
		h.logger.Info("Reading OS disk size from QCOW2 file...")
		qcow2File, err := common.FindDiskFile(imageExportDir, ".qcow2")
		if err != nil {
			return fmt.Errorf("failed to find QCOW2 file: %w", err)
		}
		osDiskSizeGB, err := common.GetComputeOSDiskSizeGB(qcow2File)
		if err != nil {
			return fmt.Errorf("failed to get OS disk size from QCOW2: %w", err)
		}
		h.osDiskSizeGB = osDiskSizeGB
		h.logger.Successf("✓ OS disk size from QCOW2: %d GB", osDiskSizeGB)
		if h.osDiskSizeGB < common.OCIMinVolumeSizeGB {
			h.logger.Infof("OS disk size (%d GB) is less than OCI minimum (%d GB)", h.osDiskSizeGB, common.OCIMinVolumeSizeGB)
			h.logger.Infof("Boot volume will be created with minimum size of %d GB", common.OCIMinVolumeSizeGB)
		}
	}
	
	namespace, objectName, err := h.getImageImportDetails(ctx)
	if err != nil {
		return err
	}
	
	tfGen := template.NewOCIGenerator(
		h.config, h.logger, namespace, objectName,
		[]string{}, []string{}, 
		h.osDiskSizeGB, 0, 0, h.osArchitecture, 
	)
	return tfGen.GenerateTemplate()
}

func (h *LinuxImageToOCIHandler) deployTemplate(ctx context.Context) error {
	h.logger.Step(7, "Deploying the template")
	
	namespace, objectName, err := h.getImageImportDetails(ctx)
	if err != nil {
		return err
	}
	
	tfGen := template.NewOCIGenerator(
		h.config, h.logger, namespace, objectName,
		[]string{}, []string{},
		h.osDiskSizeGB, 0, 0, h.osArchitecture,
	)
	return tfGen.DeployTemplate()
}

func (h *LinuxImageToOCIHandler) verifyWorkflow(ctx context.Context) error {
	h.logger.Step(8, "Verifying Workflow")
	
	if !h.config.SkipExport {
		if qcow2File, err := common.FindDiskFile(imageExportDir, ".qcow2"); err == nil {
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

// Package config handles configuration loading from files, environment variables, and flags.
package config

import (
	"fmt"
	"os"

	"github.com/codebypatrickleung/kopru-cli/internal/common"
	"github.com/spf13/viper"
)

const (
	// Default configuration values
	defaultImageName     = "kopru-image"
	defaultInstanceName  = "kopru-instance"
	imageSuffix          = "-image"
	templateOutputSuffix = "-template-output"
)

// Config holds all configuration for the Kopru CLI.
type Config struct {
	// Platform configuration
	SourcePlatform string
	TargetPlatform string

	// Azure configuration
	AzureComputeName    string
	AzureResourceGroup  string
	AzureSubscriptionID string

	// OCI configuration
	OCICompartmentID      string
	OCISubnetID           string
	OCIBucketName         string
	OCIImageName          string
	OCIImageOS            string
	OCIInstanceName       string
	OCIRegion             string
	OCIAvailabilityDomain string

	// Template configuration
	TemplateOutputDir               string
	SkipTemplateDeploy              bool
	CustomOSConfigurationScript     string

	// Skip options (for resuming)
	SkipPrereq    bool
	SkipExport    bool
	SkipConvert   bool
	SkipConfigure bool
	SkipUpload    bool
	SkipImport    bool
	SkipDDExport  bool
	SkipDDImport  bool
	SkipTemplate  bool
	SkipVerify    bool

	// Advanced options
	KeepVHD         bool
	DiskMappingFile string
	Debug           bool
}

// Load initializes configuration from file, environment variables, and flags.
func Load(configFile string) (*Config, error) {
	// Set defaults
	viper.SetDefault("source_platform", "azure")
	viper.SetDefault("target_platform", "oci")
	viper.SetDefault("oci_bucket_name", "kopru-bucket")
	viper.SetDefault("oci_image_name", defaultImageName)
	viper.SetDefault("oci_instance_name", defaultInstanceName)
	viper.SetDefault("oci_region", "eu-frankfurt-1")
	viper.SetDefault("template_output_dir", "./template-output")
	viper.SetDefault("keep_vhd", true)
	viper.SetDefault("disk_mapping_file", "./disk-mapping.json")

	// Set environment variable prefix
	viper.SetEnvPrefix("")
	viper.AutomaticEnv()

	// Load config file if it exists
	if configFile != "" {
		if _, err := os.Stat(configFile); err == nil {
			viper.SetConfigFile(configFile)
			if err := viper.ReadInConfig(); err != nil {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}
		}
	}

	azureComputeName := viper.GetString("azure_compute_name")

	// Set TemplateOutputDir based on Azure Compute name if using default
	templateOutputDir := viper.GetString("template_output_dir")
	if templateOutputDir == "./template-output" && azureComputeName != "" {
		// Import common package for SanitizeName
		sanitizedName := common.SanitizeName(azureComputeName)
		templateOutputDir = fmt.Sprintf("./%s%s", sanitizedName, templateOutputSuffix)
	}

	// Set OCIInstanceName based on Azure Compute name if using default
	ociInstanceName := viper.GetString("oci_instance_name")
	if (ociInstanceName == defaultInstanceName || ociInstanceName == "") && azureComputeName != "" {
		ociInstanceName = common.SanitizeName(azureComputeName)
	} else if ociInstanceName == "" {
		// If no Azure VM name and no explicit OCI instance name, use default
		ociInstanceName = defaultInstanceName
	}

	// Set OCIImageName based on Azure Compute name if using default
	ociImageName := viper.GetString("oci_image_name")
	if (ociImageName == defaultImageName || ociImageName == "") && azureComputeName != "" {
		ociImageName = fmt.Sprintf("%s%s", common.SanitizeName(azureComputeName), imageSuffix)
	} else if ociImageName == "" {
		// If no Azure VM name and no explicit OCI image name, use default
		ociImageName = defaultImageName
	}

	cfg := &Config{
		SourcePlatform:              viper.GetString("source_platform"),
		TargetPlatform:              viper.GetString("target_platform"),
		AzureComputeName:            azureComputeName,
		AzureResourceGroup:          viper.GetString("azure_resource_group"),
		AzureSubscriptionID:         viper.GetString("azure_subscription_id"),
		OCICompartmentID:            viper.GetString("oci_compartment_id"),
		OCISubnetID:                 viper.GetString("oci_subnet_id"),
		OCIBucketName:               viper.GetString("oci_bucket_name"),
		OCIImageName:                ociImageName,
		OCIImageOS:                  viper.GetString("oci_image_os"),
		OCIInstanceName:             ociInstanceName,
		OCIRegion:                   viper.GetString("oci_region"),
		OCIAvailabilityDomain:       viper.GetString("oci_availability_domain"),
		TemplateOutputDir:           templateOutputDir,
		SkipTemplateDeploy:          viper.GetBool("skip_template_deploy"),
		CustomOSConfigurationScript: viper.GetString("custom_os_configuration_script"),
		SkipPrereq:                  viper.GetBool("skip_prereq"),
		SkipExport:                  viper.GetBool("skip_os_export"),
		SkipConvert:                 viper.GetBool("skip_os_convert"),
		SkipConfigure:               viper.GetBool("skip_os_configure"),
		SkipUpload:                  viper.GetBool("skip_os_upload"),
		SkipImport:                  viper.GetBool("skip_os_import"),
		SkipDDExport:                viper.GetBool("skip_dd_export"),
		SkipDDImport:                viper.GetBool("skip_dd_import"),
		SkipTemplate:                viper.GetBool("skip_template"),
		SkipVerify:                  viper.GetBool("skip_verify"),
		KeepVHD:                     viper.GetBool("keep_vhd"),
		DiskMappingFile:             viper.GetString("disk_mapping_file"),
		Debug:                       viper.GetBool("debug"),
	}

	return cfg, nil
}

// Validate checks that required configuration is present.
func (c *Config) Validate() error {
	if c.SourcePlatform == "azure" {
		if c.AzureComputeName == "" {
			return fmt.Errorf("azure_compute_name is required for Azure source platform")
		}
		if c.AzureResourceGroup == "" {
			return fmt.Errorf("azure_resource_group is required for Azure source platform")
		}
	}

	if c.TargetPlatform == "oci" {
		if c.OCICompartmentID == "" {
			return fmt.Errorf("oci_compartment_id is required for OCI target platform")
		}
		if c.OCISubnetID == "" {
			return fmt.Errorf("oci_subnet_id is required for OCI target platform")
		}
	}

	if c.OCIImageOS == "CUSTOM" && c.CustomOSConfigurationScript == "" {
		return fmt.Errorf("custom_os_configuration_script is required when oci_image_os is set to CUSTOM")
	}

	return nil
}

// LoadConfig loads configuration using the global Viper instance.
// This is a convenience wrapper around Load("") that uses the global
// Viper instance configured by the CLI initialization (cobra/viper).
// It reads from flags, environment variables, and config files in that priority order.
func LoadConfig() (*Config, error) {
	return Load("")
}

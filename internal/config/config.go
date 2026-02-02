// Package config handles configuration loading from files, environment variables, and flags.
package config

import (
	"fmt"
	"os"

	"github.com/codebypatrickleung/kopru-cli/internal/common"
	"github.com/spf13/viper"
)

const (
	defaultImageName     = "kopru-image"
	defaultInstanceName  = "kopru-instance"
	imageSuffix          = "-image"
	templateOutputSuffix = "-template-output"
)

// Config holds all configuration for the Kopru CLI.
type Config struct {
	SourcePlatform        string
	TargetPlatform        string
	AzureComputeName      string
	AzureResourceGroup    string
	AzureSubscriptionID   string
	OCICompartmentID      string
	OCISubnetID           string
	OCIBucketName         string
	OCIImageName          string
	OCIImageOS            string
	OCIImageOSVersion     string
	OCIImageEnableUEFI    bool
	OCIInstanceName       string
	OCIRegion             string
	OCIAvailabilityDomain string
	OSImageURL            string
	TemplateOutputDir     string
	SSHKeyFilePath        string
	SkipPrereq            bool
	SkipExport            bool
	SkipConvert           bool
	SkipConfigure         bool
	SkipUpload            bool
	SkipDDExport          bool
	SkipDDImport          bool
	SkipTemplate          bool
	SkipTemplateDeploy    bool
	SkipVerify            bool
	Debug                 bool
}

// Load initializes configuration from file, environment variables, and flags.
func Load(configFile string) (*Config, error) {
	viper.SetDefault("source_platform", "azure")
	viper.SetDefault("target_platform", "oci")
	viper.SetDefault("oci_bucket_name", "kopru-bucket")
	viper.SetDefault("oci_image_name", defaultImageName)
	viper.SetDefault("oci_instance_name", defaultInstanceName)
	viper.SetDefault("template_output_dir", "./template-output")

	viper.AutomaticEnv()

	if configFile != "" {
		if _, err := os.Stat(configFile); err == nil {
			viper.SetConfigFile(configFile)
			if err := viper.ReadInConfig(); err != nil {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}
		}
	}

	azureComputeName := viper.GetString("azure_compute_name")
	templateOutputDir := viper.GetString("template_output_dir")
	if templateOutputDir == "./template-output" && azureComputeName != "" {
		templateOutputDir = fmt.Sprintf("./%s%s", common.SanitizeName(azureComputeName), templateOutputSuffix)
	}

	ociInstanceName := viper.GetString("oci_instance_name")
	if (ociInstanceName == defaultInstanceName || ociInstanceName == "") && azureComputeName != "" {
		ociInstanceName = common.SanitizeName(azureComputeName)
	} else if ociInstanceName == "" {
		ociInstanceName = defaultInstanceName
	}

	ociImageName := viper.GetString("oci_image_name")
	if (ociImageName == defaultImageName || ociImageName == "") && azureComputeName != "" {
		ociImageName = fmt.Sprintf("%s%s", common.SanitizeName(azureComputeName), imageSuffix)
	} else if ociImageName == "" {
		ociImageName = defaultImageName
	}

	cfg := &Config{
		SourcePlatform:        viper.GetString("source_platform"),
		TargetPlatform:        viper.GetString("target_platform"),
		AzureComputeName:      azureComputeName,
		AzureResourceGroup:    viper.GetString("azure_resource_group"),
		AzureSubscriptionID:   viper.GetString("azure_subscription_id"),
		OCICompartmentID:      viper.GetString("oci_compartment_id"),
		OCISubnetID:           viper.GetString("oci_subnet_id"),
		OCIBucketName:         viper.GetString("oci_bucket_name"),
		OCIImageName:          ociImageName,
		OCIImageOS:            viper.GetString("oci_image_os"),
		OCIImageOSVersion:     viper.GetString("oci_image_os_version"),
		OCIImageEnableUEFI:    viper.GetBool("oci_image_enable_uefi"),
		OCIInstanceName:       ociInstanceName,
		OCIRegion:             viper.GetString("oci_region"),
		OCIAvailabilityDomain: viper.GetString("oci_availability_domain"),
		OSImageURL:            viper.GetString("os_image_url"),
		TemplateOutputDir:     templateOutputDir,
		SSHKeyFilePath:        viper.GetString("ssh_key_file"),
		SkipPrereq:            viper.GetBool("skip_prereq"),
		SkipExport:            viper.GetBool("skip_os_export"),
		SkipConvert:           viper.GetBool("skip_os_convert"),
		SkipConfigure:         viper.GetBool("skip_os_configure"),
		SkipUpload:            viper.GetBool("skip_os_upload"),
		SkipDDExport:          viper.GetBool("skip_dd_export"),
		SkipDDImport:          viper.GetBool("skip_dd_import"),
		SkipTemplate:          viper.GetBool("skip_template"),
		SkipTemplateDeploy:    viper.GetBool("skip_template_deploy"),
		SkipVerify:            viper.GetBool("skip_verify"),
		Debug:                 viper.GetBool("debug"),
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
		if c.OCIRegion == "" {
			return fmt.Errorf("oci_region is required for OCI target platform")
		}
	}
	return nil
}

// LoadConfig loads configuration using the global Viper instance.
func LoadConfig() (*Config, error) {
	return Load("")
}

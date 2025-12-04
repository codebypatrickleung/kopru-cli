// Package main provides the entry point for the Kopru CLI.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/codebypatrickleung/kopru-cli/internal/config"
	"github.com/codebypatrickleung/kopru-cli/internal/logger"
	"github.com/codebypatrickleung/kopru-cli/internal/workflow"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	version = "0.1.0"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:     "kopru",
	Short:   "Kopru - Compute Migration Tool",
	Long:    `Kopru is a Go-based CLI tool that orchestrates Compute import into Oracle Cloud Infrastructure (OCI).`,
	Version: version,
	RunE:    run,
}

func init() {
	cobra.OnInitialize(initConfig)

	// Configuration file flag
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./kopru-config.env)")

	// Azure flags
	rootCmd.Flags().String("azure-subscription-id", "", "Azure subscription ID")
	rootCmd.Flags().String("azure-resource-group", "", "Azure resource group name")
	rootCmd.Flags().String("azure-compute-name", "", "Azure compute instance name")

	// OCI flags
	rootCmd.Flags().String("oci-region", "", "OCI region")
	rootCmd.Flags().String("oci-compartment-id", "", "OCI compartment OCID")
	rootCmd.Flags().String("oci-subnet-id", "", "OCI subnet OCID")
	rootCmd.Flags().String("oci-bucket-name", "", "OCI Object Storage bucket name")
	rootCmd.Flags().String("oci-image-name", "", "OCI custom image name")
	rootCmd.Flags().String("oci-image-os", "Ubuntu", "OS type for OCI adjustments (Ubuntu, CUSTOM)")
	rootCmd.Flags().String("oci-instance-name", "", "OCI instance name")
	rootCmd.Flags().String("oci-availability-domain", "", "OCI availability domain")

	// Workflow control flags
	rootCmd.Flags().Bool("skip-prereq", false, "Skip prerequisite checks")
	rootCmd.Flags().Bool("skip-os-export", false, "Skip OS disk export")
	rootCmd.Flags().Bool("skip-os-convert", false, "Skip QCOW2 conversion")
	rootCmd.Flags().Bool("skip-os-configure", false, "Skip image configuration")
	rootCmd.Flags().Bool("skip-os-upload", false, "Skip image upload to OCI")
	rootCmd.Flags().Bool("skip-os-import", false, "Skip custom image import")
	rootCmd.Flags().Bool("skip-dd-export", false, "Skip data disk export")
	rootCmd.Flags().Bool("skip-dd-import", false, "Skip data disk import")
	rootCmd.Flags().Bool("skip-template", false, "Skip template generation")
	rootCmd.Flags().Bool("skip-template-deploy", false, "Skip template deployment")
	rootCmd.Flags().Bool("skip-verify", false, "Skip workflow verification")

	// Other flags
	rootCmd.Flags().Bool("keep-vhd", false, "Keep VHD file after conversion to QCOW2")
	rootCmd.Flags().String("custom-os-configuration-script", "", "Path to custom OS configuration script")
	rootCmd.Flags().String("template-output-dir", "./template-output", "Directory for template files")
	rootCmd.Flags().String("source-platform", "azure", "Source cloud platform (azure)")
	rootCmd.Flags().String("target-platform", "oci", "Target cloud platform (oci)")
	rootCmd.Flags().Bool("debug", false, "Enable debug logging")

	// Bind flags to viper
	viper.BindPFlag("AZURE_SUBSCRIPTION_ID", rootCmd.Flags().Lookup("azure-subscription-id"))
	viper.BindPFlag("AZURE_RESOURCE_GROUP", rootCmd.Flags().Lookup("azure-resource-group"))
	viper.BindPFlag("AZURE_COMPUTE_NAME", rootCmd.Flags().Lookup("azure-compute-name"))
	viper.BindPFlag("OCI_REGION", rootCmd.Flags().Lookup("oci-region"))
	viper.BindPFlag("OCI_COMPARTMENT_ID", rootCmd.Flags().Lookup("oci-compartment-id"))
	viper.BindPFlag("OCI_SUBNET_ID", rootCmd.Flags().Lookup("oci-subnet-id"))
	viper.BindPFlag("OCI_BUCKET_NAME", rootCmd.Flags().Lookup("oci-bucket-name"))
	viper.BindPFlag("OCI_IMAGE_NAME", rootCmd.Flags().Lookup("oci-image-name"))
	viper.BindPFlag("OCI_IMAGE_OS", rootCmd.Flags().Lookup("oci-image-os"))
	viper.BindPFlag("OCI_INSTANCE_NAME", rootCmd.Flags().Lookup("oci-instance-name"))
	viper.BindPFlag("OCI_AVAILABILITY_DOMAIN", rootCmd.Flags().Lookup("oci-availability-domain"))
	viper.BindPFlag("SKIP_PREREQ", rootCmd.Flags().Lookup("skip-prereq"))
	viper.BindPFlag("SKIP_OS_EXPORT", rootCmd.Flags().Lookup("skip-os-export"))
	viper.BindPFlag("SKIP_OS_CONVERT", rootCmd.Flags().Lookup("skip-os-convert"))
	viper.BindPFlag("SKIP_OS_CONFIGURE", rootCmd.Flags().Lookup("skip-os-configure"))
	viper.BindPFlag("SKIP_OS_UPLOAD", rootCmd.Flags().Lookup("skip-os-upload"))
	viper.BindPFlag("SKIP_OS_IMPORT", rootCmd.Flags().Lookup("skip-os-import"))
	viper.BindPFlag("SKIP_DD_EXPORT", rootCmd.Flags().Lookup("skip-dd-export"))
	viper.BindPFlag("SKIP_DD_IMPORT", rootCmd.Flags().Lookup("skip-dd-import"))
	viper.BindPFlag("SKIP_TEMPLATE", rootCmd.Flags().Lookup("skip-template"))
	viper.BindPFlag("SKIP_TEMPLATE_DEPLOY", rootCmd.Flags().Lookup("skip-template-deploy"))
	viper.BindPFlag("SKIP_VERIFY", rootCmd.Flags().Lookup("skip-verify"))
	viper.BindPFlag("KEEP_VHD", rootCmd.Flags().Lookup("keep-vhd"))
	viper.BindPFlag("CUSTOM_OS_ADJUSTMENT_SCRIPT", rootCmd.Flags().Lookup("custom-os-configuration-script"))
	viper.BindPFlag("TEMPLATE_OUTPUT_DIR", rootCmd.Flags().Lookup("template-output-dir"))
	viper.BindPFlag("SOURCE_PLATFORM", rootCmd.Flags().Lookup("source-platform"))
	viper.BindPFlag("TARGET_PLATFORM", rootCmd.Flags().Lookup("target-platform"))
	viper.BindPFlag("DEBUG", rootCmd.Flags().Lookup("debug"))
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Search for kopru-config.env in current directory
		viper.AddConfigPath(".")
		viper.SetConfigName("kopru-config")
		viper.SetConfigType("env")
	}

	// Read in environment variables that match
	viper.AutomaticEnv()

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Load configuration first to get debug flag
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create log file with timestamp
	timestamp := logger.GetTimestamp()
	logFileName := fmt.Sprintf("kopru-%s.log", timestamp)

	// Initialize logger with debug mode and log file
	log, err := logger.NewWithFile(cfg.Debug, logFileName)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer log.Close()

	log.Infof("Kopru version %s", version)
	log.Infof("Log file: %s", logFileName)

	// Validate required configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Create workflow manager
	ctx := context.Background()
	mgr, err := workflow.NewManager(cfg, log, version)
	if err != nil {
		return fmt.Errorf("failed to create workflow manager: %w", err)
	}

	// Run the migration workflow
	if err := mgr.Run(ctx); err != nil {
		log.Error(fmt.Sprintf("Workflow failed: %v", err))
		return err
	}

	return nil
}

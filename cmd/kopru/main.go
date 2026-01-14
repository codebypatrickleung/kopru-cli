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
	version = "0.1.6"
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

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./kopru-config.env)")

	flags := []struct {
		name, shorthand, usage, defaultValue string
	}{
		{"azure-subscription-id", "", "Azure subscription ID", ""},
		{"azure-resource-group", "", "Azure resource group name", ""},
		{"azure-compute-name", "", "Azure compute instance name", ""},
		{"oci-region", "", "OCI region", ""},
		{"oci-compartment-id", "", "OCI compartment OCID", ""},
		{"oci-subnet-id", "", "OCI subnet OCID", ""},
		{"oci-bucket-name", "", "OCI Object Storage bucket name", ""},
		{"oci-image-name", "", "OCI custom image name", ""},
		{"oci-image-os", "", "OS type for OCI (Ubuntu, Windows, Debian, Oracle Linux, AlmaLinux, CentOS, RHEL, Rocky Linux, SUSE, Generic Linux)", ""},
		{"oci-image-os-version", "", "OS version for OCI (e.g., 20.04, 22.04, 2019, 2022)", ""},
		{"oci-image-enable-uefi", "", "Enable UEFI for OCI image (true or false)", "false"},
		{"oci-instance-name", "", "OCI instance name", ""},
		{"oci-availability-domain", "", "OCI availability domain", ""},
		{"template-output-dir", "", "Directory for template files", "./template-output"},
		{"source-platform", "", "Source cloud platform (azure)", "azure"},
		{"target-platform", "", "Target cloud platform (oci)", "oci"},
	}
	for _, f := range flags {
		rootCmd.Flags().String(f.name, f.defaultValue, f.usage)
	}

	boolFlags := []struct {
		name, usage string
	}{
		{"skip-prereq", "Skip prerequisite checks"},
		{"skip-os-export", "Skip OS disk export"},
		{"skip-os-convert", "Skip QCOW2 conversion"},
		{"skip-os-configure", "Skip image configuration"},
		{"skip-os-upload", "Skip image upload to OCI"},
		{"skip-dd-export", "Skip data disk export"},
		{"skip-dd-import", "Skip data disk import"},
		{"skip-template", "Skip template generation"},
		{"skip-template-deploy", "Skip template deployment"},
		{"skip-verify", "Skip workflow verification"},
		{"debug", "Enable debug logging"},
	}
	for _, f := range boolFlags {
		rootCmd.Flags().Bool(f.name, false, f.usage)
	}

	bindings := map[string]string{
		"AZURE_SUBSCRIPTION_ID":   "azure-subscription-id",
		"AZURE_RESOURCE_GROUP":    "azure-resource-group",
		"AZURE_COMPUTE_NAME":      "azure-compute-name",
		"OCI_REGION":              "oci-region",
		"OCI_COMPARTMENT_ID":      "oci-compartment-id",
		"OCI_SUBNET_ID":           "oci-subnet-id",
		"OCI_BUCKET_NAME":         "oci-bucket-name",
		"OCI_IMAGE_NAME":          "oci-image-name",
		"OCI_IMAGE_OS":            "oci-image-os",
		"OCI_IMAGE_OS_VERSION":    "oci-image-os-version",
		"OCI_IMAGE_ENABLE_UEFI":   "oci-image-enable-uefi",
		"OCI_INSTANCE_NAME":       "oci-instance-name",
		"OCI_AVAILABILITY_DOMAIN": "oci-availability-domain",
		"SKIP_PREREQ":             "skip-prereq",
		"SKIP_OS_EXPORT":          "skip-os-export",
		"SKIP_OS_CONVERT":         "skip-os-convert",
		"SKIP_OS_CONFIGURE":       "skip-os-configure",
		"SKIP_OS_UPLOAD":          "skip-os-upload",
		"SKIP_DD_EXPORT":          "skip-dd-export",
		"SKIP_DD_IMPORT":          "skip-dd-import",
		"SKIP_TEMPLATE":           "skip-template",
		"SKIP_TEMPLATE_DEPLOY":    "skip-template-deploy",
		"SKIP_VERIFY":             "skip-verify",
		"TEMPLATE_OUTPUT_DIR":     "template-output-dir",
		"SOURCE_PLATFORM":         "source-platform",
		"TARGET_PLATFORM":         "target-platform",
		"DEBUG":                   "debug",
	}
	for env, flag := range bindings {
		if err := viper.BindPFlag(env, rootCmd.Flags().Lookup(flag)); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to bind flag %s to env %s: %v\n", flag, env, err)
		}
	}
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName("kopru-config")
		viper.SetConfigType("env")
	}
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

func run(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	timestamp := logger.GetTimestamp()
	logFileName := fmt.Sprintf("kopru-%s.log", timestamp)

	log, err := logger.NewWithFile(cfg.Debug, logFileName)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer log.Close()

	log.Infof("Kopru version %s", version)
	log.Infof("Log file: %s", logFileName)

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	ctx := context.Background()
	mgr, err := workflow.NewManager(cfg, log, version)
	if err != nil {
		return fmt.Errorf("failed to create workflow manager: %w", err)
	}

	if err := mgr.Run(ctx); err != nil {
		log.Error(fmt.Sprintf("Workflow failed: %v", err))
		return err
	}

	return nil
}

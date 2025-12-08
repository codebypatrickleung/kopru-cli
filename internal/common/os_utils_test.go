package common

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codebypatrickleung/kopru-cli/internal/logger"
)

func TestExecuteCustomOSConfigScript(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test-script.sh")
	scriptContent := `#!/bin/bash
echo "stdout: Script started"
echo "stderr: Processing..." >&2
echo "stdout: Script completed"
exit 0
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	logFile := filepath.Join(tmpDir, "test.log")
	log, err := logger.NewWithFile(false, logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer log.Close()

	if err := ExecuteCustomOSConfigScript(tmpDir, scriptPath, log); err != nil {
		t.Fatalf("Script execution failed: %v", err)
	}

	logContent, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	logString := string(logContent)

	tests := []struct {
		substr string
		msg    string
	}{
		{"Starting script execution: test-script.sh", "Log should contain start message"},
		{"Script execution completed: test-script.sh", "Log should contain completion message"},
		{"stdout: Script started", "Log should contain stdout output"},
		{"stderr: Processing...", "Log should contain stderr output"},
		{"stdout: Script completed", "Log should contain final stdout output"},
	}
	for _, tt := range tests {
		if !strings.Contains(logString, tt.substr) {
			t.Error(tt.msg)
		}
	}
}

func TestExecuteCustomOSConfigScript_ScriptFailure(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "failing-script.sh")
	scriptContent := `#!/bin/bash
echo "Script starting"
echo "An error occurred" >&2
exit 1
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	logFile := filepath.Join(tmpDir, "test.log")
	log, err := logger.NewWithFile(false, logFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer log.Close()

	if err := ExecuteCustomOSConfigScript(tmpDir, scriptPath, log); err == nil {
		t.Fatal("Script execution should have failed")
	}

	logContent, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	logString := string(logContent)

	tests := []struct {
		substr string
		msg    string
	}{
		{"Starting script execution: failing-script.sh", "Log should contain start message"},
		{"Script execution failed: failing-script.sh", "Log should contain failure message"},
		{"Script starting", "Log should contain script output even on failure"},
		{"An error occurred", "Log should contain error message from script"},
	}
	for _, tt := range tests {
		if !strings.Contains(logString, tt.substr) {
			t.Error(tt.msg)
		}
	}
}

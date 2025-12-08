package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoggerNew(t *testing.T) {
	log := New(false)
	if log == nil {
		t.Fatal("Expected logger to be created, got nil")
	}
	if log.debug {
		t.Error("Expected debug to be false")
	}

	logDebug := New(true)
	if !logDebug.debug {
		t.Error("Expected debug to be true")
	}
}

func TestLoggerNewWithFile(t *testing.T) {
	tmpDir := t.TempDir()
	logFilePath := filepath.Join(tmpDir, "test.log")

	log, err := NewWithFile(false, logFilePath)
	if err != nil {
		t.Fatalf("Failed to create logger with file: %v", err)
	}
	defer log.Close()

	if log == nil {
		t.Fatal("Expected logger to be created, got nil")
	}
	if log.logFile == nil {
		t.Fatal("Expected log file to be set, got nil")
	}

	log.Info("test message")
	log.Close()

	content, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	if !strings.Contains(string(content), "test message") {
		t.Error("Expected log file to contain 'test message'")
	}
}

func TestLoggerClose(t *testing.T) {
	log := New(false)
	if err := log.Close(); err != nil {
		t.Errorf("Expected Close() to succeed, got error: %v", err)
	}

	tmpDir := t.TempDir()
	logFilePath := filepath.Join(tmpDir, "test.log")
	logWithFile, err := NewWithFile(false, logFilePath)
	if err != nil {
		t.Fatalf("Failed to create logger with file: %v", err)
	}
	if err := logWithFile.Close(); err != nil {
		t.Errorf("Expected Close() to succeed, got error: %v", err)
	}
}

func TestLoggerInfo(t *testing.T) {
	log := New(false)
	log.Info("test info message")
	log.Infof("test info formatted: %s", "value")
}

func TestLoggerSuccess(t *testing.T) {
	log := New(false)
	log.Success("test success message")
	log.Successf("test success formatted: %d", 42)
}

func TestLoggerWarning(t *testing.T) {
	log := New(false)
	log.Warning("test warning message")
	log.Warningf("test warning formatted: %v", true)
}

func TestLoggerError(t *testing.T) {
	log := New(false)
	log.Error("test error message")
	log.Errorf("test error formatted: %s", "error value")
}

func TestLoggerDebug(t *testing.T) {
	logNoDebug := New(false)
	logNoDebug.Debug("this should not be logged")

	logWithDebug := New(true)
	logWithDebug.Debug("this should be logged")
	logWithDebug.Debugf("formatted debug: %s", "value")
}

func TestLoggerStep(t *testing.T) {
	log := New(false)
	log.Step(1, "Test Step")
}

func TestGetTimestamp(t *testing.T) {
	timestamp := GetTimestamp()
	if len(timestamp) != 15 {
		t.Errorf("Expected timestamp length to be 15, got %d", len(timestamp))
	}
	if timestamp[8] != '-' {
		t.Error("Expected timestamp to have hyphen at position 8")
	}
}

func TestLoggerOutput(t *testing.T) {
	log := New(false)
	log.Info("test")
}

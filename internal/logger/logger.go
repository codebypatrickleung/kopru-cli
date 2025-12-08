// Package logger provides structured logging functionality for the Kopru CLI tool.
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// Logger provides structured logging with different severity levels.
type Logger struct {
	infoLog    *log.Logger
	successLog *log.Logger
	warningLog *log.Logger
	errorLog   *log.Logger
	debugLog   *log.Logger
	debug      bool
	logFile    *os.File
}

// New creates a new Logger instance.
func New(debug bool) *Logger {
	flags := log.Ldate | log.Ltime
	return &Logger{
		infoLog:    log.New(os.Stderr, "[INFO] ", flags),
		successLog: log.New(os.Stderr, "[DONE] ", flags),
		warningLog: log.New(os.Stderr, "[WARNING] ", flags),
		errorLog:   log.New(os.Stderr, "[ERROR] ", flags),
		debugLog:   log.New(os.Stderr, "[DEBUG] ", flags),
		debug:      debug,
	}
}

// NewWithFile creates a new Logger instance that writes to both console and a file.
func NewWithFile(debug bool, logFilePath string) (*Logger, error) {
	flags := log.Ldate | log.Ltime
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}
	multiWriter := io.MultiWriter(os.Stderr, logFile)
	return &Logger{
		infoLog:    log.New(multiWriter, "[INFO] ", flags),
		successLog: log.New(multiWriter, "[DONE] ", flags),
		warningLog: log.New(multiWriter, "[WARNING] ", flags),
		errorLog:   log.New(multiWriter, "[ERROR] ", flags),
		debugLog:   log.New(multiWriter, "[DEBUG] ", flags),
		debug:      debug,
		logFile:    logFile,
	}, nil
}

// Close closes the log file if one is open.
func (l *Logger) Close() error {
	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

// Info logs an informational message.
func (l *Logger) Info(msg string) {
	l.infoLog.Println(msg)
}

// Infof logs a formatted informational message.
func (l *Logger) Infof(format string, args ...interface{}) {
	l.infoLog.Printf(format, args...)
}

// Success logs a success message.
func (l *Logger) Success(msg string) {
	l.successLog.Println(msg)
}

// Successf logs a formatted success message.
func (l *Logger) Successf(format string, args ...interface{}) {
	l.successLog.Printf(format, args...)
}

// Warning logs a warning message.
func (l *Logger) Warning(msg string) {
	l.warningLog.Println(msg)
}

// Warningf logs a formatted warning message.
func (l *Logger) Warningf(format string, args ...interface{}) {
	l.warningLog.Printf(format, args...)
}

// Error logs an error message.
func (l *Logger) Error(msg string) {
	l.errorLog.Println(msg)
}

// Errorf logs a formatted error message.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.errorLog.Printf(format, args...)
}

// Debug logs a debug message (only if debug mode is enabled).
func (l *Logger) Debug(msg string) {
	if l.debug {
		l.debugLog.Println(msg)
	}
}

// Debugf logs a formatted debug message (only if debug mode is enabled).
func (l *Logger) Debugf(format string, args ...interface{}) {
	if l.debug {
		l.debugLog.Printf(format, args...)
	}
}

// Step logs a step header for workflow progress.
func (l *Logger) Step(stepNum int, description string) {
	l.Info("")
	l.Info("=========================================")
	l.Infof("Step %d: %s", stepNum, description)
	l.Info("=========================================")
}

// GetTimestamp returns a timestamp string in the format YYYYMMDD-HHMMSS.
func GetTimestamp() string {
	return time.Now().Format("20060102-150405")
}

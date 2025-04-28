package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// LogLevel defines the severity of a log entry
type LogLevel string

const (
	DEBUG LogLevel = "DEBUG"
	INFO  LogLevel = "INFO"
	WARN  LogLevel = "WARN"
	ERROR LogLevel = "ERROR"
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp time.Time         `json:"timestamp"`
	Level     LogLevel          `json:"level"`
	Message   string            `json:"message"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Logger handles audit logging
type Logger struct {
	filePath string
	file     *os.File
}

// NewLogger creates a new audit logger
func NewLogger(filePath string) *Logger {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return &Logger{filePath: filePath}
	}

	return &Logger{
		filePath: filePath,
		file:     file,
	}
}

// log writes a log entry at the specified level
func (l *Logger) log(level LogLevel, message string, metadata map[string]string) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Metadata:  metadata,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal log entry: %v\n", err)
		return
	}

	// Add newline
	jsonData = append(jsonData, '\n')

	// Write to console
	fmt.Fprintf(os.Stderr, "[%s] %s %s\n",
		level,
		entry.Timestamp.Format(time.RFC3339),
		message)

	// Write to file if available
	if l.file != nil {
		if _, err := l.file.Write(jsonData); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write to log file: %v\n", err)
		}
	}
}

// Debug logs a debug message
func (l *Logger) Debug(message string, metadata map[string]string) {
	l.log(DEBUG, message, metadata)
}

// Info logs an info message
func (l *Logger) Info(message string, metadata map[string]string) {
	l.log(INFO, message, metadata)
}

// Warn logs a warning message
func (l *Logger) Warn(message string, metadata map[string]string) {
	l.log(WARN, message, metadata)
}

// Error logs an error message
func (l *Logger) Error(message string, metadata map[string]string) {
	l.log(ERROR, message, metadata)
}

// Close closes the log file
func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}

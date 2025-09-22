package main

import (
	"testing"
)

func TestCustomLogFormat(t *testing.T) {
	// Test custom format for application logs
	customFormat := &LogFormat{
		Name:    "app-log",
		Pattern: `\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\] (\w+)\.(\w+): (.+)`,
		Fields: map[string]string{
			"method": "2",      // Log level (INFO, ERROR, etc)
			"path":   "3",      // Component name
			"status": "200",    // Default status for app logs
			"rt":     "0.0",    // Default response time
			"bytes":  "0",      // Default bytes
		},
		Default: map[string]any{
			"status": 200,
			"rt":     0.0,
			"bytes":  0,
		},
	}

	tests := []struct {
		name     string
		line     string
		expected map[string]any
		wantOK   bool
	}{
		{
			name: "valid app log",
			line: "[2025-01-15 14:30:00] INFO.UserController: User login successful",
			expected: map[string]any{
				"host":   "test-host",
				"method": "INFO",
				"path":   "UserController",
				"status": 200,
				"rt":     0.0,
				"bytes":  0,
				"src":    "/var/log/app.log",
			},
			wantOK: true,
		},
		{
			name: "error log",
			line: "[2025-01-15 14:31:00] ERROR.DatabaseManager: Connection timeout",
			expected: map[string]any{
				"host":   "test-host",
				"method": "ERROR",
				"path":   "DatabaseManager",
				"status": 200,
				"rt":     0.0,
				"bytes":  0,
				"src":    "/var/log/app.log",
			},
			wantOK: true,
		},
		{
			name:   "invalid format",
			line:   "Invalid log line format",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, ok := parseCustomFormat(tt.line, "/var/log/app.log", "test-host", customFormat)

			if ok != tt.wantOK {
				t.Errorf("parseCustomFormat() ok = %v, want %v", ok, tt.wantOK)
				return
			}

			if !tt.wantOK {
				return
			}

			for key, expected := range tt.expected {
				if actual, exists := event[key]; !exists || actual != expected {
					t.Errorf("Expected %s=%v, got %v", key, expected, actual)
				}
			}
		})
	}
}

func TestCustomFormatWithNumbers(t *testing.T) {
	// Test custom format with numeric fields
	customFormat := &LogFormat{
		Name:    "numeric-log",
		Pattern: `^(\d+\.\d+\.\d+\.\d+) - (\w+) (\S+) (\d+) (\d+) ([0-9.]+)`,
		Fields: map[string]string{
			"ip":     "1",
			"method": "2",
			"path":   "3",
			"status": "4",
			"bytes":  "5",
			"rt":     "6",
		},
	}

	line := "192.168.1.100 - GET /api/users 200 1024 0.156"
	event, ok := parseCustomFormat(line, "/var/log/custom.log", "test-host", customFormat)

	if !ok {
		t.Fatal("Expected parseCustomFormat to succeed")
	}

	// Check that numeric fields are properly converted
	if status, ok := event["status"].(int); !ok || status != 200 {
		t.Errorf("Expected status to be int 200, got %v (%T)", event["status"], event["status"])
	}

	if bytes, ok := event["bytes"].(int64); !ok || bytes != 1024 {
		t.Errorf("Expected bytes to be int64 1024, got %v (%T)", event["bytes"], event["bytes"])
	}

	if rt, ok := event["rt"].(float64); !ok || rt != 0.156 {
		t.Errorf("Expected rt to be float64 0.156, got %v (%T)", event["rt"], event["rt"])
	}
}

func TestCustomFormatWithSpecialFields(t *testing.T) {
	// Test custom format with hostname and filename placeholders
	customFormat := &LogFormat{
		Name:    "special-fields",
		Pattern: `(\w+): (.+)`,
		Fields: map[string]string{
			"method": "1",
			"path":   "2",
			"host":   "hostname",
			"src":    "filename",
		},
		Default: map[string]any{
			"status": 200,
			"rt":     0.0,
			"bytes":  0,
		},
	}

	line := "INFO: Application started"
	event, ok := parseCustomFormat(line, "/var/log/app.log", "my-server", customFormat)

	if !ok {
		t.Fatal("Expected parseCustomFormat to succeed")
	}

	if event["host"] != "my-server" {
		t.Errorf("Expected host to be 'my-server', got %v", event["host"])
	}

	if event["src"] != "/var/log/app.log" {
		t.Errorf("Expected src to be '/var/log/app.log', got %v", event["src"])
	}
}
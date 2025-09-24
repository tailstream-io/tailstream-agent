package main

import (
	"os"
	"testing"
)

func TestNeedsSetup(t *testing.T) {
	tests := []struct {
		name           string
		configContent  string
		envKeySet      bool
		expectedResult bool
	}{
		{
			name:           "valid config with streams",
			configContent:  "env: production\nkey: test-token\nstreams:\n  - name: test\n    stream_id: test-123\n    paths: ['/var/log/*.log']",
			envKeySet:      false,
			expectedResult: false,
		},
		{
			name:           "valid legacy config",
			configContent:  "env: production\nkey: test-token\nship:\n  url: https://example.com\n  stream_id: test-123",
			envKeySet:      false,
			expectedResult: false,
		},
		{
			name:           "environment key set",
			configContent:  "",
			envKeySet:      true,
			expectedResult: false,
		},
		{
			name:           "incomplete config file",
			configContent:  "test: config",
			envKeySet:      false,
			expectedResult: true,
		},
		{
			name:           "no config and no env key",
			configContent:  "",
			envKeySet:      false,
			expectedResult: true,
		},
		{
			name:           "config with key but no streams",
			configContent:  "env: production\nkey: test-token",
			envKeySet:      false,
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save current directory and change to temp dir
			originalDir, _ := os.Getwd()
			tmpDir := t.TempDir()
			os.Chdir(tmpDir)
			defer os.Chdir(originalDir)

			// Create config file if needed
			if tt.configContent != "" {
				os.WriteFile("tailstream.yaml", []byte(tt.configContent), 0600)
			}

			// Set environment variable if needed
			if tt.envKeySet {
				os.Setenv("TAILSTREAM_KEY", "test-key")
				defer os.Unsetenv("TAILSTREAM_KEY")
			} else {
				os.Unsetenv("TAILSTREAM_KEY")
			}

			result := needsSetup()
			if result != tt.expectedResult {
				t.Errorf("Expected needsSetup() to return %v, got %v", tt.expectedResult, result)
			}
		})
	}
}

func TestSetupWizardValidation(t *testing.T) {
	// Note: Testing the actual setupWizard() function is challenging because it uses
	// fmt.Scanln() for user input. In a real application, we'd want to refactor
	// this to accept input/output interfaces for better testability.

	// For now, we can test the validation logic by extracting it into a separate function
	// or by testing the overall behavior with mock input (which is complex in Go).

	// This is a placeholder to demonstrate what we'd want to test:
	t.Skip("Setup wizard requires user interaction - would need refactoring for proper testing")
}
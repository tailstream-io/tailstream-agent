package main

import (
	"os"
	"testing"
)

func TestNeedsSetup(t *testing.T) {
	tests := []struct {
		name           string
		configExists   bool
		envKeySet      bool
		expectedResult bool
	}{
		{
			name:           "config file exists",
			configExists:   true,
			envKeySet:      false,
			expectedResult: false,
		},
		{
			name:           "environment key set",
			configExists:   false,
			envKeySet:      true,
			expectedResult: false,
		},
		{
			name:           "neither config nor env key",
			configExists:   false,
			envKeySet:      false,
			expectedResult: true,
		},
		{
			name:           "both config and env key",
			configExists:   true,
			envKeySet:      true,
			expectedResult: false,
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
			if tt.configExists {
				os.WriteFile("tailstream.yaml", []byte("test: config"), 0600)
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
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetDefaultConfigPath(t *testing.T) {
	// This test verifies the config path logic without creating actual system files
	// In real usage, system paths would be checked for existence first

	// Create a temporary directory structure to simulate system paths
	tempDir := t.TempDir()

	// Test 1: No system configs exist - should fall back to local
	path := getDefaultConfigPath()
	expected := "tailstream.yaml"
	if path != expected {
		t.Errorf("Expected fallback to %s, got %s", expected, path)
	}

	// Test 2: Create a local config in temp dir and verify it's used
	tempLocalConfig := filepath.Join(tempDir, "tailstream.yaml")
	if err := os.WriteFile(tempLocalConfig, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create temp config: %v", err)
	}

	// Change to temp directory
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	path = getDefaultConfigPath()
	if path != "tailstream.yaml" {
		t.Errorf("Expected local config to be found, got %s", path)
	}

	// Clean up
	os.Remove(tempLocalConfig)
}

func TestGetSystemConfigPath(t *testing.T) {
	// Test the system config path selection logic
	path := getSystemConfigPath()

	// The exact path depends on the OS, but it should be one of the expected system paths
	expectedPaths := []string{
		"/etc/tailstream/agent.yaml",           // Linux
		"/usr/local/etc/tailstream/agent.yaml", // macOS
		"tailstream.yaml",                      // Windows/other fallback
	}

	found := false
	for _, expected := range expectedPaths {
		if path == expected {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("System config path %s not in expected paths %v", path, expectedPaths)
	}
}

func TestCreateConfigDir(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "nested", "dir", "config.yaml")

	// Test directory creation
	err := createConfigDir(configPath)
	if err != nil {
		t.Fatalf("createConfigDir failed: %v", err)
	}

	// Verify directory was created
	dirPath := filepath.Dir(configPath)
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		t.Errorf("Directory %s was not created", dirPath)
	}

	// Test that it doesn't fail if directory already exists
	err = createConfigDir(configPath)
	if err != nil {
		t.Errorf("createConfigDir should not fail for existing directory: %v", err)
	}
}
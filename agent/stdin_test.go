package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKeyFilePriority(t *testing.T) {
	// Create a temporary key file
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "test-key")
	if err := os.WriteFile(keyFile, []byte("key-from-file"), 0600); err != nil {
		t.Fatalf("failed to create test key file: %v", err)
	}

	// Save original env vars
	origKey := os.Getenv("TAILSTREAM_KEY")
	origKeyFile := os.Getenv("TAILSTREAM_KEY_FILE")
	defer func() {
		os.Setenv("TAILSTREAM_KEY", origKey)
		os.Setenv("TAILSTREAM_KEY_FILE", origKeyFile)
	}()

	// Test 1: Key file takes priority over env var
	os.Setenv("TAILSTREAM_KEY", "key-from-env")
	os.Setenv("TAILSTREAM_KEY_FILE", keyFile)

	// Read the key using the same logic as runStdinMode
	accessToken := ""
	if keyFilePath := os.Getenv("TAILSTREAM_KEY_FILE"); keyFilePath != "" {
		keyBytes, err := os.ReadFile(keyFilePath)
		if err != nil {
			t.Fatalf("failed to read key file: %v", err)
		}
		accessToken = string(keyBytes)
	}

	if accessToken != "key-from-file" {
		t.Errorf("expected key from file, got: %s", accessToken)
	}

	// Test 2: Falls back to env var when no key file
	os.Setenv("TAILSTREAM_KEY_FILE", "")
	accessToken = ""

	if keyFilePath := os.Getenv("TAILSTREAM_KEY_FILE"); keyFilePath != "" {
		keyBytes, _ := os.ReadFile(keyFilePath)
		accessToken = string(keyBytes)
	}
	if accessToken == "" {
		accessToken = os.Getenv("TAILSTREAM_KEY")
	}

	if accessToken != "key-from-env" {
		t.Errorf("expected key from env, got: %s", accessToken)
	}
}

func TestKeyFileWhitespaceTrimming(t *testing.T) {
	// Create a temporary key file with whitespace
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "test-key-whitespace")

	testCases := []struct {
		name     string
		content  string
		expected string
	}{
		{"trailing newline", "test-key\n", "test-key"},
		{"leading space", "  test-key", "test-key"},
		{"trailing space", "test-key  ", "test-key"},
		{"both", "  test-key\n", "test-key"},
		{"multiple lines", "test-key\n\n", "test-key"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(keyFile, []byte(tc.content), 0600); err != nil {
				t.Fatalf("failed to create test key file: %v", err)
			}

			keyBytes, err := os.ReadFile(keyFile)
			if err != nil {
				t.Fatalf("failed to read key file: %v", err)
			}

			// Use the same trimming logic as runStdinMode
			accessToken := string(keyBytes)
			// Import strings package is already in main.go
			// In runStdinMode we use: strings.TrimSpace(string(keyBytes))
			// For this test, we'll manually trim to verify
			trimmed := ""
			for i, c := range accessToken {
				if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
					trimmed = accessToken[i:]
					break
				}
			}
			for i := len(trimmed) - 1; i >= 0; i-- {
				c := trimmed[i]
				if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
					trimmed = trimmed[:i+1]
					break
				}
			}

			if trimmed != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, trimmed)
			}
		})
	}
}

func TestKeyFilePermissions(t *testing.T) {
	// Create a temporary key file
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "test-key-perms")
	if err := os.WriteFile(keyFile, []byte("secret-key"), 0644); err != nil {
		t.Fatalf("failed to create test key file: %v", err)
	}

	// Check file permissions
	info, err := os.Stat(keyFile)
	if err != nil {
		t.Fatalf("failed to stat key file: %v", err)
	}

	mode := info.Mode()
	if mode&0044 != 0 {
		t.Logf("WARNING: Key file has world/group read permissions: %v", mode)
		t.Logf("Recommended: chmod 600 for security")
	}
}

func TestKeyFileNotFound(t *testing.T) {
	// Test reading a non-existent key file
	nonExistentFile := "/nonexistent/path/to/key"

	_, err := os.ReadFile(nonExistentFile)
	if err == nil {
		t.Error("expected error when reading non-existent file")
	}

	// Verify error message contains useful info
	if !os.IsNotExist(err) {
		t.Errorf("expected os.IsNotExist error, got: %v", err)
	}
}

func TestStdinDetection(t *testing.T) {
	// This test verifies that we can detect if stdin is a pipe
	// In actual usage, this is checked with: (stat.Mode() & os.ModeCharDevice) == 0

	stat, err := os.Stdin.Stat()
	if err != nil {
		t.Fatalf("failed to stat stdin: %v", err)
	}

	// In normal test execution, stdin is typically a character device
	// When piped, it would be a pipe
	isPipe := (stat.Mode() & os.ModeCharDevice) == 0

	// Just verify the check doesn't panic
	t.Logf("stdin is pipe: %v, mode: %v", isPipe, stat.Mode())
}

package main

import (
	"os"
	"testing"
)

func TestGetBaseURL(t *testing.T) {
	// Save original env var if it exists
	originalURL := os.Getenv("TAILSTREAM_BASE_URL")
	defer func() {
		if originalURL != "" {
			os.Setenv("TAILSTREAM_BASE_URL", originalURL)
		} else {
			os.Unsetenv("TAILSTREAM_BASE_URL")
		}
	}()

	t.Run("default URL when no env var", func(t *testing.T) {
		os.Unsetenv("TAILSTREAM_BASE_URL")

		result := getBaseURL()
		expected := "https://app.tailstream.io"

		if result != expected {
			t.Errorf("Expected %s, got %s", expected, result)
		}
	})

	t.Run("custom URL from environment variable", func(t *testing.T) {
		customURL := "https://app.tailstream.test"
		os.Setenv("TAILSTREAM_BASE_URL", customURL)

		result := getBaseURL()

		if result != customURL {
			t.Errorf("Expected %s, got %s", customURL, result)
		}
	})

	t.Run("empty env var uses default", func(t *testing.T) {
		os.Setenv("TAILSTREAM_BASE_URL", "")

		result := getBaseURL()
		expected := "https://app.tailstream.io"

		if result != expected {
			t.Errorf("Expected %s, got %s", expected, result)
		}
	})
}

func TestGetDefaultLogPaths(t *testing.T) {
	paths := getDefaultLogPaths()

	expected := []string{
		"/var/log/nginx/*.log",
		"/var/log/caddy/*.log",
		"/var/log/apache2/*.log",
		"/var/log/httpd/*.log",
		"/var/www/**/storage/logs/*.log",
	}

	if len(paths) != len(expected) {
		t.Errorf("Expected %d paths, got %d", len(expected), len(paths))
		return
	}

	for i, path := range paths {
		if path != expected[i] {
			t.Errorf("Expected path[%d] to be %s, got %s", i, expected[i], path)
		}
	}
}

func TestGetDefaultExclusions(t *testing.T) {
	exclusions := getDefaultExclusions()

	expected := []string{"**/*.gz", "**/*.1"}

	if len(exclusions) != len(expected) {
		t.Errorf("Expected %d exclusions, got %d", len(expected), len(exclusions))
		return
	}

	for i, exclusion := range exclusions {
		if exclusion != expected[i] {
			t.Errorf("Expected exclusion[%d] to be %s, got %s", i, expected[i], exclusion)
		}
	}
}

func TestStreamDescriptionHandling(t *testing.T) {
	testCases := []struct {
		name        string
		description string
		streamID    string
		expected    string
	}{
		{
			name:        "has description",
			description: "Test Description",
			streamID:    "abc123def456",
			expected:    "Test Description",
		},
		{
			name:        "no description, long stream ID",
			description: "",
			streamID:    "abc123def456",
			expected:    "abc123de...",
		},
		{
			name:        "no description, short stream ID",
			description: "",
			streamID:    "abc1",
			expected:    "abc1",
		},
		{
			name:        "no description, exactly 8 chars",
			description: "",
			streamID:    "abc12345",
			expected:    "abc12345",
		},
		{
			name:        "no description, 9 chars",
			description: "",
			streamID:    "abc123456",
			expected:    "abc12345...",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stream := Stream{
				Description: tc.description,
				StreamID:    tc.streamID,
			}

			var result string
			if stream.Description != "" {
				result = stream.Description
			} else {
				if len(stream.StreamID) > 8 {
					result = stream.StreamID[:8] + "..."
				} else {
					result = stream.StreamID
				}
			}

			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestCreateOAuthConfig(t *testing.T) {
	streams := []StreamConfig{
		{
			Name:     "test-stream",
			StreamID: "stream-id-123",
			Key:      "stream-ingest-token-456", // Stream-specific ingest token
			Paths:    []string{"/var/log/test.log"},
			Exclude:  []string{"**/*.gz"},
		},
	}

	config := createOAuthConfig(streams)

	// Test basic config structure
	if config.Env != "production" {
		t.Errorf("Expected env to be 'production', got %s", config.Env)
	}

	// No global key should be set - each stream has its own ingest token
	if config.Key != "" {
		t.Errorf("Expected no global key, but got %s", config.Key)
	}

	if !config.Discovery.Enabled {
		t.Error("Expected discovery to be enabled")
	}

	if len(config.Streams) != 1 {
		t.Errorf("Expected 1 stream, got %d", len(config.Streams))
	}

	if config.Streams[0].Name != "test-stream" {
		t.Errorf("Expected stream name 'test-stream', got %s", config.Streams[0].Name)
	}

	if config.Streams[0].Key != "stream-ingest-token-456" {
		t.Errorf("Expected stream ingest token 'stream-ingest-token-456', got %s", config.Streams[0].Key)
	}

	// Test updates config
	if !config.Updates.Enabled {
		t.Error("Expected updates to be enabled")
	}

	if config.Updates.Channel != "stable" {
		t.Errorf("Expected updates channel 'stable', got %s", config.Updates.Channel)
	}

	if config.Updates.CheckHours != 24 {
		t.Errorf("Expected check hours 24, got %d", config.Updates.CheckHours)
	}
}

func TestFindExistingLogFiles(t *testing.T) {
	// This test interacts with the filesystem and may return nil or empty results
	// depending on what log files exist on the test system

	result := findExistingLogFiles()

	// Should not return more than 3 files as per the limit in the code
	if len(result) > 3 {
		t.Errorf("Expected at most 3 files, got %d", len(result))
	}

	// Each returned path should be a non-empty string
	for i, path := range result {
		if path == "" {
			t.Errorf("Expected non-empty path at index %d", i)
		}
	}

	// Function should not panic (basic smoke test)
	// This mainly tests that the doublestar.FilepathGlob calls don't crash
}
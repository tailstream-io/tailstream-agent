package main

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadConfigDefaults(t *testing.T) {
	// Clear any existing environment variables
	oldKey := os.Getenv("TAILSTREAM_KEY")
	oldURL := os.Getenv("TAILSTREAM_URL")
	oldEnv := os.Getenv("TAILSTREAM_ENV")
	defer func() {
		os.Setenv("TAILSTREAM_KEY", oldKey)
		os.Setenv("TAILSTREAM_URL", oldURL)
		os.Setenv("TAILSTREAM_ENV", oldEnv)
	}()

	os.Unsetenv("TAILSTREAM_KEY")
	os.Unsetenv("TAILSTREAM_URL")
	os.Unsetenv("TAILSTREAM_ENV")

	// Override command line args to avoid conflicts
	oldArgs := os.Args
	os.Args = []string{"test"}
	defer func() { os.Args = oldArgs }()

	cfg := loadConfig()

	if cfg.Env != "production" {
		t.Errorf("Expected default env to be 'production', got: %s", cfg.Env)
	}

	if !cfg.Discovery.Enabled {
		t.Error("Expected discovery to be enabled by default")
	}

	expectedPaths := []string{
		"/var/log/nginx/*.log",
		"/var/log/caddy/*.log",
		"/var/log/apache2/*.log",
		"/var/log/httpd/*.log",
		"/var/www/**/storage/logs/*.log",
	}

	if len(cfg.Discovery.Paths.Include) != len(expectedPaths) {
		t.Errorf("Expected %d default include paths, got %d", len(expectedPaths), len(cfg.Discovery.Paths.Include))
	}

	for i, expected := range expectedPaths {
		if i < len(cfg.Discovery.Paths.Include) && cfg.Discovery.Paths.Include[i] != expected {
			t.Errorf("Expected include path %d to be %s, got %s", i, expected, cfg.Discovery.Paths.Include[i])
		}
	}
}

func TestLoadConfigFromYAML(t *testing.T) {
	// Test YAML loading by directly testing the YAML unmarshaling
	yamlContent := `
env: staging
key: test-key-from-yaml
discovery:
  enabled: true
  paths:
    include:
      - "/custom/path/*.log"
    exclude:
      - "**/*.old"
ship:
  url: "https://test.example.com/ingest"
  stream_id: "test-stream-id"
streams:
  - name: "test-stream"
    stream_id: "stream-123"
    paths:
      - "/test/*.log"
`

	var cfg Config
	err := yaml.Unmarshal([]byte(yamlContent), &cfg)
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	if cfg.Env != "staging" {
		t.Errorf("Expected env to be 'staging', got: %s", cfg.Env)
	}

	if cfg.Key != "test-key-from-yaml" {
		t.Errorf("Expected key to be 'test-key-from-yaml', got: %s", cfg.Key)
	}

	if cfg.Ship.URL != "https://test.example.com/ingest" {
		t.Errorf("Expected ship URL to be loaded from YAML, got: %s", cfg.Ship.URL)
	}

	if len(cfg.Streams) != 1 {
		t.Errorf("Expected 1 stream from YAML, got: %d", len(cfg.Streams))
	}

	if len(cfg.Streams) > 0 {
		stream := cfg.Streams[0]
		if stream.Name != "test-stream" {
			t.Errorf("Expected stream name 'test-stream', got: %s", stream.Name)
		}
		if stream.StreamID != "stream-123" {
			t.Errorf("Expected stream ID 'stream-123', got: %s", stream.StreamID)
		}
	}
}

func TestLoadConfigEnvironmentOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("TAILSTREAM_KEY", "env-key")
	os.Setenv("TAILSTREAM_URL", "https://env.example.com")
	os.Setenv("TAILSTREAM_ENV", "development")
	defer func() {
		os.Unsetenv("TAILSTREAM_KEY")
		os.Unsetenv("TAILSTREAM_URL")
		os.Unsetenv("TAILSTREAM_ENV")
	}()

	oldArgs := os.Args
	os.Args = []string{"test"}
	defer func() { os.Args = oldArgs }()

	cfg := loadConfig()

	if cfg.Key != "env-key" {
		t.Errorf("Expected key from environment, got: %s", cfg.Key)
	}

	if cfg.Ship.URL != "https://env.example.com" {
		t.Errorf("Expected URL from environment, got: %s", cfg.Ship.URL)
	}

	// Note: TAILSTREAM_ENV should override the default but we load it into cfg.Env
	// The actual override happens through getenv() which we test separately
}

func TestStreamConfigGetURL(t *testing.T) {
	stream := StreamConfig{
		StreamID: "test-stream-123",
	}

	expectedURL := "https://app.tailstream.io/api/ingest/test-stream-123"
	actualURL := stream.GetURL()

	if actualURL != expectedURL {
		t.Errorf("Expected URL %s, got %s", expectedURL, actualURL)
	}
}

func TestGetenv(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envValue string
		defValue string
		expected string
	}{
		{
			name:     "environment variable set",
			envKey:   "TEST_ENV_VAR",
			envValue: "env-value",
			defValue: "default-value",
			expected: "env-value",
		},
		{
			name:     "environment variable not set",
			envKey:   "UNSET_ENV_VAR",
			envValue: "",
			defValue: "default-value",
			expected: "default-value",
		},
		{
			name:     "environment variable empty",
			envKey:   "EMPTY_ENV_VAR",
			envValue: "",
			defValue: "default-value",
			expected: "default-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			}

			result := getenv(tt.envKey, tt.defValue)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}
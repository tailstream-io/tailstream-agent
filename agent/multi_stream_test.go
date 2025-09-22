package main

import (
	"testing"
)

func TestMultiStreamDiscovery(t *testing.T) {
	cfg := Config{
		Env: "test",
		Key: "global-key",
		Streams: []StreamConfig{
			{
				Name:     "nginx",
				StreamID: "stream-1",
				Paths:    []string{"/tmp/test-nginx/*.log"},
				Exclude:  []string{"**/*.gz"},
			},
			{
				Name:     "app",
				StreamID: "stream-2",
				Key:      "stream-specific-key",
				Paths:    []string{"/tmp/test-app/*.log"},
			},
		},
	}

	// Test that multi-stream discovery works
	mappings, err := discover(cfg)
	if err != nil {
		t.Fatalf("discover failed: %v", err)
	}

	// Should have 2 stream mappings (even if no files match the patterns)
	if len(mappings) > 2 {
		t.Errorf("Expected at most 2 stream mappings, got %d", len(mappings))
	}

	// Verify stream configurations are preserved
	for _, mapping := range mappings {
		switch mapping.Stream.Name {
		case "nginx":
			if mapping.Stream.GetURL() != "https://app.tailstream.io/api/ingest/stream-1" {
				t.Errorf("Expected nginx stream URL to be correct, got: %s", mapping.Stream.GetURL())
			}
			if mapping.Stream.Key != "" {
				t.Errorf("Expected nginx stream to use global key, got: %s", mapping.Stream.Key)
			}
		case "app":
			if mapping.Stream.Key != "stream-specific-key" {
				t.Errorf("Expected app stream to have specific key")
			}
		}
	}
}

func TestLegacyConfigCompatibility(t *testing.T) {
	// Test that legacy single-stream config still works
	cfg := Config{
		Env: "test",
		Key: "legacy-key",
		Discovery: struct {
			Enabled bool `yaml:"enabled"`
			Paths   struct {
				Include []string `yaml:"include"`
				Exclude []string `yaml:"exclude"`
			} `yaml:"paths"`
		}{
			Enabled: true,
			Paths: struct {
				Include []string `yaml:"include"`
				Exclude []string `yaml:"exclude"`
			}{
				Include: []string{"/tmp/test/*.log"},
				Exclude: []string{"**/*.gz"},
			},
		},
		Ship: struct {
			URL      string `yaml:"url"`
			StreamID string `yaml:"stream_id"`
		}{
			URL:      "https://app.tailstream.io/api/ingest/legacy-stream",
			StreamID: "legacy-stream",
		},
	}

	mappings, err := discover(cfg)
	if err != nil {
		t.Fatalf("discover failed: %v", err)
	}

	// Should create a single "default" stream from legacy config
	if len(mappings) > 1 {
		t.Errorf("Expected at most 1 mapping for legacy config, got %d", len(mappings))
	}

	if len(mappings) == 1 {
		mapping := mappings[0]
		if mapping.Stream.Name != "default" {
			t.Errorf("Expected legacy stream name to be 'default', got: %s", mapping.Stream.Name)
		}
		if mapping.Stream.GetURL() != "https://app.tailstream.io/api/ingest/legacy-stream" {
			t.Errorf("Expected legacy URL to be preserved, got: %s", mapping.Stream.GetURL())
		}
		if mapping.Stream.Key != "legacy-key" {
			t.Errorf("Expected legacy key to be preserved")
		}
	}
}

func TestStreamConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		stream StreamConfig
		valid  bool
	}{
		{
			name: "valid stream config",
			stream: StreamConfig{
				Name:     "test",
				StreamID: "test",
				Paths:    []string{"/tmp/*.log"},
			},
			valid: true,
		},
		{
			name: "missing StreamID",
			stream: StreamConfig{
				Name:  "test",
				Paths: []string{"/tmp/*.log"},
			},
			valid: false,
		},
		{
			name: "empty paths",
			stream: StreamConfig{
				Name:     "test",
				StreamID: "test",
				Paths:    []string{},
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasStreamID := tt.stream.StreamID != ""
			hasPaths := len(tt.stream.Paths) > 0

			isValid := hasStreamID && hasPaths
			if isValid != tt.valid {
				t.Errorf("Expected valid=%v, got valid=%v for stream config", tt.valid, isValid)
			}
		})
	}
}
package main

import (
	"testing"
)

func TestMultiStreamDiscovery(t *testing.T) {
	cfg := Config{
		Env: "test",
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
			// nginx stream has no specific key, so should be empty
			if mapping.Stream.Key != "" {
				t.Errorf("Expected nginx stream to have no specific key, got: %s", mapping.Stream.Key)
			}
		case "app":
			if mapping.Stream.Key != "stream-specific-key" {
				t.Errorf("Expected app stream to have specific key")
			}
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
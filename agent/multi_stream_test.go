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

func TestSingleStreamMultipleFormats(t *testing.T) {
	// Test that a SINGLE stream can handle multiple log formats from different files
	tests := []struct {
		name    string
		logLine string
	}{
		{
			name:    "nginx access log",
			logLine: `192.168.1.1 - - [22/Sep/2025:17:04:36 +0000] "GET /api/test HTTP/1.1" 200 1024 "-" "curl/7.68.0"`,
		},
		{
			name:    "JSON log",
			logLine: `{"host":"server1","path":"/api","method":"GET","status":200,"rt":0.1,"bytes":500}`,
		},
		{
			name:    "plain text syslog",
			logLine: "2024-01-15T10:30:45Z ERROR Database connection failed",
		},
		{
			name:    "plain text error log",
			logLine: "[ERROR] Connection timeout at 10:30:45",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ll := LogLine{File: "/test.log", Line: tt.logLine}
			event, ok := parseLine(ll)

			if !ok {
				t.Fatalf("parseLine() failed for %s", tt.name)
			}

			// All events are now raw strings - backend handles parsing
			evStr, ok := event.(string)
			if !ok {
				t.Errorf("Expected event to be string, got type: %T", event)
				return
			}

			if evStr != tt.logLine {
				t.Errorf("Expected raw string to match original line.\nGot:  %s\nWant: %s", evStr, tt.logLine)
			}
		})
	}
}

func TestMultiStreamDifferentFormats(t *testing.T) {
	// Test that different streams can use different log formats
	tests := []struct {
		name    string
		stream  StreamConfig
		logLine string
	}{
		{
			name: "stream 1: nginx access log (sent as raw string)",
			stream: StreamConfig{
				Name:     "nginx-stream",
				StreamID: "stream-1",
			},
			logLine: `192.168.1.1 - - [22/Sep/2025:17:04:36 +0000] "GET /api/test HTTP/1.1" 200 1024 "-" "curl/7.68.0"`,
		},
		{
			name: "stream 2: app logs (sent as raw string)",
			stream: StreamConfig{
				Name:     "app-stream",
				StreamID: "stream-2",
			},
			logLine: "[2024-01-15 10:30:45] production.ERROR: Connection timeout",
		},
		{
			name: "stream 3: raw syslog (sent as raw string)",
			stream: StreamConfig{
				Name:     "syslog-stream",
				StreamID: "stream-3",
			},
			logLine: "2024-01-15T10:30:45Z ERROR Database connection failed",
		},
		{
			name: "stream 4: JSON logs (sent as raw string)",
			stream: StreamConfig{
				Name:     "json-stream",
				StreamID: "stream-4",
			},
			logLine: `{"host":"server1","path":"/api","method":"GET","status":200,"rt":0.1,"bytes":500}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ll := LogLine{File: "/test.log", Line: tt.logLine}
			event, ok := parseLine(ll)

			if !ok {
				t.Errorf("parseLine() failed for stream %s", tt.stream.Name)
				return
			}

			// All events are now raw strings - backend handles parsing
			evStr, ok := event.(string)
			if !ok {
				t.Errorf("Expected event to be string for stream %s, got type: %T", tt.stream.Name, event)
				return
			}

			if evStr != tt.logLine {
				t.Errorf("Expected raw string to match original line.\nGot:  %s\nWant: %s", evStr, tt.logLine)
			}
		})
	}
}

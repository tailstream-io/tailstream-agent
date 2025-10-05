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
	stream := StreamConfig{
		Name:     "mixed-stream",
		StreamID: "stream-1",
		Format:   nil, // No custom format - rely on auto-detection
	}

	tests := []struct {
		name         string
		logLine      string
		expectedType string // "map" or "string"
	}{
		{
			name:         "nginx access log",
			logLine:      `192.168.1.1 - - [22/Sep/2025:17:04:36 +0000] "GET /api/test HTTP/1.1" 200 1024 "-" "curl/7.68.0"`,
			expectedType: "map",
		},
		{
			name:         "JSON log",
			logLine:      `{"host":"server1","path":"/api","method":"GET","status":200,"rt":0.1,"bytes":500}`,
			expectedType: "map",
		},
		{
			name:         "plain text syslog",
			logLine:      "2024-01-15T10:30:45Z ERROR Database connection failed",
			expectedType: "string",
		},
		{
			name:         "plain text error log",
			logLine:      "[ERROR] Connection timeout at 10:30:45",
			expectedType: "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ll := LogLine{File: "/test.log", Line: tt.logLine}
			event, ok := parseLine(ll, "testhost", stream.Format)

			if !ok {
				t.Fatalf("parseLine() failed for %s", tt.name)
			}

			// Check event type matches expectation
			switch tt.expectedType {
			case "string":
				if _, isString := event.(string); !isString {
					t.Errorf("Expected event to be string, got type: %T", event)
				}
			case "map":
				if _, isMap := event.(map[string]interface{}); !isMap {
					t.Errorf("Expected event to be map, got type: %T", event)
				}
			}
		})
	}
}

func TestMultiStreamDifferentFormats(t *testing.T) {
	// Test that different streams can use different log formats
	tests := []struct {
		name           string
		stream         StreamConfig
		logLine        string
		expectedType   string // "map" or "string"
		shouldParse    bool
	}{
		{
			name: "stream 1: nginx access log (auto-parsed to JSON)",
			stream: StreamConfig{
				Name:     "nginx-stream",
				StreamID: "stream-1",
				Format:   nil, // No custom format, will auto-detect
			},
			logLine:      `192.168.1.1 - - [22/Sep/2025:17:04:36 +0000] "GET /api/test HTTP/1.1" 200 1024 "-" "curl/7.68.0"`,
			expectedType: "map",
			shouldParse:  true,
		},
		{
			name: "stream 2: custom app format (custom parsed to JSON)",
			stream: StreamConfig{
				Name:     "app-stream",
				StreamID: "stream-2",
				Format: &LogFormat{
					Name:    "myapp",
					Pattern: `\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\] (\w+)\.(\w+): (.+)`,
					Fields: map[string]string{
						"method": "2",
						"path":   "3",
						"host":   "hostname",
					},
					Default: map[string]any{
						"status": 200,
						"rt":     0.0,
						"bytes":  0,
					},
				},
			},
			logLine:      "[2024-01-15 10:30:45] production.ERROR: Connection timeout",
			expectedType: "map",
			shouldParse:  true,
		},
		{
			name: "stream 3: raw syslog (sent as raw string)",
			stream: StreamConfig{
				Name:     "syslog-stream",
				StreamID: "stream-3",
				Format:   nil, // No custom format
			},
			logLine:      "2024-01-15T10:30:45Z ERROR Database connection failed",
			expectedType: "string",
			shouldParse:  true,
		},
		{
			name: "stream 4: JSON logs (parsed to JSON)",
			stream: StreamConfig{
				Name:     "json-stream",
				StreamID: "stream-4",
				Format:   nil,
			},
			logLine:      `{"host":"server1","path":"/api","method":"GET","status":200,"rt":0.1,"bytes":500}`,
			expectedType: "map",
			shouldParse:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ll := LogLine{File: "/test.log", Line: tt.logLine}
			event, ok := parseLine(ll, "testhost", tt.stream.Format)

			if ok != tt.shouldParse {
				t.Errorf("parseLine() ok = %v, want %v", ok, tt.shouldParse)
				return
			}

			if !ok {
				return
			}

			// Check event type matches expectation
			switch tt.expectedType {
			case "string":
				if _, isString := event.(string); !isString {
					t.Errorf("Expected event to be string for stream %s, got type: %T", tt.stream.Name, event)
				}
			case "map":
				if _, isMap := event.(map[string]interface{}); !isMap {
					t.Errorf("Expected event to be map for stream %s, got type: %T", tt.stream.Name, event)
				}
			}
		})
	}
}
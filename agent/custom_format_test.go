package main

import (
	"testing"
)


func TestCustomAccessLogFormats(t *testing.T) {
	tests := []struct {
		name     string
		format   *LogFormat
		line     string
		expected map[string]interface{}
	}{
		{
			name: "nginx custom format with upstream time",
			format: &LogFormat{
				Name:    "nginx-upstream",
				Pattern: `^(\S+) - - \[([^\]]+)\] "(\S+) ([^"]*) HTTP/[^"]*" (\d+) (\d+) "([^"]*)" "([^"]*)" ([0-9.]+) ([0-9.]+)`,
				Fields: map[string]string{
					"src":            "1",
					"method":         "3",
					"path":           "4",
					"status":         "5",
					"bytes":          "6",
					"user_agent":     "8",
					"rt":             "9",
					"upstream_time":  "10",
				},
			},
			line: `203.0.113.42 - - [22/Sep/2025:17:04:36 +0000] "GET /api/data HTTP/1.1" 200 5432 "https://example.com/" "Mozilla/5.0" 0.234 0.189`,
			expected: map[string]interface{}{
				"src":           "203.0.113.42",
				"method":        "GET",
				"path":          "/api/data",
				"status":        200,
				"bytes":         int64(5432),
				"user_agent":    "Mozilla/5.0",
				"rt":            0.234,
				"upstream_time": "0.189",
			},
		},
		{
			name: "apache custom format with extra fields",
			format: &LogFormat{
				Name:    "apache-extended",
				Pattern: `^(\S+) \S+ \S+ \[([^\]]+)\] "(\S+) ([^"]*) HTTP/[^"]*" (\d+) (\d+) (\d+) ([0-9.]+)`,
				Fields: map[string]string{
					"src":          "1",
					"method":       "3",
					"path":         "4",
					"status":       "5",
					"bytes":        "6",
					"response_size": "7",
					"rt":           "8",
				},
			},
			line: `10.0.0.15 - - [22/Sep/2025:17:04:36 +0000] "POST /submit HTTP/1.1" 201 128 64000 1.456`,
			expected: map[string]interface{}{
				"src":           "10.0.0.15",
				"method":        "POST",
				"path":          "/submit",
				"status":        201,
				"bytes":         int64(128),
				"response_size": "64000",
				"rt":            1.456,
			},
		},
		{
			name: "caddy json-like custom format",
			format: &LogFormat{
				Name:    "caddy-custom",
				Pattern: `^(\S+) \[([^\]]+)\] (\S+) (\S+) (\d+) (\d+) ([0-9.]+)ms`,
				Fields: map[string]string{
					"src":    "1",
					"method": "3",
					"path":   "4",
					"status": "5",
					"bytes":  "6",
					"rt":     "7",
				},
			},
			line: `192.168.1.25 [22/Sep/2025:17:04:36 +0000] GET /health 200 2 0.5ms`,
			expected: map[string]interface{}{
				"src":    "192.168.1.25",
				"method": "GET",
				"path":   "/health",
				"status": 200,
				"bytes":  int64(2),
				"rt":     0.5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, ok := parseCustomFormat(tt.line, "/var/log/access.log", "test-host", tt.format)

			if !ok {
				t.Fatalf("Expected parseCustomFormat to succeed for %s", tt.name)
			}

			// Event should be a map for custom formats
			eventMap, ok := event.(map[string]interface{})
			if !ok {
				t.Fatalf("Expected event to be a map, got type: %T", event)
			}

			// Check all expected fields
			for key, expectedValue := range tt.expected {
				actualValue, exists := eventMap[key]
				if !exists {
					t.Errorf("Expected field %s to exist in event", key)
					continue
				}

				if actualValue != expectedValue {
					t.Errorf("Expected %s=%v (%T), got %v (%T)", key, expectedValue, expectedValue, actualValue, actualValue)
				}
			}

			// Ensure host is set
			if eventMap["host"] != "test-host" {
				t.Errorf("Expected host to be 'test-host', got %v", eventMap["host"])
			}
		})
	}
}


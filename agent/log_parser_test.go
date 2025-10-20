package main

import (
	"testing"
)

func TestParseLineIntegration(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantOk   bool
		expected string
	}{
		{
			name:     "nginx access log",
			line:     `192.168.1.1 - - [22/Sep/2025:17:04:36 +0000] "GET /test HTTP/1.1" 200 1024 "https://example.com/" "curl/7.68.0"`,
			wantOk:   true,
			expected: `192.168.1.1 - - [22/Sep/2025:17:04:36 +0000] "GET /test HTTP/1.1" 200 1024 "https://example.com/" "curl/7.68.0"`,
		},
		{
			name:     "JSON log",
			line:     `{"host":"server1","path":"/api","method":"GET","status":200,"rt":0.1,"bytes":500,"src":"test.log"}`,
			wantOk:   true,
			expected: `{"host":"server1","path":"/api","method":"GET","status":200,"rt":0.1,"bytes":500,"src":"test.log"}`,
		},
		{
			name:     "plain text syslog",
			line:     "2024-01-15T10:30:45Z ERROR Database connection failed after 3 retries",
			wantOk:   true,
			expected: "2024-01-15T10:30:45Z ERROR Database connection failed after 3 retries",
		},
		{
			name:     "plain text application log",
			line:     "[2024-01-15 10:30:45] production.ERROR: Connection timeout",
			wantOk:   true,
			expected: "[2024-01-15 10:30:45] production.ERROR: Connection timeout",
		},
		{
			name:     "random text",
			line:     "random text that cannot be parsed",
			wantOk:   true,
			expected: "random text that cannot be parsed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ll := LogLine{File: "/test.log", Line: tt.line}
			event, ok := parseLine(ll)

			if ok != tt.wantOk {
				t.Errorf("parseLine() ok = %v, wantOk %v", ok, tt.wantOk)
				return
			}

			if !tt.wantOk {
				return
			}

			// All events should now be raw strings
			str, ok := event.(string)
			if !ok {
				t.Errorf("Expected event to be a raw string, got type: %T", event)
				return
			}

			if str != tt.expected {
				t.Errorf("Expected raw string to match original line.\nGot:  %s\nWant: %s", str, tt.expected)
			}
		})
	}
}

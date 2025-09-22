package main

import (
	"testing"
)

func TestParseAccessLog(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected *AccessLogEntry
		wantOk   bool
	}{
		{
			name: "nginx combined log format",
			line: `86.94.167.37 - - [22/Sep/2025:17:04:36 +0000] "GET /article/test HTTP/2.0" 200 18547 "https://accounts.google.com/" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"`,
			expected: &AccessLogEntry{
				Host:      "testhost",
				Path:      "/article/test",
				Method:    "GET",
				Status:    200,
				RT:        0.0,
				Bytes:     18547,
				Src:       "/var/log/nginx/access.log",
				IP:        "86.94.167.37",
				UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
			},
			wantOk: true,
		},
		{
			name: "nginx with response time",
			line: `192.168.1.1 - - [22/Sep/2025:17:04:36 +0000] "POST /api/test HTTP/1.1" 201 1024 "https://example.com/" "curl/7.68.0" 0.123`,
			expected: &AccessLogEntry{
				Host:      "testhost",
				Path:      "/api/test",
				Method:    "POST",
				Status:    201,
				RT:        0.123,
				Bytes:     1024,
				Src:       "/var/log/nginx/access.log",
				IP:        "192.168.1.1",
				UserAgent: "curl/7.68.0",
			},
			wantOk: true,
		},
		{
			name: "common log format",
			line: `127.0.0.1 - - [22/Sep/2025:17:04:36 +0000] "GET /index.html HTTP/1.1" 200 2326`,
			expected: &AccessLogEntry{
				Host:   "testhost",
				Path:   "/index.html",
				Method: "GET",
				Status: 200,
				RT:     0.0,
				Bytes:  2326,
				Src:    "/var/log/nginx/access.log",
				IP:     "127.0.0.1",
			},
			wantOk: true,
		},
		{
			name:     "invalid log format",
			line:     "this is not a valid access log line",
			expected: nil,
			wantOk:   false,
		},
		{
			name: "apache combined format",
			line: `10.0.0.1 - frank [10/Oct/2000:13:55:36 -0700] "GET /apache_pb.gif HTTP/1.0" 200 2326 "http://www.example.com/start.html" "Mozilla/4.08 [en] (Win98; I ;Nav)"`,
			expected: &AccessLogEntry{
				Host:      "testhost",
				Path:      "/apache_pb.gif",
				Method:    "GET",
				Status:    200,
				RT:        0.0,
				Bytes:     2326,
				Src:       "/var/log/apache2/access.log",
				IP:        "10.0.0.1",
				UserAgent: "Mozilla/4.08 [en] (Win98; I ;Nav)",
			},
			wantOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := parseAccessLog(tt.line, "/var/log/nginx/access.log", "testhost")
			if tt.name == "apache combined format" {
				result, ok = parseAccessLog(tt.line, "/var/log/apache2/access.log", "testhost")
			}

			if ok != tt.wantOk {
				t.Errorf("parseAccessLog() ok = %v, wantOk %v", ok, tt.wantOk)
				return
			}

			if !tt.wantOk {
				return
			}

			if result.Host != tt.expected.Host {
				t.Errorf("Host = %v, want %v", result.Host, tt.expected.Host)
			}
			if result.Path != tt.expected.Path {
				t.Errorf("Path = %v, want %v", result.Path, tt.expected.Path)
			}
			if result.Method != tt.expected.Method {
				t.Errorf("Method = %v, want %v", result.Method, tt.expected.Method)
			}
			if result.Status != tt.expected.Status {
				t.Errorf("Status = %v, want %v", result.Status, tt.expected.Status)
			}
			if result.RT != tt.expected.RT {
				t.Errorf("RT = %v, want %v", result.RT, tt.expected.RT)
			}
			if result.Bytes != tt.expected.Bytes {
				t.Errorf("Bytes = %v, want %v", result.Bytes, tt.expected.Bytes)
			}
			if result.IP != tt.expected.IP {
				t.Errorf("IP = %v, want %v", result.IP, tt.expected.IP)
			}
			if result.UserAgent != tt.expected.UserAgent {
				t.Errorf("UserAgent = %v, want %v", result.UserAgent, tt.expected.UserAgent)
			}
		})
	}
}

func TestParseLineIntegration(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantOk   bool
		checkKey string
	}{
		{
			name:     "valid access log",
			line:     `192.168.1.1 - - [22/Sep/2025:17:04:36 +0000] "GET /test HTTP/1.1" 200 1024 "https://example.com/" "curl/7.68.0"`,
			wantOk:   true,
			checkKey: "path",
		},
		{
			name:     "valid JSON log",
			line:     `{"host":"server1","path":"/api","method":"GET","status":200,"rt":0.1,"bytes":500,"src":"test.log"}`,
			wantOk:   true,
			checkKey: "path",
		},
		{
			name:   "invalid log",
			line:   "random text that cannot be parsed",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ll := LogLine{File: "/test.log", Line: tt.line}
			event, ok := parseLine(ll, "testhost")

			if ok != tt.wantOk {
				t.Errorf("parseLine() ok = %v, wantOk %v", ok, tt.wantOk)
				return
			}

			if tt.wantOk && event != nil {
				if _, hasKey := event[tt.checkKey]; !hasKey {
					t.Errorf("Expected key %s not found in event: %+v", tt.checkKey, event)
				}

				// Verify required fields are present
				requiredFields := []string{"host", "path", "method", "status", "rt", "bytes", "src"}
				for _, field := range requiredFields {
					if _, has := event[field]; !has {
						t.Errorf("Required field %s missing from event: %+v", field, event)
					}
				}
			}
		})
	}
}
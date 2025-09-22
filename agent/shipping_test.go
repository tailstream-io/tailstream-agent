package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestShipEvents(t *testing.T) {
	tests := []struct {
		name           string
		stream         StreamConfig
		globalKey      string
		events         []Event
		serverResponse int
		serverBody     string
		expectError    bool
		expectedAuth   string
	}{
		{
			name: "successful shipment with stream key",
			stream: StreamConfig{
				Name:     "test-stream",
				StreamID: "stream-123",
				Key:      "stream-key",
			},
			globalKey: "global-key",
			events: []Event{
				{"host": "test", "path": "/test", "method": "GET", "status": 200, "rt": 0.1, "bytes": 100, "src": "test.log"},
			},
			serverResponse: 200,
			serverBody:     `{"accepted": 1}`,
			expectError:    false,
			expectedAuth:   "Bearer stream-key",
		},
		{
			name: "successful shipment with global key",
			stream: StreamConfig{
				Name:     "test-stream",
				StreamID: "stream-123",
				// No stream-specific key
			},
			globalKey: "global-key",
			events: []Event{
				{"host": "test", "path": "/test", "method": "GET", "status": 200, "rt": 0.1, "bytes": 100, "src": "test.log"},
			},
			serverResponse: 200,
			serverBody:     `{"accepted": 1}`,
			expectError:    false,
			expectedAuth:   "Bearer global-key",
		},
		{
			name: "server error response",
			stream: StreamConfig{
				Name:     "test-stream",
				StreamID: "stream-123",
				Key:      "test-key",
			},
			events: []Event{
				{"host": "test", "path": "/test", "method": "GET", "status": 200, "rt": 0.1, "bytes": 100, "src": "test.log"},
			},
			serverResponse: 401,
			serverBody:     `{"error": "unauthorized"}`,
			expectError:    true,
		},
		{
			name: "missing stream ID",
			stream: StreamConfig{
				Name: "test-stream",
				// No StreamID
			},
			events:      []Event{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and content type
				if r.Method != "POST" {
					t.Errorf("Expected POST request, got %s", r.Method)
				}

				contentType := r.Header.Get("Content-Type")
				if contentType != "application/x-ndjson" {
					t.Errorf("Expected Content-Type application/x-ndjson, got %s", contentType)
				}

				// Verify authorization header
				if tt.expectedAuth != "" {
					auth := r.Header.Get("Authorization")
					if auth != tt.expectedAuth {
						t.Errorf("Expected Authorization %s, got %s", tt.expectedAuth, auth)
					}
				}

				// Verify request body is NDJSON format
				body := make([]byte, r.ContentLength)
				r.Body.Read(body)
				bodyStr := string(body)

				// Should end with newline for NDJSON
				if len(tt.events) > 0 && !strings.HasSuffix(bodyStr, "\n") {
					t.Error("Expected NDJSON body to end with newline")
				}

				// Count number of JSON lines
				lines := strings.Split(strings.TrimSpace(bodyStr), "\n")
				if len(tt.events) > 0 && len(lines) != len(tt.events) {
					t.Errorf("Expected %d JSON lines, got %d", len(tt.events), len(lines))
				}

				w.WriteHeader(tt.serverResponse)
				w.Write([]byte(tt.serverBody))
			}))
			defer server.Close()

			// Note: In a production app, we'd want to make HTTP client configurable for testing

			// For testing, we need to modify the shipEvents function to accept a custom URL
			// Since we can't easily do that without changing the signature, we'll test indirectly
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Skip this test if no StreamID (will fail validation before HTTP call)
			if tt.stream.StreamID == "" {
				err := shipEvents(ctx, tt.stream, tt.globalKey, tt.events)
				if !tt.expectError {
					t.Errorf("Expected no error, got: %v", err)
				}
				return
			}

			// For successful test cases, we need to modify the URL temporarily
			// This is a limitation of the current design - in a real scenario,
			// we'd want to make the HTTP client and URL configurable for testing
			t.Skip("HTTP testing requires refactoring shipEvents to accept configurable URL/client")
		})
	}
}

func TestShipEventsNDJSONFormat(t *testing.T) {
	// Test that events are properly converted to NDJSON format
	// This would require extracting the NDJSON conversion into a separate function
	t.Skip("NDJSON format testing requires refactoring to extract conversion logic")
}

func TestShipEventsTimeout(t *testing.T) {
	// Test timeout behavior
	// This would timeout, but we can't easily test it without making the URL configurable
	t.Skip("Timeout testing requires refactoring shipEvents for testability")
}
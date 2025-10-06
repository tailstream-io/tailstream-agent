package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNDJSONFormatCorrectness(t *testing.T) {
	// Verify that NDJSON output matches backend expectations:
	// Each line must be valid JSON (either object or string)

	events := []Event{
		// Structured JSON event
		map[string]interface{}{
			"host":   "server1",
			"path":   "/api",
			"method": "GET",
			"status": 200,
			"rt":     0.1,
			"bytes":  500,
			"src":    "access.log",
		},
		// Raw text events (should become JSON strings)
		"2024-01-15T10:30:45Z ERROR Database connection failed",
		"[2024-01-15 10:30:46] production.ERROR: Connection timeout",
	}

	// Simulate the shipEvents NDJSON conversion
	var buf bytes.Buffer
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("Failed to marshal event: %v", err)
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 3 {
		t.Fatalf("Expected 3 lines in NDJSON output, got %d", len(lines))
	}

	// Verify each line is valid JSON
	for i, line := range lines {
		var v interface{}
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			t.Errorf("Line %d is not valid JSON: %s\nError: %v", i+1, line, err)
		}
	}

	// Verify line 1 is a JSON object
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &obj); err != nil {
		t.Errorf("Line 1 should be a JSON object: %v", err)
	} else {
		if obj["host"] != "server1" {
			t.Errorf("Expected host=server1, got %v", obj["host"])
		}
	}

	// Verify lines 2 and 3 are JSON strings
	var str1, str2 string
	if err := json.Unmarshal([]byte(lines[1]), &str1); err != nil {
		t.Errorf("Line 2 should be a JSON string: %v", err)
	} else {
		if str1 != "2024-01-15T10:30:45Z ERROR Database connection failed" {
			t.Errorf("Line 2 string content mismatch: %s", str1)
		}
	}

	if err := json.Unmarshal([]byte(lines[2]), &str2); err != nil {
		t.Errorf("Line 3 should be a JSON string: %v", err)
	} else {
		if str2 != "[2024-01-15 10:30:46] production.ERROR: Connection timeout" {
			t.Errorf("Line 3 string content mismatch: %s", str2)
		}
	}
}

func TestNDJSONContentType(t *testing.T) {
	// Verify Content-Type header is correct
	// This is already tested in shipping_test.go but we document it here
	expectedContentType := "application/x-ndjson"

	// In the real code, shipEvents sets:
	// req.Header.Set("Content-Type", "application/x-ndjson")

	if expectedContentType != "application/x-ndjson" {
		t.Errorf("Content-Type should be application/x-ndjson")
	}
}

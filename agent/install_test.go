package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestBinaryInstallation builds the agent binary and verifies it can
// ship a JSON log line to an HTTP endpoint.
func TestBinaryInstallation(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "tailstream-agent")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	received := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		received <- b
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	logDir := filepath.Join(tmp, "logs")
	if err := os.Mkdir(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logFile := filepath.Join(logDir, "access.log")
	if f, err := os.Create(logFile); err == nil {
		f.Close()
	} else {
		t.Fatalf("create log: %v", err)
	}

	cfg := []byte("ship:\n  url: '" + srv.URL + "'\n  stream_id: 'test'\nkey: 'dummy'\ndiscovery:\n  paths:\n    include:\n      - '" + filepath.Join(logDir, "*.log") + "'\nupdates:\n  enabled: false\n")
	cfgFile := filepath.Join(tmp, "agent.yaml")
	if err := os.WriteFile(cfgFile, cfg, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, bin, "--config", cfgFile, "--debug")
	cmd.Env = append(os.Environ(), "TAILSTREAM_KEY=dummy")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		t.Fatalf("start agent: %v", err)
	}
	defer func() {
		cancel()
		cmd.Wait()
	}()

	// Append an access log line after the agent starts tailing
	time.Sleep(1 * time.Second) // Give more time for the agent to start tailing
	appendFile(t, logFile, `192.168.1.1 - - [01/Jan/2025:12:00:00 +0000] "GET /test HTTP/1.1" 200 1234 "https://example.com" "TestAgent/1.0"`+"\n")

	// Give some time for the log to be processed
	time.Sleep(100 * time.Millisecond)

	select {
	case data := <-received:
		// Data should be in NDJSON format (newline-delimited JSON)
		lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
		if len(lines) != 1 {
			t.Fatalf("expected 1 NDJSON line, got %d", len(lines))
		}

		var event map[string]any
		if err := json.Unmarshal(lines[0], &event); err != nil {
			t.Fatalf("bad NDJSON payload: %v", err)
		}

		// Verify required fields are present
		requiredFields := []string{"host", "path", "method", "status", "rt", "bytes", "src"}
		for _, field := range requiredFields {
			if _, ok := event[field]; !ok {
				t.Fatalf("missing required field %s in event: %v", field, event)
			}
		}

		// Verify some specific values
		if event["path"] != "/test" {
			t.Errorf("expected path '/test', got %v", event["path"])
		}
		if event["method"] != "GET" {
			t.Errorf("expected method 'GET', got %v", event["method"])
		}
		if event["status"] != float64(200) { // JSON numbers are float64
			t.Errorf("expected status 200, got %v", event["status"])
		}

	case <-time.After(5 * time.Second):
		t.Fatalf("agent did not ship logs:\n%s", out.String())
	}
}

func appendFile(t *testing.T, name, line string) {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("open append: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString(line); err != nil {
		t.Fatalf("write: %v", err)
	}
}

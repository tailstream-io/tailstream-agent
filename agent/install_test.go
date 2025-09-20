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

	cfg := []byte("ship:\n  url: '" + srv.URL + "'\ndiscovery:\n  paths:\n    include:\n      - '" + filepath.Join(logDir, "*.log") + "'\n")
	cfgFile := filepath.Join(tmp, "agent.yaml")
	if err := os.WriteFile(cfgFile, cfg, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, bin, "--config", cfgFile)
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

	// Append a JSON line after the agent starts tailing
	time.Sleep(300 * time.Millisecond)
	appendFile(t, logFile, "{\"msg\":\"hi\"}\n")

	select {
	case data := <-received:
		var events []map[string]any
		if err := json.Unmarshal(data, &events); err != nil {
			t.Fatalf("bad payload: %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
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

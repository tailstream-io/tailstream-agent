package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "a.log"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(tmp, "b.gz"), []byte("x"), 0o644)

	var cfg Config
	cfg.Streams = []StreamConfig{
		{
			Name:     "test-stream",
			StreamID: "test-123",
			URL:      "https://test.example.com",
			Paths:    []string{filepath.Join(tmp, "*.log"), filepath.Join(tmp, "*.gz")},
			Exclude:  []string{"**/*.gz"},
		},
	}

	mappings, err := discover(cfg)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(mappings) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(mappings))
	}

	files := mappings[0].Files
	if len(files) != 1 || files[0] != filepath.Join(tmp, "a.log") {
		t.Fatalf("unexpected files: %v", files)
	}
}

func TestParseLineJSON(t *testing.T) {
	ll := LogLine{File: "/var/log/test.log", Line: `{"msg":"hi","host":"host1","src":"/var/log/test.log","path":"/test","method":"GET","status":200,"rt":0.1,"bytes":100}`}
	ev, ok := parseLine(ll, "host1", nil)
	if !ok {
		t.Fatal("not parsed")
	}
	if ev["msg"] != "hi" || ev["host"] != "host1" || ev["src"] != "/var/log/test.log" {
		t.Fatalf("bad event: %#v", ev)
	}
}

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
	cfg.Discovery.Paths.Include = []string{filepath.Join(tmp, "*.log"), filepath.Join(tmp, "*.gz")}
	cfg.Discovery.Paths.Exclude = []string{"**/*.gz"}

	files, err := discover(cfg)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(files) != 1 || files[0] != filepath.Join(tmp, "a.log") {
		t.Fatalf("unexpected files: %v", files)
	}
}

func TestParseLineJSON(t *testing.T) {
	ll := LogLine{File: "/var/log/test.log", Line: `{"msg":"hi"}`}
	ev, ok := parseLine(ll, "prod", "host1")
	if !ok {
		t.Fatal("not parsed")
	}
	if ev["msg"] != "hi" || ev["env"] != "prod" || ev["host"] != "host1" {
		t.Fatalf("bad event: %#v", ev)
	}
}

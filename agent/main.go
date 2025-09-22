package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

// Config holds agent configuration values.
type Config struct {
	Env string `yaml:"env"`
	Key string `yaml:"-"`

	Discovery struct {
		Enabled bool `yaml:"enabled"`
		Paths   struct {
			Include []string `yaml:"include"`
			Exclude []string `yaml:"exclude"`
		} `yaml:"paths"`
	} `yaml:"discovery"`

	Ship struct {
		URL string `yaml:"url"`
	} `yaml:"ship"`
}

// loadConfig resolves configuration from environment, flags and optional YAML file.
func loadConfig() Config {
	var cfg Config

	cfg.Env = getenv("TAILSTREAM_ENV", "production")
	cfg.Ship.URL = getenv("TAILSTREAM_URL", "https://ingest.tailstream.com/v1/batch")
	cfg.Discovery.Enabled = true
	cfg.Discovery.Paths.Include = []string{
		"/var/log/nginx/*.log",
		"/var/log/caddy/*.log",
		"/var/log/apache2/*.log",
		"/var/log/httpd/*.log",
		"/var/www/**/storage/logs/*.log",
	}
	cfg.Discovery.Paths.Exclude = []string{"**/*.gz", "**/*.1"}
	cfg.Key = os.Getenv("TAILSTREAM_KEY")

	configFile := ""
	flag.StringVar(&configFile, "config", "", "path to YAML config")
	envFlag := flag.String("env", "", "environment")
	keyFile := flag.String("key-file", "", "path to key file")
	shipURL := flag.String("ship-url", "", "ship URL")
	debug := flag.Bool("debug", false, "enable debug output")
	flag.Parse()

	if configFile != "" {
		if b, err := os.ReadFile(configFile); err == nil {
			yaml.Unmarshal(b, &cfg)
		}
	}
	if *envFlag != "" {
		cfg.Env = *envFlag
	}
	if *shipURL != "" {
		cfg.Ship.URL = *shipURL
	}
	if *keyFile != "" {
		if b, err := os.ReadFile(*keyFile); err == nil {
			cfg.Key = strings.TrimSpace(string(b))
		}
	}

	// Store debug flag in environment for access elsewhere
	if *debug {
		os.Setenv("DEBUG", "1")
	}

	return cfg
}

func getenv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

// discover finds log files based on include/exclude glob patterns.
func discover(cfg Config) ([]string, error) {
	var files []string
	for _, pattern := range cfg.Discovery.Paths.Include {
		matches, err := doublestar.FilepathGlob(pattern)
		if err != nil {
			continue
		}
		for _, m := range matches {
			if excluded(m, cfg.Discovery.Paths.Exclude) {
				continue
			}
			files = append(files, m)
		}
	}
	return files, nil
}

func excluded(path string, patterns []string) bool {
	for _, p := range patterns {
		ok, err := doublestar.Match(p, path)
		if err == nil && ok {
			return true
		}
	}
	return false
}

// LogLine represents a line read from a file.
type LogLine struct {
	File string
	Line string
}

// tailFile streams appended lines from a file.
func tailFile(ctx context.Context, file string, ch chan<- LogLine) {
	f, err := os.Open(file)
	if err != nil {
		log.Printf("open %s: %v", file, err)
		return
	}
	defer f.Close()

	f.Seek(0, io.SeekEnd)
	reader := bufio.NewReader(f)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					time.Sleep(200 * time.Millisecond)
					continue
				}
				log.Printf("read %s: %v", file, err)
				return
			}
			ch <- LogLine{File: file, Line: strings.TrimRight(line, "\r\n")}
		}
	}
}

// Event is the normalized record to send to Tailstream.
type Event map[string]interface{}

// parseLine normalizes a log line. Currently only JSON lines are parsed.
func parseLine(ll LogLine, env, host string) (Event, bool) {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(ll.Line), &m); err == nil {
		m["env"] = env
		m["host"] = host
		m["file"] = ll.File
		return m, true
	}
	// fallback raw line
	return Event{
		"env":  env,
		"host": host,
		"file": ll.File,
		"line": ll.Line,
	}, true
}

// shipEvents POSTs a batch of events to Tailstream ingest endpoint.
func shipEvents(ctx context.Context, cfg Config, events []Event) error {
	if cfg.Ship.URL == "" {
		return fmt.Errorf("ship URL not configured")
	}
	data, err := json.Marshal(events)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Ship.URL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.Key != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Key)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("ship: %s", resp.Status)
	}
	return nil
}

func main() {
	cfg := loadConfig()
	host, _ := os.Hostname()

	if os.Getenv("DEBUG") == "1" {
		log.Printf("Starting tailstream agent (env=%s, key=%s, url=%s)",
			cfg.Env,
			cfg.Key[:min(len(cfg.Key), 10)]+"...",
			cfg.Ship.URL)
	}

	files, err := discover(cfg)
	if err != nil {
		log.Fatalf("discover: %v", err)
	}
	if len(files) == 0 {
		log.Println("no log files discovered")
		return
	}

	if os.Getenv("DEBUG") == "1" {
		log.Printf("Found %d log files: %v", len(files), files)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lines := make(chan LogLine, 100)
	var wg sync.WaitGroup
	for _, f := range files {
		wg.Add(1)
		go func(fn string) {
			defer wg.Done()
			tailFile(ctx, fn, lines)
		}(f)
	}

	batch := make([]Event, 0, 100)
	ticker := time.NewTicker(2 * time.Second)
	for {
		select {
		case ll := <-lines:
			if os.Getenv("DEBUG") == "1" {
				log.Printf("Processing line from %s: %s", ll.File, ll.Line)
			}
			ev, ok := parseLine(ll, cfg.Env, host)
			if ok {
				batch = append(batch, ev)
				if len(batch) >= 100 {
					if os.Getenv("DEBUG") == "1" {
						log.Printf("Batch full, shipping %d events", len(batch))
					}
					if err := shipEvents(ctx, cfg, batch); err != nil {
						log.Printf("ship: %v", err)
					} else if os.Getenv("DEBUG") == "1" {
						log.Printf("Successfully shipped batch of %d events", len(batch))
					}
					batch = batch[:0]
				}
			}
		case <-ticker.C:
			if len(batch) > 0 {
				if os.Getenv("DEBUG") == "1" {
					log.Printf("Timer tick, shipping %d events", len(batch))
				}
				if err := shipEvents(ctx, cfg, batch); err != nil {
					log.Printf("ship: %v", err)
				} else if os.Getenv("DEBUG") == "1" {
					log.Printf("Successfully shipped %d events", len(batch))
				}
				batch = batch[:0]
			}
		case <-ctx.Done():
			wg.Wait()
			return
		}
	}
}

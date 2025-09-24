package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	Version   = "dev"
	BuildDate = "unknown"
	GitCommit = "unknown"
)



// LogLine represents a line read from a file.
type LogLine struct {
	File string
	Line string
}

// tailFile streams appended lines from a file.
// If the file becomes inaccessible, it will retry opening it every 5 seconds.
func tailFile(ctx context.Context, file string, ch chan<- LogLine) {
	var f *os.File
	var reader *bufio.Reader
	var err error

	// Try to open file initially
	f, err = os.Open(file)
	if err != nil {
		log.Printf("ERROR: Cannot open %s: %v - will retry every 5s", file, err)
	} else {
		f.Seek(0, io.SeekEnd)
		reader = bufio.NewReader(f)
	}

	retryTicker := time.NewTicker(5 * time.Second)
	defer retryTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			if f != nil {
				f.Close()
			}
			return

		case <-retryTicker.C:
			// Retry opening file if we don't have it open
			if f == nil {
				f, err = os.Open(file)
				if err != nil {
					log.Printf("ERROR: Still cannot access %s: %v - will keep retrying", file, err)
					continue
				}
				log.Printf("SUCCESS: Reconnected to %s after access issue", file)
				f.Seek(0, io.SeekEnd)
				reader = bufio.NewReader(f)
			}

		default:
			if f == nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					time.Sleep(200 * time.Millisecond)
					continue
				}
				log.Printf("ERROR: Lost access to %s: %v - will attempt to reconnect", file, err)
				f.Close()
				f = nil
				reader = nil
				continue
			}
			ch <- LogLine{File: file, Line: strings.TrimRight(line, "\r\n")}
		}
	}
}


// shipEvents POSTs a batch of events to a specific stream's ingest endpoint as NDJSON.
func shipEvents(ctx context.Context, stream StreamConfig, globalKey string, events []Event) error {
	if stream.StreamID == "" {
		return fmt.Errorf("stream ID not configured for stream %s", stream.Name)
	}

	url := stream.GetURL()

	// Convert events to NDJSON format
	var buf bytes.Buffer
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			continue
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-ndjson")

	// Use stream-specific key if available, otherwise fall back to global key
	key := stream.Key
	if key == "" {
		key = globalKey
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	// Read response body for better error reporting
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if os.Getenv("DEBUG") == "1" {
		log.Printf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("ship: %s - %s", resp.Status, string(body))
	}
	return nil
}

func main() {
	// Handle version command
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v" || os.Args[1] == "version") {
		fmt.Printf("tailstream-agent %s\n", Version)
		fmt.Printf("Build date: %s\n", BuildDate)
		fmt.Printf("Git commit: %s\n", GitCommit)
		return
	}

	// Handle help command
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h" || os.Args[1] == "help") {
		fmt.Printf("Tailstream Agent %s\n\n", Version)
		fmt.Printf("USAGE:\n")
		fmt.Printf("  tailstream-agent [COMMAND]\n\n")
		fmt.Printf("COMMANDS:\n")
		fmt.Printf("  (none)       Start the agent (runs setup wizard if needed)\n")
		fmt.Printf("  version      Show version information\n")
		fmt.Printf("  update       Check for and install updates manually\n")
		fmt.Printf("  status       Show agent and update status\n")
		fmt.Printf("  help         Show this help message\n\n")
		fmt.Printf("OPTIONS:\n")
		fmt.Printf("  --config     Path to configuration file\n")
		fmt.Printf("  --version    Show version information\n")
		fmt.Printf("  --help       Show this help message\n\n")
		fmt.Printf("EXAMPLES:\n")
		fmt.Printf("  tailstream-agent                           # Start with setup wizard\n")
		fmt.Printf("  tailstream-agent --config /path/config.yaml\n")
		fmt.Printf("  tailstream-agent update                    # Manual update check\n")
		fmt.Printf("  tailstream-agent status                    # Check status\n")
		return
	}

	// Handle status command
	if len(os.Args) > 1 && os.Args[1] == "status" {
		fmt.Printf("Tailstream Agent Status\n")
		fmt.Printf("Version: %s\n", Version)
		fmt.Printf("Build Date: %s\n", BuildDate)
		fmt.Printf("Git Commit: %s\n", GitCommit)

		// Show installation type
		execPath, _ := os.Executable()
		if realPath, err := filepath.EvalSymlinks(execPath); err == nil && realPath != execPath {
			fmt.Printf("Installation: %s (symlinked from %s)\n", realPath, execPath)
		} else {
			fmt.Printf("Installation: %s\n", execPath)
		}

		fmt.Printf("Auto-updates: Enabled\n")
		return
	}

	// Check if setup wizard is needed
	if len(os.Args) == 1 && needsSetup() {
		if err := setupWizard(); err != nil {
			log.Fatalf("Setup failed: %v", err)
		}
		return
	}

	cfg := loadConfig()

	// Handle update command (after config loading)
	for _, arg := range os.Args {
		if arg == "update" || arg == "--update" {
			fmt.Printf("Checking for updates (current version: %s)...\n", Version)
			checkForUpdatesForce(cfg, true)
			return
		}
	}

	// Check for updates in the background
	go checkForUpdates(cfg)
	host, _ := os.Hostname()

	// Validate required configuration
	if cfg.Key == "" && len(cfg.Streams) == 0 {
		log.Fatal("No access token configured. Use environment variable TAILSTREAM_KEY, run setup wizard, or create tailstream.yaml")
	}

	// For legacy single-stream config
	if len(cfg.Streams) == 0 && cfg.Ship.URL == "" {
		log.Fatal("No ship URL configured. Run setup wizard or create tailstream.yaml")
	}

	if os.Getenv("DEBUG") == "1" {
		keyDisplay := cfg.Key
		if len(keyDisplay) > 10 {
			keyDisplay = keyDisplay[:10] + "..."
		}
		if len(cfg.Streams) > 0 {
			log.Printf("Starting tailstream agent with %d streams (env=%s)", len(cfg.Streams), cfg.Env)
		} else {
			log.Printf("Starting tailstream agent (env=%s, key=%s, url=%s)",
				cfg.Env, keyDisplay, cfg.Ship.URL)
		}
	}

	mappings, err := discover(cfg)
	if err != nil {
		log.Fatalf("discover: %v", err)
	}
	if len(mappings) == 0 {
		log.Println("no log files discovered")
		return
	}

	if os.Getenv("DEBUG") == "1" {
		for _, mapping := range mappings {
			log.Printf("Stream '%s': found %d files: %v", mapping.Stream.Name, len(mapping.Files), mapping.Files)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create separate channels and batches for each stream
	type streamData struct {
		stream StreamConfig
		lines  chan LogLine
		batch  []Event
	}

	streamMap := make(map[string]*streamData)
	var wg sync.WaitGroup

	// Set up tailing for each stream's files
	for _, mapping := range mappings {
		sd := &streamData{
			stream: mapping.Stream,
			lines:  make(chan LogLine, 100),
			batch:  make([]Event, 0, 100),
		}
		streamMap[mapping.Stream.Name] = sd

		// Start tailing all files for this stream
		for _, f := range mapping.Files {
			wg.Add(1)
			go func(filename string, ch chan LogLine) {
				defer wg.Done()
				tailFile(ctx, filename, ch)
			}(f, sd.lines)
		}
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Process events from all streams
	for {
		select {
		case <-ticker.C:
			// Ship batches for all streams
			for streamName, sd := range streamMap {
				if len(sd.batch) > 0 {
					if os.Getenv("DEBUG") == "1" {
						log.Printf("Timer tick, shipping %d events for stream '%s'", len(sd.batch), streamName)
					}
					if err := shipEvents(ctx, sd.stream, cfg.Key, sd.batch); err != nil {
						log.Printf("ship to stream '%s': %v", streamName, err)
					} else if os.Getenv("DEBUG") == "1" {
						log.Printf("Successfully shipped %d events to stream '%s'", len(sd.batch), streamName)
					}
					sd.batch = sd.batch[:0]
				}
			}
		case <-ctx.Done():
			wg.Wait()
			return
		default:
			// Check for new lines from any stream
			for streamName, sd := range streamMap {
				select {
				case ll := <-sd.lines:
					if os.Getenv("DEBUG") == "1" {
						log.Printf("Processing line from %s (stream '%s'): %s", ll.File, streamName, ll.Line)
					}
					ev, ok := parseLine(ll, host, sd.stream.Format)
					if ok && ev != nil {
						sd.batch = append(sd.batch, ev)
						if os.Getenv("DEBUG") == "1" {
							log.Printf("Parsed event for stream '%s': %+v", streamName, ev)
						}
						if len(sd.batch) >= 100 {
							if os.Getenv("DEBUG") == "1" {
								log.Printf("Batch full, shipping %d events for stream '%s'", len(sd.batch), streamName)
							}
							if err := shipEvents(ctx, sd.stream, cfg.Key, sd.batch); err != nil {
								log.Printf("ship to stream '%s': %v", streamName, err)
							} else if os.Getenv("DEBUG") == "1" {
								log.Printf("Successfully shipped batch of %d events to stream '%s'", len(sd.batch), streamName)
							}
							sd.batch = sd.batch[:0]
						}
					}
				default:
					// No new lines for this stream, continue
				}
			}
		}
	}
}

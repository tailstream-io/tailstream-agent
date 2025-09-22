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
	"strings"
	"sync"
	"time"
)



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
	// Check if setup wizard is needed
	if len(os.Args) == 1 && needsSetup() {
		if err := setupWizard(); err != nil {
			log.Fatalf("Setup failed: %v", err)
		}
		return
	}

	cfg := loadConfig()
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
					ev, ok := parseLine(ll, host)
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

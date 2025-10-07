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
	"syscall"
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
// It also detects log rotation by tracking file inodes.
func tailFile(ctx context.Context, file string, ch chan<- LogLine) {
	var f *os.File
	var reader *bufio.Reader
	var currentInode uint64
	var err error

	// Try to open file initially
	f, err = os.Open(file)
	if err != nil {
		log.Printf("ERROR: Cannot open %s: %v - will retry every 5s", file, err)
	} else {
		f.Seek(0, io.SeekEnd)
		reader = bufio.NewReader(f)
		if stat, err := f.Stat(); err == nil {
			if sys := stat.Sys(); sys != nil {
				if stat_t, ok := sys.(*syscall.Stat_t); ok {
					currentInode = stat_t.Ino
				}
			}
		}
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
			// Check if file has been rotated (inode changed)
			if f != nil {
				if stat, err := os.Stat(file); err == nil {
					if sys := stat.Sys(); sys != nil {
						if stat_t, ok := sys.(*syscall.Stat_t); ok {
							if stat_t.Ino != currentInode {
								log.Printf("ROTATION: File %s rotated (inode changed %d -> %d), reconnecting", file, currentInode, stat_t.Ino)
								f.Close()
								f = nil
								reader = nil
								currentInode = 0
							}
						}
					}
				} else {
					// File disappeared, close and retry
					log.Printf("ROTATION: File %s disappeared, will reconnect", file)
					f.Close()
					f = nil
					reader = nil
					currentInode = 0
				}
			}

			// Retry opening file if we don't have it open
			if f == nil {
				f, err = os.Open(file)
				if err != nil {
					log.Printf("ERROR: Still cannot access %s: %v - will keep retrying", file, err)
					continue
				}
				log.Printf("SUCCESS: Reconnected to %s after access issue/rotation", file)
				f.Seek(0, io.SeekEnd)
				reader = bufio.NewReader(f)
				if stat, err := f.Stat(); err == nil {
					if sys := stat.Sys(); sys != nil {
						if stat_t, ok := sys.(*syscall.Stat_t); ok {
							currentInode = stat_t.Ino
						}
					}
				}
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
				currentInode = 0
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
		// Marshal event to JSON (handles both strings and objects)
		// - Strings become JSON strings: "raw text here\n"
		// - Objects become JSON objects: {"field":"value"}
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

// runStdinMode processes logs from stdin and ships them to a stream
func runStdinMode(cfg Config) {
	// Get stream ID from flag or environment
	streamID := os.Getenv("TAILSTREAM_STREAM_ID")
	if streamID == "" {
		log.Fatal("stdin mode requires --stream-id flag")
	}

	// Get access token from multiple sources (in priority order):
	// 1. --key-file flag
	// 2. TAILSTREAM_KEY environment variable
	// 3. Config file
	accessToken := ""

	// Try key file first
	if keyFile := os.Getenv("TAILSTREAM_KEY_FILE"); keyFile != "" {
		keyBytes, err := os.ReadFile(keyFile)
		if err != nil {
			log.Fatalf("failed to read key file %s: %v", keyFile, err)
		}
		accessToken = strings.TrimSpace(string(keyBytes))
	}

	// Fall back to environment variable
	if accessToken == "" {
		accessToken = os.Getenv("TAILSTREAM_KEY")
	}

	// Fall back to config file
	if accessToken == "" && len(cfg.Streams) > 0 && cfg.Streams[0].Key != "" {
		accessToken = cfg.Streams[0].Key
	}

	if accessToken == "" {
		log.Fatal("stdin mode requires authentication via --key-file, TAILSTREAM_KEY environment variable, or config file with access token")
	}

	// Create a stream config for stdin mode
	stream := StreamConfig{
		Name:     "stdin",
		StreamID: streamID,
		Key:      accessToken,
	}

	host, _ := os.Hostname()
	ctx := context.Background()

	scanner := bufio.NewScanner(os.Stdin)
	batch := make([]Event, 0, 100)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	if os.Getenv("DEBUG") == "1" {
		log.Printf("Starting stdin mode for stream: %s", streamID)
	}

	// Channel for new lines
	lines := make(chan string, 100)

	// Read stdin in goroutine
	go func() {
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			log.Printf("Error reading stdin: %v", err)
		}
		close(lines) // Close channel instead of sending done signal
	}()

	// Process lines
	running := true
	for running {
		select {
		case <-ticker.C:
			if len(batch) > 0 {
				if os.Getenv("DEBUG") == "1" {
					log.Printf("Timer tick, shipping %d events", len(batch))
				}
				if err := shipEvents(ctx, stream, "", batch); err != nil {
					log.Printf("ship: %v", err)
				}
				batch = batch[:0]
			}

		case line, ok := <-lines:
			if !ok {
				// Channel closed, process remaining batch and exit
				running = false
				break
			}
			if os.Getenv("DEBUG") == "1" {
				log.Printf("Processing line: %s", line)
			}
			ll := LogLine{File: "stdin", Line: line}
			ev, ok := parseLine(ll, host, stream.Format)
			if ok && ev != nil {
				batch = append(batch, ev)
				if len(batch) >= 100 {
					if os.Getenv("DEBUG") == "1" {
						log.Printf("Batch full, shipping %d events", len(batch))
					}
					if err := shipEvents(ctx, stream, "", batch); err != nil {
						log.Printf("ship: %v", err)
					}
					batch = batch[:0]
				}
			}
		}
	}

	// Ship any remaining events after stdin closes
	if len(batch) > 0 {
		if os.Getenv("DEBUG") == "1" {
			log.Printf("EOF, shipping final %d events", len(batch))
		}
		if err := shipEvents(ctx, stream, "", batch); err != nil {
			log.Printf("ship: %v", err)
		}
	}
}

func main() {
	// Handle version command
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v" || os.Args[1] == "version") {
		fmt.Printf("tailstream-agent %s\n", Version)
		fmt.Printf("Build date: %s\n", BuildDate)
		fmt.Printf("Git commit: %s\n", GitCommit)
		return
	}

	// Show help by default when no arguments
	showHelp := func() {
		fmt.Printf("Tailstream Agent %s\n\n", Version)
		fmt.Printf("USAGE:\n")
		fmt.Printf("  tailstream-agent [COMMAND]\n")
		fmt.Printf("  <command> | tailstream-agent --stream-id <id>    # Stdin mode\n\n")
		fmt.Printf("COMMANDS:\n")
		fmt.Printf("  run          Start the agent\n")
		fmt.Printf("  setup        Run OAuth setup wizard\n")
		fmt.Printf("  version      Show version information\n")
		fmt.Printf("  update       Check for and install updates manually\n")
		fmt.Printf("  status       Show agent and update status\n")
		fmt.Printf("  help         Show this help message\n\n")
		fmt.Printf("OPTIONS:\n")
		fmt.Printf("  --config     Path to configuration file\n")
		fmt.Printf("  --stream-id  Stream ID for stdin mode (pipe logs directly)\n")
		fmt.Printf("  --key-file   Path to file containing access token (for stdin mode)\n")
		fmt.Printf("  --version    Show version information\n")
		fmt.Printf("  --help       Show this help message\n\n")
		fmt.Printf("ENVIRONMENT VARIABLES:\n")
		fmt.Printf("  TAILSTREAM_BASE_URL    Override default API base URL\n")
		fmt.Printf("  TAILSTREAM_KEY         Access token for authentication\n\n")
		fmt.Printf("EXAMPLES:\n")
		fmt.Printf("  # First time setup:\n")
		fmt.Printf("  tailstream-agent setup                     # Run OAuth setup wizard\n\n")
		fmt.Printf("  # Standard file-based mode:\n")
		fmt.Printf("  tailstream-agent run                       # Start the agent\n")
		fmt.Printf("  tailstream-agent run --config /path/config.yaml\n")
		fmt.Printf("  tailstream-agent update                    # Manual update check\n")
		fmt.Printf("  tailstream-agent status                    # Check status\n\n")
		fmt.Printf("  # Stdin mode (pipe any log source):\n")
		fmt.Printf("  # First, securely store your access token:\n")
		fmt.Printf("  echo 'your-access-token' > ~/.tailstream-key && chmod 600 ~/.tailstream-key\n\n")
		fmt.Printf("  # Then pipe logs directly:\n")
		fmt.Printf("  tail -f /var/log/nginx/access.log | tailstream-agent --stream-id <id> --key-file ~/.tailstream-key\n")
		fmt.Printf("  kubectl logs -f pod-name | tailstream-agent --stream-id <id> --key-file ~/.tailstream-key\n")
		fmt.Printf("  docker logs -f container | tailstream-agent --stream-id <id> --key-file ~/.tailstream-key\n")
		fmt.Printf("  journalctl -f | tailstream-agent --stream-id <id> --key-file ~/.tailstream-key\n")
	}

	// Handle help command
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h" || os.Args[1] == "help") {
		showHelp()
		return
	}

	// Handle setup command (OAuth)
	if len(os.Args) > 1 && os.Args[1] == "setup" {
		if err := setupOAuth(); err != nil {
			log.Fatalf("Setup failed: %v", err)
		}
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

	// Handle update command
	if len(os.Args) > 1 && (os.Args[1] == "update" || os.Args[1] == "--update") {
		cfg := loadConfig()
		fmt.Printf("Checking for updates (current version: %s)...\n", Version)
		checkForUpdatesForce(cfg, true)
		return
	}

	// Handle run command (or stdin mode)
	isRunCommand := len(os.Args) > 1 && os.Args[1] == "run"
	hasFlags := len(os.Args) > 1 && strings.HasPrefix(os.Args[1], "-")

	if isRunCommand {
		// Remove "run" from args for config parsing
		os.Args = append(os.Args[:1], os.Args[2:]...)
	} else if len(os.Args) == 1 || hasFlags {
		// Check for stdin mode (piped input)
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			// Not piped and no explicit run command
			if hasFlags {
				// Has flags but no pipe - this is an error
				fmt.Printf("Error: Flags without command require piped input (stdin mode)\n\n")
				showHelp()
				return
			}
			// No command, no flags, no pipe - default to "run" command
			// This allows systemd service to start with just "tailstream-agent"
		}
		// stdin is piped, continue to stdin mode below
	} else {
		// Unknown command
		fmt.Printf("Unknown command: %s\n\n", os.Args[1])
		showHelp()
		return
	}

	cfg := loadConfig()

	// Check for stdin mode
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// stdin is a pipe - enter stdin mode
		runStdinMode(cfg)
		return
	}

	// Check for updates in the background
	go checkForUpdates(cfg)
	host, _ := os.Hostname()

	// Validate required configuration
	if len(cfg.Streams) == 0 {
		log.Fatal("No streams configured. Run setup wizard or create tailstream.yaml")
	}

	if os.Getenv("DEBUG") == "1" {
		log.Printf("Starting tailstream agent with %d streams (env=%s)", len(cfg.Streams), cfg.Env)
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
					if err := shipEvents(ctx, sd.stream, "", sd.batch); err != nil {
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
							if err := shipEvents(ctx, sd.stream, "", sd.batch); err != nil {
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

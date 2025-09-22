package main

import (
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds agent configuration values.
type Config struct {
	Env string `yaml:"env"`
	Key string `yaml:"key"` // Default key, can be overridden per stream

	Discovery struct {
		Enabled bool `yaml:"enabled"`
		Paths   struct {
			Include []string `yaml:"include"`
			Exclude []string `yaml:"exclude"`
		} `yaml:"paths"`
	} `yaml:"discovery"`

	// Legacy single stream config (for backward compatibility)
	Ship struct {
		URL      string `yaml:"url"`
		StreamID string `yaml:"stream_id"`
	} `yaml:"ship"`

	// New multi-stream configuration
	Streams []StreamConfig `yaml:"streams,omitempty"`
}

// LogFormat defines a custom log format configuration
type LogFormat struct {
	Name    string            `yaml:"name"`              // Human-readable name for this format
	Pattern string            `yaml:"pattern"`           // Regex pattern to match log lines
	Fields  map[string]string `yaml:"fields"`            // Field mapping from regex groups to output fields
	Default map[string]any    `yaml:"default,omitempty"` // Default values for missing fields
}

// StreamConfig defines a destination stream with its own settings
type StreamConfig struct {
	Name      string     `yaml:"name"`              // Human-readable name for this stream
	StreamID  string     `yaml:"stream_id"`         // Stream ID - URL will be constructed as https://app.tailstream.io/api/ingest/{stream_id}
	Key       string     `yaml:"key,omitempty"`     // Optional stream-specific access token
	Paths     []string   `yaml:"paths"`             // Log file patterns for this stream
	Exclude   []string   `yaml:"exclude,omitempty"` // Exclusion patterns for this stream
	Format    *LogFormat `yaml:"format,omitempty"`  // Custom log format for this stream
	LegacyURL string     `yaml:"-"`                 // Legacy URL override (not saved to YAML)
}

// GetURL returns the full ingest URL for this stream
func (sc StreamConfig) GetURL() string {
	if sc.LegacyURL != "" {
		return sc.LegacyURL
	}
	return fmt.Sprintf("https://app.tailstream.io/api/ingest/%s", sc.StreamID)
}

// loadConfig resolves configuration from environment, flags and optional YAML file.
func loadConfig() Config {
	var cfg Config

	// Set defaults
	cfg.Env = getenv("TAILSTREAM_ENV", "production")
	cfg.Discovery.Enabled = true
	cfg.Discovery.Paths.Include = []string{
		"/var/log/nginx/*.log",
		"/var/log/caddy/*.log",
		"/var/log/apache2/*.log",
		"/var/log/httpd/*.log",
		"/var/www/**/storage/logs/*.log",
	}
	cfg.Discovery.Paths.Exclude = []string{"**/*.gz", "**/*.1"}

	// Parse flags only if not already parsed (to avoid redefinition in tests)
	if !flag.Parsed() {
		configFile := flag.String("config", getDefaultConfigPath(), "path to YAML config")
		envFlag := flag.String("env", "", "environment")
		shipURL := flag.String("ship-url", "", "ship URL")
		debug := flag.Bool("debug", false, "enable debug output")
		flag.Parse()

		// Load config file (default or specified)
		if b, err := os.ReadFile(*configFile); err == nil {
			yaml.Unmarshal(b, &cfg)
		}

		// Apply flag overrides
		if *envFlag != "" {
			cfg.Env = *envFlag
		}
		if *shipURL != "" {
			cfg.Ship.URL = *shipURL
		}
		if *debug {
			os.Setenv("DEBUG", "1")
		}
	} else {
		// In tests, just load the default config file if it exists
		if b, err := os.ReadFile("tailstream.yaml"); err == nil {
			yaml.Unmarshal(b, &cfg)
		}
	}

	// Environment variable overrides (always apply)
	if envKey := os.Getenv("TAILSTREAM_KEY"); envKey != "" {
		cfg.Key = envKey
	}
	if envURL := os.Getenv("TAILSTREAM_URL"); envURL != "" {
		cfg.Ship.URL = envURL
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

// getDefaultConfigPath returns the default config file path based on the OS
func getDefaultConfigPath() string {
	// Try system-wide locations first
	systemPaths := []string{
		"/etc/tailstream/agent.yaml",     // Linux system-wide
		"/usr/local/etc/tailstream/agent.yaml", // macOS system-wide
	}

	for _, path := range systemPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Fall back to current directory for development/testing
	return "tailstream.yaml"
}
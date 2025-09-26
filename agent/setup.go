package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// setupWizard runs an interactive setup wizard for first-time users
func setupWizard() error {
	fmt.Println("🚀 Tailstream Agent Setup")
	fmt.Println("Let's get you set up! This wizard will create a config file for easy future use.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)

	var streamID, accessToken string

	fmt.Print("Enter your Tailstream Stream ID: ")
	if scanner.Scan() {
		streamID = strings.TrimSpace(scanner.Text())
	}

	fmt.Print("Enter your Tailstream Access Token: ")
	if scanner.Scan() {
		accessToken = strings.TrimSpace(scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input: %v", err)
	}

	if streamID == "" || accessToken == "" {
		return fmt.Errorf("both Stream ID and Access Token are required")
	}

	// Create config
	cfg := Config{
		Env: "production",
		Discovery: struct {
			Enabled bool `yaml:"enabled"`
			Paths   struct {
				Include []string `yaml:"include"`
				Exclude []string `yaml:"exclude"`
			} `yaml:"paths"`
		}{
			Enabled: true,
			Paths: struct {
				Include []string `yaml:"include"`
				Exclude []string `yaml:"exclude"`
			}{
				Include: []string{
					"/var/log/nginx/*.log",
					"/var/log/caddy/*.log",
					"/var/log/apache2/*.log",
					"/var/log/httpd/*.log",
				},
				Exclude: []string{"**/*.gz", "**/*.1"},
			},
		},
		Streams: []StreamConfig{{
			Name:     "Default Stream",
			StreamID: streamID,
			Key:      accessToken,
			Paths: []string{
				"/var/log/nginx/*.log",
				"/var/log/caddy/*.log",
				"/var/log/apache2/*.log",
				"/var/log/httpd/*.log",
			},
			Exclude: []string{"**/*.gz", "**/*.1"},
		}},
	}

	// Save config to file
	configPath := getSystemConfigPath()
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	// Create directory if it doesn't exist
	if err := createConfigDir(configPath); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	fmt.Printf("\n✅ Configuration saved to %s\n", configPath)
	fmt.Println("🎉 Setup complete! You can now run the agent without any arguments.")
	fmt.Printf("\nTo start the agent: %s\n", os.Args[0])
	fmt.Printf("To run with debug output: %s --debug\n", os.Args[0])
	fmt.Println("\nThe agent will automatically discover and stream logs from common locations.")

	return nil
}

// getSystemConfigPath returns the appropriate system config path for the OS
func getSystemConfigPath() string {
	switch runtime.GOOS {
	case "linux":
		return "/etc/tailstream/agent.yaml"
	default:
		// Other platforms - use current directory
		return "tailstream.yaml"
	}
}

// createConfigDir creates the configuration directory if it doesn't exist
func createConfigDir(configPath string) error {
	dir := filepath.Dir(configPath)
	return os.MkdirAll(dir, 0755)
}

// needsSetup checks if initial setup is required
func needsSetup() bool {
	// Check if config file exists in system locations
	systemPaths := []string{
		"/etc/tailstream/agent.yaml",
		"/usr/local/etc/tailstream/agent.yaml",
		"tailstream.yaml", // Local fallback
	}

	for _, path := range systemPaths {
		if _, err := os.Stat(path); err == nil {
			return false
		}
	}

	// Check if environment variables are set
	if os.Getenv("TAILSTREAM_KEY") != "" {
		return false
	}

	return true
}

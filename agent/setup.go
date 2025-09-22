package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// setupWizard runs an interactive setup wizard for first-time users
func setupWizard() error {
	fmt.Println("🚀 Tailstream Agent Setup")
	fmt.Println("Let's get you set up! This wizard will create a config file for easy future use.")
	fmt.Println()

	var streamID, accessToken string

	fmt.Print("Enter your Tailstream Stream ID: ")
	fmt.Scanln(&streamID)

	fmt.Print("Enter your Tailstream Access Token: ")
	fmt.Scanln(&accessToken)

	if streamID == "" || accessToken == "" {
		return fmt.Errorf("both Stream ID and Access Token are required")
	}

	// Create config
	cfg := Config{
		Env: "production",
		Key: accessToken,
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
					"/var/www/**/storage/logs/*.log",
				},
				Exclude: []string{"**/*.gz", "**/*.1"},
			},
		},
		Ship: struct {
			URL      string `yaml:"url"`
			StreamID string `yaml:"stream_id"`
		}{
			URL:      fmt.Sprintf("https://app.tailstream.io/api/ingest/%s", streamID),
			StreamID: streamID,
		},
	}

	// Save config to file
	configPath := "tailstream.yaml"
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
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

// needsSetup checks if initial setup is required
func needsSetup() bool {
	// Check if config file exists
	if _, err := os.Stat("tailstream.yaml"); err == nil {
		return false
	}

	// Check if environment variables are set
	if os.Getenv("TAILSTREAM_KEY") != "" {
		return false
	}

	return true
}
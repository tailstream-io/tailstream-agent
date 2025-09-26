package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

const (
	defaultBaseURL = "https://app.tailstream.io"
	clientID       = "tailstream-agent"
)

// getBaseURL returns the base URL for API calls, checking environment variable first
func getBaseURL() string {
	if url := os.Getenv("TAILSTREAM_BASE_URL"); url != "" {
		return url
	}
	return defaultBaseURL
}

// DeviceCodeResponse represents the response from the device code request
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// TokenResponse represents the response from the token exchange
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
}

// Stream represents a user's stream from the API
type Stream struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	StreamID    string `json:"stream_id"`
	Description string `json:"description"`
	Ingest      struct {
		Token string `json:"token"`
	} `json:"ingest"`
}

// UserPlan represents the user's plan information
type UserPlan struct {
	PlanName string `json:"plan_name"`
	Limits   struct {
		MaxStreams int `json:"max_streams"`
	} `json:"limits"`
	CurrentUsage struct {
		Streams int `json:"streams"`
	} `json:"current_usage"`
	Permissions struct {
		CanCreateStreams bool `json:"can_create_streams"`
	} `json:"permissions"`
}

// StreamsResponse represents the API response for user streams
type StreamsResponse struct {
	Streams []Stream `json:"streams"`
}

// setupOAuth runs the OAuth Device Code Flow setup
func setupOAuth() error {
	fmt.Println("🚀 Tailstream Agent Setup")
	fmt.Println()

	// Step 1: Request device code
	deviceResp, err := requestDeviceCode()
	if err != nil {
		return fmt.Errorf("failed to request device code: %v", err)
	}

	// Step 2: Show user instructions
	fmt.Printf("Visit: %s\n", deviceResp.VerificationURI)
	fmt.Printf("Enter code: %s\n", deviceResp.UserCode)
	fmt.Println()
	fmt.Print("Waiting for authorization... ⏳")

	// Step 3: Poll for token
	token, err := pollForToken(deviceResp.DeviceCode, deviceResp.Interval)
	if err != nil {
		return fmt.Errorf("authorization failed: %v", err)
	}

	fmt.Println("\n✅ Connected to Tailstream!")
	fmt.Println()

	// Step 4: Fetch user's streams and plan
	streams, plan, err := fetchUserData(token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to fetch user data: %v", err)
	}

	// Step 5: Simplified stream selection/creation
	selectedStream, err := selectStream(streams, plan, token.AccessToken)
	if err != nil {
		return fmt.Errorf("stream selection failed: %v", err)
	}

	// Step 6: Save configuration
	cfg := createOAuthConfig([]StreamConfig{*selectedStream})
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %v", err)
	}

	fmt.Printf("✅ Stream configured: %s\n", selectedStream.Name)
	fmt.Println("🎉 Setup complete! Agent starting...")
	fmt.Println()

	// Show post-setup summary
	showSetupSummary()

	return nil
}

// requestDeviceCode initiates the OAuth Device Code Flow
func requestDeviceCode() (*DeviceCodeResponse, error) {
	data := url.Values{
		"client_id": {clientID},
		"scope":     {"stream:read stream:write stream:create"},
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm(getBaseURL()+"/api/oauth/device/code", data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code request failed: %s", resp.Status)
	}

	var deviceResp DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceResp); err != nil {
		return nil, err
	}

	return &deviceResp, nil
}

// pollForToken polls the token endpoint until authorization is complete
func pollForToken(deviceCode string, interval int) (*TokenResponse, error) {
	data := url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"device_code": {deviceCode},
		"client_id":   {clientID},
	}

	timeout := time.Now().Add(10 * time.Minute)

	client := &http.Client{Timeout: 10 * time.Second}

	for time.Now().Before(timeout) {
		resp, err := client.PostForm(getBaseURL()+"/api/oauth/device/token", data)
		if err != nil {
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}

		var tokenResp TokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
			resp.Body.Close()
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}
		resp.Body.Close()

		if tokenResp.Error == "authorization_pending" {
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}

		if tokenResp.Error != "" {
			return nil, fmt.Errorf("oauth error: %s", tokenResp.Error)
		}

		return &tokenResp, nil
	}

	return nil, fmt.Errorf("authorization timeout")
}

// fetchUserData retrieves the user's streams and plan information
func fetchUserData(accessToken string) ([]Stream, UserPlan, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Fetch streams
	req, err := http.NewRequest("GET", getBaseURL()+"/api/user/streams", nil)
	if err != nil {
		return nil, UserPlan{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, UserPlan{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, UserPlan{}, fmt.Errorf("failed to fetch streams: %s", resp.Status)
	}

	var streamsResp StreamsResponse
	if err := json.NewDecoder(resp.Body).Decode(&streamsResp); err != nil {
		return nil, UserPlan{}, err
	}

	// Fetch plan
	req, err = http.NewRequest("GET", getBaseURL()+"/api/user/plan", nil)
	if err != nil {
		return nil, UserPlan{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err = client.Do(req)
	if err != nil {
		return nil, UserPlan{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, UserPlan{}, fmt.Errorf("failed to fetch plan: %s", resp.Status)
	}

	var plan UserPlan
	if err := json.NewDecoder(resp.Body).Decode(&plan); err != nil {
		return nil, UserPlan{}, err
	}

	return streamsResp.Streams, plan, nil
}

// fetchStreamDetails retrieves detailed information for a specific stream including ingest token
func fetchStreamDetails(streamID string, accessToken string) (*Stream, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", getBaseURL()+"/api/streams/"+streamID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch stream details: %s", resp.Status)
	}

	var stream Stream
	if err := json.NewDecoder(resp.Body).Decode(&stream); err != nil {
		return nil, err
	}

	return &stream, nil
}

// selectStream handles the simplified stream selection/creation process
func selectStream(existingStreams []Stream, plan UserPlan, accessToken string) (*StreamConfig, error) {
	if len(existingStreams) == 0 {
		// No streams exist - let user create one
		fmt.Print("Enter stream name: ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		streamName := strings.TrimSpace(scanner.Text())

		if streamName == "" {
			return nil, fmt.Errorf("stream name is required")
		}

		newStream, err := createStreamWithName(streamName, accessToken)
		if err != nil {
			return nil, fmt.Errorf("failed to create stream: %v", err)
		}

		// Get log path from user
		logPath, err := getLogPathFromUser()
		if err != nil {
			return nil, fmt.Errorf("failed to get log path: %v", err)
		}

		newStream.Paths = []string{logPath}
		return newStream, nil
	}

	// Streams exist - let user pick one
	for i, stream := range existingStreams {
		desc := stream.Description
		if desc == "" {
			if len(stream.StreamID) > 8 {
				desc = stream.StreamID[:8] + "..."
			} else {
				desc = stream.StreamID
			}
		}
		fmt.Printf("[%d] %s (%s)\n", i+1, stream.Name, desc)
	}
	fmt.Println()

	fmt.Print("Enter selection: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	selection := strings.TrimSpace(scanner.Text())

	idx, err := strconv.Atoi(selection)
	if err != nil || idx < 1 || idx > len(existingStreams) {
		return nil, fmt.Errorf("invalid selection")
	}

	selectedStream := existingStreams[idx-1]

	// Fetch detailed stream info including ingest token
	streamDetails, err := fetchStreamDetails(selectedStream.StreamID, accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch stream details: %v", err)
	}

	// Get log path from user
	logPath, err := getLogPathFromUser()
	if err != nil {
		return nil, fmt.Errorf("failed to get log path: %v", err)
	}

	streamConfig := &StreamConfig{
		Name:     streamDetails.Name,
		StreamID: streamDetails.StreamID,
		Key:      streamDetails.Ingest.Token, // Use ingest token instead of OAuth token
		Paths:    []string{logPath},
		Exclude:  getDefaultExclusions(),
	}

	return streamConfig, nil
}

// createStreamWithName creates a new stream with the given name via the API
func createStreamWithName(name, accessToken string) (*StreamConfig, error) {
	// Create stream via API
	data := map[string]string{"name": name}
	jsonData, _ := json.Marshal(data)

	req, err := http.NewRequest("POST", getBaseURL()+"/api/streams", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body := make([]byte, 1024)
		n, _ := resp.Body.Read(body)
		return nil, fmt.Errorf("failed to create stream: %s - %s", resp.Status, string(body[:n]))
	}

	var stream Stream
	if err := json.NewDecoder(resp.Body).Decode(&stream); err != nil {
		return nil, err
	}

	// Fetch detailed stream info including ingest token
	streamDetails, err := fetchStreamDetails(stream.StreamID, accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch stream details after creation: %v", err)
	}

	return &StreamConfig{
		Name:     streamDetails.Name,
		StreamID: streamDetails.StreamID,
		Key:      streamDetails.Ingest.Token, // Use ingest token instead of OAuth token
		Paths:    getDefaultLogPaths(),
		Exclude:  getDefaultExclusions(),
	}, nil
}

// getLogPathFromUser prompts the user for a log path with auto-detected suggestions
func getLogPathFromUser() (string, error) {
	// Try to find existing log files
	detectedPaths := findExistingLogFiles()
	suggestedPath := getSuggestedLogPath(detectedPaths)

	if suggestedPath != "" {
		fmt.Printf("Select log path: %s\n", suggestedPath)
		fmt.Print("Press Enter to use this path, or type a different one: ")
	} else {
		fmt.Print("Select log path: /var/log/nginx/access.log\n")
		fmt.Print("Press Enter to use this path, or type a different one: ")
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())

	if input == "" {
		if suggestedPath != "" {
			return suggestedPath, nil
		}
		return "/var/log/nginx/access.log", nil
	}

	return input, nil
}

// getSuggestedLogPath analyzes detected paths and suggests wildcards when appropriate
func getSuggestedLogPath(detectedPaths []string) string {
	if len(detectedPaths) == 0 {
		return ""
	}

	if len(detectedPaths) == 1 {
		return detectedPaths[0]
	}

	// Group paths by directory to detect wildcard opportunities
	dirGroups := make(map[string][]string)
	for _, path := range detectedPaths {
		dir := filepath.Dir(path)
		dirGroups[dir] = append(dirGroups[dir], path)
	}

	// If multiple files in the same directory, suggest wildcard
	for dir, paths := range dirGroups {
		if len(paths) >= 2 {
			// Check if they're similar log files (same extension, similar names)
			if allSimilarLogFiles(paths) {
				return filepath.Join(dir, "*.log")
			}
		}
	}

	// Fallback to first detected path
	return detectedPaths[0]
}

// allSimilarLogFiles checks if all paths are log files in the same directory
func allSimilarLogFiles(paths []string) bool {
	if len(paths) < 2 {
		return false
	}

	for _, path := range paths {
		ext := filepath.Ext(path)
		if ext != ".log" {
			return false
		}
	}
	return true
}

// findExistingLogFiles attempts to find existing log files on the system
func findExistingLogFiles() []string {
	defaultPaths := getDefaultLogPaths()
	var foundPaths []string

	for _, pattern := range defaultPaths {
		matches, err := doublestar.FilepathGlob(pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			if !excluded(match, getDefaultExclusions()) {
				foundPaths = append(foundPaths, match)
				if len(foundPaths) >= 3 { // Limit to first few matches
					return foundPaths
				}
			}
		}
	}

	return foundPaths
}

// createOAuthConfig creates a clean configuration with only stream-specific tokens
func createOAuthConfig(streams []StreamConfig) Config {
	return Config{
		Env: "production",
		// No global Key field - each stream has its own ingest token
		Discovery: struct {
			Enabled bool `yaml:"enabled"`
			Paths   struct {
				Include []string `yaml:"include"`
				Exclude []string `yaml:"exclude"`
			} `yaml:"paths"`
		}{
			Enabled: true,
		},
		// No Ship section - legacy single-stream config not needed
		Updates: struct {
			Enabled       bool   `yaml:"enabled"`
			Channel       string `yaml:"channel"`
			CheckHours    int    `yaml:"check_hours"`
			MaxDelayHours int    `yaml:"max_delay_hours"`
		}{
			Enabled:       true,
			Channel:       "stable",
			CheckHours:    24,
			MaxDelayHours: 6,
		},
		Streams: streams,
	}
}

// getDefaultLogPaths returns the default log file patterns
func getDefaultLogPaths() []string {
	return []string{
		"/var/log/nginx/*.log",
		"/var/log/caddy/*.log",
		"/var/log/apache2/*.log",
		"/var/log/httpd/*.log",
	}
}

// getDefaultExclusions returns the default exclusion patterns
func getDefaultExclusions() []string {
	return []string{"**/*.gz", "**/*.1"}
}

// saveConfig saves the configuration to the appropriate system location
func saveConfig(cfg Config) error {
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

	fmt.Printf("✅ Configuration saved to %s\n", configPath)
	return nil
}

// showSetupSummary displays information about how the agent can be configured further
func showSetupSummary() {
	configPath := getSystemConfigPath()
	fmt.Println("📋 Agent Configuration Summary:")
	fmt.Printf("• Config file: %s\n", configPath)
	fmt.Println("• Stream configured with ingest token")
	fmt.Println("• Auto-updates: Enabled (checks daily)")
	fmt.Println()
	fmt.Println("🚀 Next Steps:")
	fmt.Println("• Start the service: sudo systemctl start tailstream-agent")
	fmt.Println("• Check status: sudo systemctl status tailstream-agent")
	fmt.Println("• View logs: journalctl -fu tailstream-agent")
	fmt.Println()
	fmt.Println("💡 Configuration:")
	fmt.Printf("• Edit %s to customize settings\n", configPath)
	fmt.Println("• Debug mode: tailstream-agent --debug")
}

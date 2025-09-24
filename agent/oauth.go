package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	baseURL  = "https://app.tailstream.io"
	clientID = "tailstream-agent"
)

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

	// Step 5: Interactive stream selection/creation
	selectedStreams, err := selectStreams(streams, plan, token.AccessToken)
	if err != nil {
		return fmt.Errorf("stream selection failed: %v", err)
	}

	// Step 6: Save configuration
	cfg := createOAuthConfig(token.AccessToken, selectedStreams)
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %v", err)
	}

	fmt.Printf("✅ Configured %d streams\n", len(selectedStreams))
	fmt.Println("🎉 Setup complete! Agent starting...")

	return nil
}

// requestDeviceCode initiates the OAuth Device Code Flow
func requestDeviceCode() (*DeviceCodeResponse, error) {
	data := url.Values{
		"client_id": {clientID},
		"scope":     {"stream:read stream:write stream:create"},
	}

	resp, err := http.PostForm(baseURL+"/api/oauth/device/code", data)
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

	for time.Now().Before(timeout) {
		resp, err := http.PostForm(baseURL+"/api/oauth/device/token", data)
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
	req, err := http.NewRequest("GET", baseURL+"/api/user/streams", nil)
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
	req, err = http.NewRequest("GET", baseURL+"/api/user/plan", nil)
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

// selectStreams handles the interactive stream selection/creation process
func selectStreams(existingStreams []Stream, plan UserPlan, accessToken string) ([]StreamConfig, error) {
	if len(existingStreams) == 0 {
		// New user - create first stream
		return createFirstStream(accessToken)
	}

	// Show existing streams
	fmt.Printf("Your streams (%d/%d", plan.CurrentUsage.Streams, plan.Limits.MaxStreams)
	if !plan.Permissions.CanCreateStreams {
		fmt.Print(" - upgrade for more")
	}
	fmt.Println("):")

	for i, stream := range existingStreams {
		desc := stream.Description
		if desc == "" {
			desc = stream.StreamID[:8] + "..."
		}
		fmt.Printf("[%d] %s (%s)\n", i+1, stream.Name, desc)
	}

	if plan.Permissions.CanCreateStreams {
		fmt.Printf("[%d] → Create new stream\n", len(existingStreams)+1)
	}
	fmt.Println()

	fmt.Print("Select streams to configure (comma-separated): ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	selections := strings.Split(scanner.Text(), ",")

	var selectedStreams []StreamConfig

	for _, sel := range selections {
		sel = strings.TrimSpace(sel)
		if sel == "" {
			continue
		}

		idx, err := strconv.Atoi(sel)
		if err != nil {
			continue
		}

		if idx >= 1 && idx <= len(existingStreams) {
			// Existing stream selected
			stream := existingStreams[idx-1]
			selectedStreams = append(selectedStreams, StreamConfig{
				Name:     stream.Name,
				StreamID: stream.StreamID,
				Paths:    getDefaultLogPaths(),
				Exclude:  getDefaultExclusions(),
			})
		} else if idx == len(existingStreams)+1 && plan.Permissions.CanCreateStreams {
			// Create new stream
			newStream, err := createNewStream(accessToken)
			if err != nil {
				fmt.Printf("Failed to create stream: %v\n", err)
				continue
			}
			selectedStreams = append(selectedStreams, *newStream)
		}
	}

	if len(selectedStreams) == 0 {
		return nil, fmt.Errorf("no streams selected")
	}

	return selectedStreams, nil
}

// createFirstStream creates a first stream for new users
func createFirstStream(accessToken string) ([]StreamConfig, error) {
	fmt.Println("Let's create your first stream!")

	newStream, err := createNewStream(accessToken)
	if err != nil {
		return nil, err
	}

	return []StreamConfig{*newStream}, nil
}

// createNewStream creates a new stream via the API
func createNewStream(accessToken string) (*StreamConfig, error) {
	fmt.Print("Enter name for new stream: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	name := strings.TrimSpace(scanner.Text())

	if name == "" {
		return nil, fmt.Errorf("stream name is required")
	}

	// Create stream via API
	data := map[string]string{"name": name}
	jsonData, _ := json.Marshal(data)

	req, err := http.NewRequest("POST", baseURL+"/api/streams", bytes.NewBuffer(jsonData))
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

	fmt.Printf("✅ Created stream: %s (stream_id: %s)\n", stream.Name, stream.StreamID)

	return &StreamConfig{
		Name:     stream.Name,
		StreamID: stream.StreamID,
		Paths:    getDefaultLogPaths(),
		Exclude:  getDefaultExclusions(),
	}, nil
}

// createOAuthConfig creates a configuration with OAuth token and selected streams
func createOAuthConfig(accessToken string, streams []StreamConfig) Config {
	return Config{
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
				Include: getDefaultLogPaths(),
				Exclude: getDefaultExclusions(),
			},
		},
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
		"/var/www/**/storage/logs/*.log",
	}
}

// getDefaultExclusions returns the default exclusion patterns
func getDefaultExclusions() []string {
	return []string{"**/*.gz", "**/*.1"}
}

// saveConfig saves the configuration to the appropriate system location
func saveConfig(cfg Config) error {
	configPath := getSystemConfigPath()

	// Import yaml at the top of the file
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
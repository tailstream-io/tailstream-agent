package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	GitHubRepo      = "tailstream-io/tailstream-agent"
	UpdateCheckFile = ".tailstream_last_update_check"
)

type GitHubRelease struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	HTMLURL    string `json:"html_url"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

type UpdateInfo struct {
	Available      bool
	CurrentVersion string
	LatestVersion  string
	DownloadURL    string
	ChecksumURL    string
}

func compareVersions(current, latest string) (bool, error) {
	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")

	if current == "dev" || current == "unknown" {
		return true, nil
	}

	currentParts := strings.Split(current, ".")
	latestParts := strings.Split(latest, ".")

	maxLen := len(currentParts)
	if len(latestParts) > maxLen {
		maxLen = len(latestParts)
	}

	for i := 0; i < maxLen; i++ {
		var currentVal, latestVal int
		var err error

		if i < len(currentParts) {
			currentVal, err = strconv.Atoi(currentParts[i])
			if err != nil {
				return false, fmt.Errorf("invalid current version format: %s", current)
			}
		}

		if i < len(latestParts) {
			latestVal, err = strconv.Atoi(latestParts[i])
			if err != nil {
				return false, fmt.Errorf("invalid latest version format: %s", latest)
			}
		}

		if latestVal > currentVal {
			return true, nil
		} else if latestVal < currentVal {
			return false, nil
		}
	}

	return false, nil
}

func getLatestRelease(channel string) (*GitHubRelease, error) {
	var url string

	switch channel {
	case "stable":
		// Use GitHub's latest release endpoint (excludes pre-releases)
		url = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", GitHubRepo)
	case "beta", "latest":
		// Get all releases and find the most recent (including pre-releases)
		url = fmt.Sprintf("https://api.github.com/repos/%s/releases", GitHubRepo)
	default:
		// Default to stable
		url = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", GitHubRepo)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", fmt.Sprintf("tailstream-agent/%s", Version))

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	if channel == "beta" || channel == "latest" {
		// Handle multiple releases response
		var releases []GitHubRelease
		if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
			return nil, err
		}

		if len(releases) == 0 {
			return nil, fmt.Errorf("no releases found")
		}

		// Find the latest appropriate release
		for _, release := range releases {
			if release.Draft {
				continue // Skip draft releases
			}

			if channel == "latest" {
				// Accept any non-draft release (stable or pre-release)
				return &release, nil
			} else if channel == "beta" && release.Prerelease {
				// Accept pre-releases for beta channel
				return &release, nil
			} else if channel == "beta" && !release.Prerelease {
				// Also accept stable releases for beta channel
				return &release, nil
			}
		}

		return nil, fmt.Errorf("no suitable releases found")
	} else {
		// Handle single release response (stable channel)
		var release GitHubRelease
		if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
			return nil, err
		}

		return &release, nil
	}
}

func shouldCheckForUpdates(cfg Config) bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return true
	}

	checkFile := fmt.Sprintf("%s/%s", homeDir, UpdateCheckFile)

	info, err := os.Stat(checkFile)
	if err != nil {
		return true
	}

	interval := time.Duration(cfg.Updates.CheckHours) * time.Hour
	return time.Since(info.ModTime()) > interval
}

func markUpdateCheckTime() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	checkFile := fmt.Sprintf("%s/%s", homeDir, UpdateCheckFile)

	file, err := os.Create(checkFile)
	if err != nil {
		return
	}
	file.Close()
}

func getBinaryName() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	if goos == "linux" && goarch == "amd64" {
		return "tailstream-agent-linux-amd64"
	} else if goos == "linux" && goarch == "arm64" {
		return "tailstream-agent-linux-arm64"
	}
	return ""
}

func checkWritePermission(dir string) error {
	// Try to create a temporary file to test write permissions
	testFile := filepath.Join(dir, ".tailstream-write-test")
	f, err := os.Create(testFile)
	if err != nil {
		return err
	}
	f.Close()
	os.Remove(testFile)
	return nil
}

func downloadFile(url, destination string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func verifyChecksum(filePath, checksumURL, filename string) error {
	// Download checksums file
	resp, err := http.Get(checksumURL)
	if err != nil {
		return fmt.Errorf("failed to download checksums: %v", err)
	}
	defer resp.Body.Close()

	checksumData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read checksums: %v", err)
	}

	// Parse checksums file and find our file
	lines := strings.Split(string(checksumData), "\n")
	var expectedChecksum string
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == filename {
			expectedChecksum = parts[0]
			break
		}
	}

	if expectedChecksum == "" {
		return fmt.Errorf("checksum not found for %s", filename)
	}

	// Calculate actual checksum
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	actualChecksum := hex.EncodeToString(hash.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}

func performSelfUpdate(updateInfo UpdateInfo) error {
	log.Printf("Starting self-update from %s to %s", updateInfo.CurrentVersion, updateInfo.LatestVersion)

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %v", err)
	}

	// Check if we have write permission to the directory
	execDir := filepath.Dir(execPath)
	if err := checkWritePermission(execDir); err != nil {
		// For typical installations in /usr/local/bin, provide helpful guidance
		if strings.Contains(execDir, "/usr/local/bin") || strings.Contains(execDir, "/usr/bin") {
			return fmt.Errorf("cannot update binary in %s (permission denied)\n\n"+
				"The agent is installed system-wide but running as a non-root user.\n"+
				"To update, run one of these commands:\n\n"+
				"  sudo systemctl stop tailstream-agent\n"+
				"  sudo tailstream-agent update\n"+
				"  sudo systemctl start tailstream-agent\n\n"+
				"Or use the installer:\n"+
				"  curl -fsSL https://install.tailstream.io | sudo bash\n\n"+
				"Original error: %v", execDir, err)
		}

		// For /opt/tailstream installations, this should work with proper ownership
		if strings.Contains(execDir, "/opt/tailstream") {
			return fmt.Errorf("cannot update binary in %s (permission denied)\n\n"+
				"This appears to be a /opt/tailstream installation but permissions are incorrect.\n"+
				"The /opt/tailstream directory should be owned by the tailstream user.\n"+
				"Try reinstalling with: curl -fsSL https://install.tailstream.io | sudo bash\n\n"+
				"Original error: %v", execDir, err)
		}

		return fmt.Errorf("insufficient permissions to update binary in %s: %v", execDir, err)
	}

	// Download new binary to temp location
	tempFile := filepath.Join(execDir, ".tailstream-agent.update.tmp")
	if err := downloadFile(updateInfo.DownloadURL, tempFile); err != nil {
		return fmt.Errorf("failed to download update: %v", err)
	}
	defer os.Remove(tempFile)

	// Verify checksum if available
	if updateInfo.ChecksumURL != "" {
		binaryName := getBinaryName()
		if err := verifyChecksum(tempFile, updateInfo.ChecksumURL, binaryName); err != nil {
			return fmt.Errorf("checksum verification failed: %v", err)
		}
		log.Printf("Checksum verification passed")
	}

	// Make the new binary executable
	if err := os.Chmod(tempFile, 0755); err != nil {
		return fmt.Errorf("failed to make binary executable: %v", err)
	}

	// Atomic replacement: rename temp file to actual executable
	if err := os.Rename(tempFile, execPath); err != nil {
		return fmt.Errorf("failed to replace binary: %v", err)
	}

	log.Printf("Binary updated successfully to %s", updateInfo.LatestVersion)

	// If running under systemd, request restart
	if os.Getenv("SYSTEMD_EXEC_PID") != "" || isSystemdService() {
		log.Printf("Detected systemd service, requesting restart")
		return requestSystemdRestart()
	}

	return nil
}

func isSystemdService() bool {
	// Check if we're running under systemd
	if _, err := os.Stat("/proc/1/comm"); err != nil {
		return false
	}

	comm, err := os.ReadFile("/proc/1/comm")
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(comm)) == "systemd"
}

func requestSystemdRestart() error {
	// Send SIGTERM to ourselves to trigger systemd restart
	pid := os.Getpid()
	cmd := exec.Command("kill", "-TERM", fmt.Sprintf("%d", pid))
	return cmd.Run()
}

func checkForUpdatesForce(cfg Config, force bool) {
	// Check environment variable to disable updates (useful for testing)
	if os.Getenv("TAILSTREAM_DISABLE_UPDATES") == "1" || os.Getenv("TAILSTREAM_DISABLE_UPDATES") == "true" {
		if os.Getenv("DEBUG") == "1" {
			log.Printf("Updates disabled by environment variable")
		}
		return
	}

	if !cfg.Updates.Enabled && !force {
		if os.Getenv("DEBUG") == "1" {
			log.Printf("Updates disabled and not forced")
		}
		return
	}

	if !force && !shouldCheckForUpdates(cfg) {
		if os.Getenv("DEBUG") == "1" {
			log.Printf("Skipping update check (should_check=false)")
		}
		return
	}

	release, err := getLatestRelease(cfg.Updates.Channel)
	if err != nil {
		if os.Getenv("DEBUG") == "1" {
			log.Printf("Failed to check for updates: %v", err)
		}
		return
	}

	hasUpdate, err := compareVersions(Version, release.TagName)
	if err != nil {
		if os.Getenv("DEBUG") == "1" {
			log.Printf("Failed to compare versions: %v", err)
		}
		return
	}

	markUpdateCheckTime()

	if !hasUpdate {
		return
	}

	// Find download URL for current platform
	binaryName := getBinaryName()
	if binaryName == "" {
		log.Printf("Auto-update not supported on %s/%s platform", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("\n🔄 Update available: %s → %s\n", Version, release.TagName)
		fmt.Printf("📥 Download: %s\n", release.HTMLURL)
		fmt.Printf("⚡ Quick install: curl -fsSL https://install.tailstream.io | bash\n\n")
		return
	}

	updateInfo := UpdateInfo{
		Available:      true,
		CurrentVersion: Version,
		LatestVersion:  release.TagName,
	}

	// Find binary and checksum URLs in release assets
	for _, asset := range release.Assets {
		switch asset.Name {
		case binaryName:
			updateInfo.DownloadURL = asset.BrowserDownloadURL
		case "checksums.txt":
			updateInfo.ChecksumURL = asset.BrowserDownloadURL
		}
	}

	if updateInfo.DownloadURL == "" {
		log.Printf("Binary not found in release assets for %s", binaryName)
		return
	}


	// Perform the self-update
	if err := performSelfUpdate(updateInfo); err != nil {
		log.Printf("Self-update failed: %v", err)

		// Show user-friendly error message only in interactive mode
		if os.Getenv("DEBUG") == "1" {
			fmt.Printf("\n🔄 Update available: %s → %s\n", Version, release.TagName)
			fmt.Printf("❌ Auto-update failed: %v\n", err)
			fmt.Printf("📥 Manual download: %s\n", release.HTMLURL)
			fmt.Printf("⚡ Quick install: curl -fsSL https://install.tailstream.io | bash\n\n")
		}
	} else {
		log.Printf("Successfully updated from %s to %s", updateInfo.CurrentVersion, updateInfo.LatestVersion)
	}
}

func checkForUpdates(cfg Config) {
	checkForUpdatesForce(cfg, false)
}

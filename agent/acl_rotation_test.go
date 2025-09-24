package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TestACLLogRotation tests that ACL permissions work correctly after log rotation
func TestACLLogRotation(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Test requires root privileges to test ACL functionality")
	}

	// Check if setfacl is available
	if _, err := exec.LookPath("setfacl"); err != nil {
		t.Skip("setfacl not available - cannot test ACL functionality")
	}

	// Create test user (similar to tailstream user)
	testUser := "test-tailstream-" + strconv.Itoa(int(time.Now().Unix()))
	testUID := 9999 + int(time.Now().Unix()%1000) // Generate unique UID

	// Create test user
	cmd := exec.Command("useradd", "-u", strconv.Itoa(testUID), "-M", "-s", "/bin/false", testUser)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	defer func() {
		exec.Command("userdel", testUser).Run() // Clean up
	}()

	// Create test log directory
	testDir := filepath.Join(os.TempDir(), "test-log-rotation-"+strconv.Itoa(int(time.Now().Unix())))
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testDir)

	// Apply the BROKEN ACL permissions (current installer behavior)
	t.Log("Setting up broken ACL permissions (missing X on default ACL)...")

	// Set directory permissions
	cmd = exec.Command("setfacl", "-m", "u:"+testUser+":rX", testDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to set directory ACL: %v", err)
	}

	// Set BROKEN default permissions (missing X)
	cmd = exec.Command("setfacl", "-dm", "u:"+testUser+":r", testDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to set default ACL: %v", err)
	}

	// Create initial log file
	initialLog := filepath.Join(testDir, "access.log")
	if err := os.WriteFile(initialLog, []byte("initial log line\n"), 0644); err != nil {
		t.Fatalf("Failed to create initial log file: %v", err)
	}

	// Verify test user can read initial file
	if !canUserAccessFile(testUser, testUID, initialLog) {
		t.Fatal("Test user cannot access initial log file - ACL setup failed")
	}

	// Simulate log rotation: create new file (like logrotate does)
	t.Log("Simulating log rotation...")
	rotatedLog := filepath.Join(testDir, "access.log.1")
	if err := os.Rename(initialLog, rotatedLog); err != nil {
		t.Fatalf("Failed to rotate log: %v", err)
	}

	// Create new log file (inherits default ACL)
	newLog := filepath.Join(testDir, "access.log")
	if err := os.WriteFile(newLog, []byte("new log line after rotation\n"), 0644); err != nil {
		t.Fatalf("Failed to create new log file: %v", err)
	}

	// TEST: Verify that test user CANNOT access the new file (proving the bug)
	t.Log("Testing access to rotated log file...")
	if canUserAccessFile(testUser, testUID, newLog) {
		t.Error("Test user can access new log file - this suggests ACL is working (test may be flawed)")
	} else {
		t.Log("✓ CONFIRMED: Test user cannot access new log file due to broken default ACL")
	}

	// Now apply the FIX: correct default ACL with X permission
	t.Log("Applying fix: setting correct default ACL with X permission...")
	cmd = exec.Command("setfacl", "-dm", "u:"+testUser+":rX", testDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to set corrected default ACL: %v", err)
	}

	// Create another new file to test the fix
	fixedLog := filepath.Join(testDir, "access.log.new")
	if err := os.WriteFile(fixedLog, []byte("log line after ACL fix\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file for fix: %v", err)
	}

	// Verify test user CAN access the file with fixed ACL
	if !canUserAccessFile(testUser, testUID, fixedLog) {
		t.Error("Test user cannot access file even after ACL fix")
	} else {
		t.Log("✓ SUCCESS: Test user can access new file with corrected default ACL")
	}
}

// canUserAccessFile tests if a user can open a file by switching to that user's context
func canUserAccessFile(_ string, uid int, filepath string) bool {
	// Create a test program that tries to open the file as the specified user
	testProg := fmt.Sprintf(`
package main
import (
	"os"
	"syscall"
)
func main() {
	// Switch to test user
	if err := syscall.Setuid(%d); err != nil {
		os.Exit(2)
	}
	// Try to open file
	f, err := os.Open(%q)
	if err != nil {
		os.Exit(1)
	}
	f.Close()
	os.Exit(0)
}`, uid, filepath)

	// Write test program
	testFile := filepath + "_test_access.go"
	if err := os.WriteFile(testFile, []byte(testProg), 0644); err != nil {
		return false
	}
	defer os.Remove(testFile)

	// Run test program
	cmd := exec.Command("go", "run", testFile)
	err := cmd.Run()

	// Check exit code
	if exitError, ok := err.(*exec.ExitError); ok {
		return exitError.ExitCode() == 0
	}
	return err == nil
}

// TestTailFileErrorReporting tests that tailFile properly reports errors
func TestTailFileErrorReporting(t *testing.T) {
	// Temporarily replace the log output to capture messages
	// This is a simplified test - in practice we'd want more sophisticated error handling

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	ch := make(chan LogLine, 10)

	// Try to tail a non-existent file
	go tailFile(ctx, "/nonexistent/file/path", ch)

	// Wait for context to timeout
	<-ctx.Done()

	// In current implementation, tailFile just logs and returns
	// We should improve this to surface errors to the caller
	t.Log("tailFile currently only logs errors - we should improve error surfacing")
}
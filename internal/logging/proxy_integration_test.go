//go:build integration

// pattern: Imperative Shell

package logging

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestProxyLogReader_Integration tests file tailing with real file operations.
// This is an integration test (not unit test) because it involves real file I/O.
// Run with: go test -tags=integration ./internal/logging/...
func TestProxyLogReader_FileCreation(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "requests.jsonl")

	// Create a ChannelSink using the constructor (properly initializes mutex)
	sink := NewChannelSink(100)

	// Create reader (file doesn't exist yet)
	reader, err := NewProxyLogReader(logFile, "test-container", sink)
	if err != nil {
		t.Fatalf("NewProxyLogReader failed: %v", err)
	}

	// Start reader in goroutine
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go func() {
		if err := reader.Start(ctx); err != nil && err != context.DeadlineExceeded && err != context.Canceled {
			t.Errorf("Start failed: %v", err)
		}
	}()

	// Wait for watcher to be ready
	time.Sleep(200 * time.Millisecond)

	// Create the file first (empty)
	if err := os.WriteFile(logFile, []byte{}, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Give fsnotify time to detect file creation
	time.Sleep(200 * time.Millisecond)

	// Now append a log entry to the file
	logLine := `{"ts": 1707235200.123, "method": "GET", "url": "https://api.example.com/test", "status": 200, "duration_ms": 45, "req_headers": {}, "res_headers": {}, "req_body": null, "res_body": null}` + "\n"
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}
	_, err = f.WriteString(logLine)
	f.Close()
	if err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}

	// Wait for entry to be received (with timeout)
	select {
	case entry := <-sink.Entries():
		if entry.Message != "200 GET https://api.example.com/test 45ms" {
			t.Errorf("Unexpected message: %s", entry.Message)
		}
		if entry.Scope != "proxy.test-container" {
			t.Errorf("Unexpected scope: %s", entry.Scope)
		}
	case <-time.After(6 * time.Second): // polling interval is 5s
		t.Error("Timeout waiting for log entry")
	}

	cancel()
	reader.Close()
}

// TestProxyLogReader_FileAppend tests that appended lines are picked up.
func TestProxyLogReader_FileAppend(t *testing.T) {
	// Create temp directory and initial file
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "requests.jsonl")

	// Create initial empty file
	if err := os.WriteFile(logFile, []byte{}, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Create reader using ChannelSink constructor (properly initializes mutex)
	sink := NewChannelSink(100)

	reader, err := NewProxyLogReader(logFile, "test-container", sink)
	if err != nil {
		t.Fatalf("NewProxyLogReader failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	go func() {
		reader.Start(ctx)
	}()

	// Wait for watcher to be ready
	time.Sleep(100 * time.Millisecond)

	// Append multiple lines
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}

	line1 := `{"ts": 1707235200.0, "method": "POST", "url": "https://api.example.com/a", "status": 201, "duration_ms": 100, "req_headers": {}, "res_headers": {}, "req_body": null, "res_body": null}` + "\n"
	line2 := `{"ts": 1707235201.0, "method": "GET", "url": "https://api.example.com/b", "status": 404, "duration_ms": 50, "req_headers": {}, "res_headers": {}, "req_body": null, "res_body": null}` + "\n"

	f.WriteString(line1)
	f.WriteString(line2)
	f.Close()

	// Wait for entries
	receivedCount := 0
	timeout := time.After(6 * time.Second)

	for receivedCount < 2 {
		select {
		case entry := <-sink.Entries():
			receivedCount++
			t.Logf("Received entry %d: %s", receivedCount, entry.Message)
		case <-timeout:
			t.Errorf("Timeout: received only %d of 2 entries", receivedCount)
			cancel()
			reader.Close()
			return
		}
	}

	if receivedCount != 2 {
		t.Errorf("Expected 2 entries, got %d", receivedCount)
	}

	cancel()
	reader.Close()
}

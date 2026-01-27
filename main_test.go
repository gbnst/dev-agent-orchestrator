package main

import (
	"os"
	"path/filepath"
	"testing"

	"devagent/internal/logging"
)

func TestLogManagerInitialization(t *testing.T) {
	// Create temp dir for logs
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Initialize LogManager with test config
	lm, err := logging.NewManager(logging.Config{
		FilePath:       logPath,
		MaxSizeMB:      1,
		MaxBackups:     1,
		MaxAgeDays:     1,
		ChannelBufSize: 10,
		Level:          "debug",
	})
	if err != nil {
		t.Fatalf("failed to create LogManager: %v", err)
	}
	defer lm.Close()

	// Get root logger and write a message
	logger := lm.For("app")
	logger.Info("test message")

	// Sync to flush
	lm.Sync()

	// Verify log file was created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("log file was not created")
	}

	// Verify channel receives entry
	select {
	case entry := <-lm.Entries():
		if entry.Scope != "app" {
			t.Errorf("expected scope 'app', got %q", entry.Scope)
		}
		if entry.Message != "test message" {
			t.Errorf("expected message 'test message', got %q", entry.Message)
		}
	default:
		t.Error("no log entry received on channel")
	}
}

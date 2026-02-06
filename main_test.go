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
	defer func() { _ = lm.Close() }()

	// Get root logger and write a message
	logger := lm.For("app")
	logger.Info("test message")

	// Sync to flush
	_ = lm.Sync()

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

func TestExtractProjectHash(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid app container",
			input:    "devagent-abc123def456-app",
			expected: "abc123def456",
		},
		{
			name:     "valid proxy container",
			input:    "devagent-abc123def456-proxy",
			expected: "abc123def456",
		},
		{
			name:     "too short",
			input:    "devagent-abc",
			expected: "",
		},
		{
			name:     "no prefix",
			input:    "mycontainer-abc123def456-app",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "just prefix no hash",
			input:    "devagent-",
			expected: "",
		},
		{
			name:     "hash exactly 12 chars with suffix",
			input:    "devagent-abcdef123456-x",
			expected: "abcdef123456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractProjectHash(tt.input)
			if got != tt.expected {
				t.Errorf("extractProjectHash(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// pattern: Imperative Shell

package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := Config{
		FilePath:   logFile,
		MaxSizeMB:  10,
		MaxBackups: 5,
		MaxAgeDays: 7,
		Level:      "debug",
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer func() { _ = mgr.Close() }()

	// File may not exist until first write, that's OK
	_, _ = os.Stat(logFile)

	// Verify entries channel is available
	if mgr.Entries() == nil {
		t.Error("Entries() returned nil")
	}
}

func TestManager_For(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := Config{
		FilePath:   logFile,
		MaxSizeMB:  10,
		MaxBackups: 5,
		MaxAgeDays: 7,
		Level:      "debug",
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer func() { _ = mgr.Close() }()

	// Get named logger
	logger := mgr.For("container.abc123")
	if logger == nil {
		t.Fatal("For() returned nil")
	}

	// Same scope should return same logger (cached)
	logger2 := mgr.For("container.abc123")
	if logger != logger2 {
		t.Error("For() should return cached logger for same scope")
	}

	// Different scope should return different logger
	logger3 := mgr.For("container.xyz789")
	if logger == logger3 {
		t.Error("For() should return different logger for different scope")
	}
}

func TestManager_LoggingToChannel(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := Config{
		FilePath:       logFile,
		MaxSizeMB:      10,
		MaxBackups:     5,
		MaxAgeDays:     7,
		Level:          "debug",
		ChannelBufSize: 100,
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer func() { _ = mgr.Close() }()

	// Log a message
	logger := mgr.For("test.component")
	logger.Info("test message", "key", "value")

	// Sync to ensure write completes
	_ = mgr.Sync()

	// Check channel received entry (non-blocking since Sync already completed)
	select {
	case entry := <-mgr.Entries():
		if entry.Message != "test message" {
			t.Errorf("Message = %q, want %q", entry.Message, "test message")
		}
		if entry.Scope != "test.component" {
			t.Errorf("Scope = %q, want %q", entry.Scope, "test.component")
		}
	default:
		t.Fatal("entry not received on channel after Sync()")
	}
}

func TestManager_LoggingToFile(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := Config{
		FilePath:   logFile,
		MaxSizeMB:  10,
		MaxBackups: 5,
		MaxAgeDays: 7,
		Level:      "debug",
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Log a message
	logger := mgr.For("file.test")
	logger.Info("file test message")

	// Close to flush
	_ = mgr.Close()

	// Check file contains entry
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "file test message") {
		t.Errorf("log file should contain message, got: %s", content)
	}
	if !strings.Contains(content, "file.test") {
		t.Errorf("log file should contain scope, got: %s", content)
	}
}

func TestManager_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := Config{
		FilePath:   logFile,
		MaxSizeMB:  10,
		MaxBackups: 5,
		MaxAgeDays: 7,
		Level:      "debug",
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer func() { _ = mgr.Close() }()

	// Create some loggers
	mgr.For("container.abc")
	mgr.For("container.xyz")
	mgr.For("session.abc.s1")

	// Cleanup container.abc and its sessions
	mgr.Cleanup("container.abc")

	// container.abc should be removed from cache
	// But we can't easily test internal cache state without exporting it
	// Just verify no panic and logger still works after cleanup
	logger := mgr.For("container.abc")
	logger.Info("after cleanup")
}

func TestManager_FileRotation(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "rotate.log")

	// Use tiny max size to trigger rotation
	cfg := Config{
		FilePath:   logFile,
		MaxSizeMB:  1, // 1MB - smallest practical size
		MaxBackups: 2,
		MaxAgeDays: 7,
		Level:      "debug",
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer func() { _ = mgr.Close() }()

	logger := mgr.For("rotation.test")

	// Write enough data to potentially trigger rotation
	// This is more of a smoke test - actual rotation happens at file level
	bigMessage := string(make([]byte, 1000))
	for i := range 100 {
		logger.Info(bigMessage, "iteration", i)
	}

	_ = mgr.Sync()

	// Verify file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("log file should exist after writing")
	}
}

func TestManager_GetChannelSink(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := Config{
		FilePath:   logFile,
		MaxSizeMB:  10,
		MaxBackups: 5,
		MaxAgeDays: 7,
		Level:      "debug",
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer func() { _ = mgr.Close() }()

	// GetChannelSink should return non-nil
	sink := mgr.GetChannelSink()
	if sink == nil {
		t.Error("GetChannelSink() returned nil")
	}

	// Sink should be usable for sending entries
	entry := LogEntry{
		Level:   "INFO",
		Scope:   "test",
		Message: "test message",
		Fields:  make(map[string]any),
	}
	sink.Send(entry)

	// Entry should be available on Entries() channel
	select {
	case got := <-mgr.Entries():
		if got.Message != "test message" {
			t.Errorf("Message = %q, want %q", got.Message, "test message")
		}
	default:
		t.Fatal("entry not received on channel")
	}
}

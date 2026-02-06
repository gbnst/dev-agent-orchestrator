package tui

import (
	"testing"

	"devagent/internal/config"
	"devagent/internal/logging"
)

func TestModel_LogsInitialization(t *testing.T) {
	cfg := &config.Config{
		Theme:   "mocha",
		Runtime: "docker",
	}

	// Create a real LogManager for testing
	logPath := t.TempDir() + "/test.log"
	lm, err := logging.NewManager(logging.Config{
		FilePath:       logPath,
		MaxSizeMB:      1,
		MaxBackups:     1,
		MaxAgeDays:     1,
		ChannelBufSize: 100,
		Level:          "debug",
	})
	if err != nil {
		t.Fatalf("failed to create LogManager: %v", err)
	}
	defer func() { _ = lm.Close() }()

	model := NewModel(cfg, lm)

	// Collect all entries during initialization
	// Container manager logs first, then TUI logs
	foundTUI := false
	foundContainer := false

drainLoop:
	for range 2 {
		select {
		case entry := <-lm.Entries():
			if entry.Scope == "tui" {
				foundTUI = true
			}
			if entry.Scope == "container" {
				foundContainer = true
			}
		default:
			break drainLoop
		}
	}

	if !foundTUI {
		t.Error("expected to find 'tui' scope log entry")
	}
	if !foundContainer {
		t.Error("expected to find 'container' scope log entry")
	}

	_ = model // Use model to avoid unused variable
}

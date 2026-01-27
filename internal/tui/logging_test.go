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
		ChannelBufSize: 10,
		Level:          "debug",
	})
	if err != nil {
		t.Fatalf("failed to create LogManager: %v", err)
	}
	defer lm.Close()

	model := NewModel(cfg, lm)

	// Model should have logged initialization
	select {
	case entry := <-lm.Entries():
		if entry.Scope != "tui" {
			t.Errorf("expected scope 'tui', got %q", entry.Scope)
		}
	default:
		t.Error("no initialization log entry received")
	}

	_ = model // Use model to avoid unused variable
}

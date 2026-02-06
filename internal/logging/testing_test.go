// pattern: Imperative Shell

package logging

import (
	"testing"
)

func TestNopLogger(t *testing.T) {
	logger := NopLogger()
	if logger == nil {
		t.Fatal("NopLogger() returned nil")
	}

	// Should not panic
	logger.Debug("test")
	logger.Info("test")
	logger.Warn("test")
	logger.Error("test")
}

func TestNopLogger_With(t *testing.T) {
	logger := NopLogger()
	withLogger := logger.With("key", "value")
	if withLogger == nil {
		t.Fatal("With() returned nil")
	}

	// Should not panic
	withLogger.Info("test with fields")
}

func TestNewTestLogManager(t *testing.T) {
	lm := NewTestLogManager(10)
	if lm == nil {
		t.Fatal("NewTestLogManager() returned nil")
	}
	defer func() { _ = lm.Close() }()

	// Get logger and write
	logger := lm.For("test")
	logger.Info("test message")

	// Should receive entry on channel
	select {
	case entry := <-lm.Channel():
		if entry.Message != "test message" {
			t.Errorf("expected 'test message', got %q", entry.Message)
		}
		if entry.Scope != "test" {
			t.Errorf("expected scope 'test', got %q", entry.Scope)
		}
	default:
		t.Error("no entry received on channel")
	}
}

func TestNewTestLogManager_Channel(t *testing.T) {
	lm := NewTestLogManager(5)
	defer func() { _ = lm.Close() }()

	ch := lm.Channel()
	if ch == nil {
		t.Error("Channel() returned nil")
	}
}

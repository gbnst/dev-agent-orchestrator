package container

import (
	"context"
	"testing"

	"devagent/internal/logging"
)

func TestManager_LogsInitialization(t *testing.T) {
	lm := logging.NewTestLogManager(100)
	defer lm.Close()

	mockRuntime := &mockRuntime{
		containers: []Container{
			{ID: "abc123", Name: "test-container", State: StateRunning},
		},
	}

	_ = NewManagerWithRuntimeAndLogger(mockRuntime, lm)

	// Should have logged initialization
	select {
	case entry := <-lm.Channel():
		if entry.Scope != "container" {
			t.Errorf("expected scope 'container', got %q", entry.Scope)
		}
	default:
		t.Error("no initialization log entry received")
	}
}

func TestManager_LogsRefresh(t *testing.T) {
	lm := logging.NewTestLogManager(100)
	defer lm.Close()

	mockRuntime := &mockRuntime{
		containers: []Container{
			{ID: "abc123", Name: "test-container", State: StateRunning},
		},
	}

	manager := NewManagerWithRuntimeAndLogger(mockRuntime, lm)

	// Drain initialization log
	<-lm.Channel()

	// Refresh should log
	ctx := context.Background()
	_ = manager.Refresh(ctx)

	select {
	case entry := <-lm.Channel():
		if entry.Scope != "container" {
			t.Errorf("expected scope 'container', got %q", entry.Scope)
		}
	default:
		t.Error("no refresh log entry received")
	}
}

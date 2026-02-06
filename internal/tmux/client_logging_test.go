package tmux

import (
	"context"
	"testing"

	"devagent/internal/logging"
)

func TestClient_LogsOperations(t *testing.T) {
	lm := logging.NewTestLogManager(100)
	defer func() { _ = lm.Close() }()

	mockExec := func(ctx context.Context, containerID string, cmd []string) (string, error) {
		return "session1: 1 windows (created Mon Jan 20 10:00:00 2025)\nsession2: 2 windows (created Mon Jan 20 09:00:00 2025) (attached)\n", nil
	}

	client := NewClientWithLogger(mockExec, lm)

	// Drain initialization log
	<-lm.Channel()

	// ListSessions should log
	_, _ = client.ListSessions(context.Background(), "container123")

	select {
	case entry := <-lm.Channel():
		if entry.Scope != "tmux" {
			t.Errorf("expected scope 'tmux', got %q", entry.Scope)
		}
	default:
		t.Error("no log entry received for ListSessions")
	}
}

package tui

import (
	"fmt"
	"testing"

	"devagent/internal/logging"
)

func TestTabMode_String(t *testing.T) {
	tests := []struct {
		tab  TabMode
		want string
	}{
		{TabContainers, "Containers"},
		{TabSessions, "Sessions"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.tab.String(); got != tt.want {
				t.Errorf("TabMode.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestModel_HasTabs(t *testing.T) {
	// After this phase, Model should have currentTab field
	// This is a compile-time check more than runtime
	m := Model{
		currentTab:   TabContainers,
		logPanelOpen: false,
	}

	if m.currentTab != TabContainers {
		t.Errorf("currentTab = %v, want %v", m.currentTab, TabContainers)
	}
}

func TestStatusLevel_String(t *testing.T) {
	tests := []struct {
		level StatusLevel
		want  string
	}{
		{StatusInfo, "info"},
		{StatusSuccess, "success"},
		{StatusError, "error"},
		{StatusLoading, "loading"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("StatusLevel.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestModel_PendingOperations(t *testing.T) {
	m := newTestModel()

	// Initially empty
	if len(m.pendingOperations) != 0 {
		t.Error("pendingOperations should be empty initially")
	}

	// Can add pending operation
	m.setPending("abc123", "start")
	if op, ok := m.pendingOperations["abc123"]; !ok || op != "start" {
		t.Errorf("pendingOperations[abc123] = %q, want 'start'", op)
	}

	// Can check if pending
	if !m.isPending("abc123") {
		t.Error("abc123 should be pending")
	}
	if m.isPending("xyz789") {
		t.Error("xyz789 should not be pending")
	}

	// Can clear pending operation
	m.clearPending("abc123")
	if m.isPending("abc123") {
		t.Error("abc123 should not be pending after clear")
	}
}

func TestModel_LogEntries(t *testing.T) {
	m := newTestModel()

	// Initially empty
	if len(m.logEntries) != 0 {
		t.Error("logEntries should be empty initially")
	}

	// Add entries
	entry1 := logging.LogEntry{Message: "test1", Scope: "app"}
	entry2 := logging.LogEntry{Message: "test2", Scope: "container.abc"}

	m.addLogEntry(entry1)
	m.addLogEntry(entry2)

	if len(m.logEntries) != 2 {
		t.Errorf("logEntries length = %d, want 2", len(m.logEntries))
	}
}

func TestModel_LogEntriesRingBuffer(t *testing.T) {
	m := newTestModel()

	// Add more than max entries
	for i := 0; i < 1050; i++ {
		m.addLogEntry(logging.LogEntry{Message: fmt.Sprintf("entry %d", i), Scope: "app"})
	}

	// Should be capped at 1000
	if len(m.logEntries) > 1000 {
		t.Errorf("logEntries length = %d, should be capped at 1000", len(m.logEntries))
	}
}

func TestModel_FilteredLogEntries(t *testing.T) {
	m := newTestModel()

	m.addLogEntry(logging.LogEntry{Message: "app log", Scope: "app"})
	m.addLogEntry(logging.LogEntry{Message: "container log", Scope: "container.abc123"})
	m.addLogEntry(logging.LogEntry{Message: "session log", Scope: "session.abc123.dev"})

	// No filter = all entries
	m.logFilter = ""
	if len(m.filteredLogEntries()) != 3 {
		t.Errorf("no filter should return all entries, got %d", len(m.filteredLogEntries()))
	}

	// Container filter
	m.logFilter = "container.abc123"
	filtered := m.filteredLogEntries()
	if len(filtered) != 1 {
		t.Errorf("container filter should return 1 entry, got %d", len(filtered))
	}
}

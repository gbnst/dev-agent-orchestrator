package tui

import (
	"fmt"
	"testing"

	"devagent/internal/container"
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

func TestSetLogFilterFromContext_NoContainerSelected(t *testing.T) {
	m := newTestModel()
	m.selectedContainer = nil
	m.currentTab = TabContainers

	m.setLogFilterFromContext()

	if m.logFilter != "" {
		t.Errorf("logFilter should be empty when no container selected, got %q", m.logFilter)
	}
}

func TestSetLogFilterFromContext_ContainerSelectedInContainersTab(t *testing.T) {
	m := newTestModel()
	m.selectedContainer = &container.Container{
		ID:   "abc123456789abcdef",
		Name: "test-container",
	}
	m.currentTab = TabContainers

	m.setLogFilterFromContext()

	expected := "container.abc123456789"
	if m.logFilter != expected {
		t.Errorf("logFilter = %q, want %q", m.logFilter, expected)
	}
}

func TestSetLogFilterFromContext_ContainerWithSessionInSessionsTab(t *testing.T) {
	m := newTestModel()
	m.selectedContainer = &container.Container{
		ID:   "abc123456789abcdef",
		Name: "test-container",
		Sessions: []container.Session{
			{Name: "dev", Windows: 1},
			{Name: "build", Windows: 2},
		},
	}
	m.currentTab = TabSessions
	m.selectedSessionIdx = 0 // Select first session

	m.setLogFilterFromContext()

	expected := "session.abc123456789.dev"
	if m.logFilter != expected {
		t.Errorf("logFilter = %q, want %q", m.logFilter, expected)
	}
}

func TestSetLogFilterFromContext_ContainerWithoutSessionInSessionsTab(t *testing.T) {
	m := newTestModel()
	m.selectedContainer = &container.Container{
		ID:       "abc123456789abcdef",
		Name:     "test-container",
		Sessions: []container.Session{},
	}
	m.currentTab = TabSessions
	m.selectedSessionIdx = 0

	m.setLogFilterFromContext()

	expected := "container.abc123456789"
	if m.logFilter != expected {
		t.Errorf("logFilter = %q, want %q", m.logFilter, expected)
	}
}

func TestSetLogFilterFromContext_ShortContainerID(t *testing.T) {
	m := newTestModel()
	m.selectedContainer = &container.Container{
		ID:   "abc12345",
		Name: "test-container",
	}
	m.currentTab = TabContainers

	// Should not panic and should use full ID
	m.setLogFilterFromContext()

	expected := "container.abc12345"
	if m.logFilter != expected {
		t.Errorf("logFilter = %q, want %q", m.logFilter, expected)
	}
}

func TestTruncateContainerID_LongID(t *testing.T) {
	id := "abc123456789abcdefghij"
	result := truncateContainerID(id)

	if len(result) > 12 {
		t.Errorf("truncateContainerID should return at most 12 chars, got %d: %q", len(result), result)
	}

	if result != "abc123456789" {
		t.Errorf("truncateContainerID(%q) = %q, want %q", id, result, "abc123456789")
	}
}

func TestTruncateContainerID_ShortID(t *testing.T) {
	id := "abc12345"
	result := truncateContainerID(id)

	if result != id {
		t.Errorf("truncateContainerID should return full ID when < 12 chars, got %q, want %q", result, id)
	}
}

func TestTruncateContainerID_ExactlyTwelve(t *testing.T) {
	id := "123456789012"
	result := truncateContainerID(id)

	if result != id {
		t.Errorf("truncateContainerID should return full ID when exactly 12 chars, got %q, want %q", result, id)
	}
}

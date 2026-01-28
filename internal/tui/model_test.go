package tui

import (
	"fmt"
	"testing"

	"devagent/internal/container"
	"devagent/internal/logging"
)

// Tab tests removed - tabs replaced by tree view

func TestTreeItemType_Methods(t *testing.T) {
	// Test TreeItemType helpers
	containerItem := TreeItem{Type: TreeItemContainer}
	sessionItem := TreeItem{Type: TreeItemSession}

	if !containerItem.IsContainer() {
		t.Error("TreeItemContainer.IsContainer() should be true")
	}
	if containerItem.IsSession() {
		t.Error("TreeItemContainer.IsSession() should be false")
	}
	if !sessionItem.IsSession() {
		t.Error("TreeItemSession.IsSession() should be true")
	}
	if sessionItem.IsContainer() {
		t.Error("TreeItemSession.IsContainer() should be false")
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
	m := newTestModel(t)

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
	m := newTestModel(t)

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
	m := newTestModel(t)

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
	m := newTestModel(t)

	m.addLogEntry(logging.LogEntry{Message: "app log", Scope: "app"})
	m.addLogEntry(logging.LogEntry{Message: "container log", Scope: "container.abc123"})
	m.addLogEntry(logging.LogEntry{Message: "session log", Scope: "session.abc123.dev"})

	// No filter = all entries
	m.logFilter = ""
	if len(m.filteredLogEntries()) != 3 {
		t.Errorf("no filter should return all entries, got %d", len(m.filteredLogEntries()))
	}

	// Container filter matches scope prefix
	m.logFilter = "container"
	filtered := m.filteredLogEntries()
	if len(filtered) != 1 {
		t.Errorf("container filter should return 1 entry, got %d", len(filtered))
	}
}

func TestSetLogFilterFromContext_NoContainerSelected(t *testing.T) {
	m := newTestModel(t)
	m.selectedContainer = nil

	m.setLogFilterFromContext()

	if m.logFilter != "" {
		t.Errorf("logFilter should be empty when no container selected, got %q", m.logFilter)
	}
}

func TestSetLogFilterFromContext_ContainerSelected(t *testing.T) {
	m := newTestModel(t)
	m.selectedContainer = &container.Container{
		ID:   "abc123456789abcdef",
		Name: "test-container",
	}

	m.setLogFilterFromContext()

	if m.logFilter != "container" {
		t.Errorf("logFilter = %q, want %q", m.logFilter, "container")
	}
	if m.logFilterLabel != "test-container" {
		t.Errorf("logFilterLabel = %q, want %q", m.logFilterLabel, "test-container")
	}
}

func TestSetLogFilterFromContext_SessionSelected(t *testing.T) {
	m := newTestModel(t)
	m.selectedContainer = &container.Container{
		ID:   "abc123456789abcdef",
		Name: "test-container",
		Sessions: []container.Session{
			{Name: "dev", Windows: 1},
			{Name: "build", Windows: 2},
		},
	}
	m.selectedSessionIdx = 0 // Select first session
	// Set up tree state so we know we're on a session item
	m.treeItems = []TreeItem{
		{Type: TreeItemContainer, ContainerID: "abc123456789abcdef"},
		{Type: TreeItemSession, ContainerID: "abc123456789abcdef", SessionName: "dev"},
	}
	m.selectedIdx = 1 // On session item

	m.setLogFilterFromContext()

	if m.logFilter != "tmux" {
		t.Errorf("logFilter = %q, want %q", m.logFilter, "tmux")
	}
	if m.logFilterLabel != "test-container > dev" {
		t.Errorf("logFilterLabel = %q, want %q", m.logFilterLabel, "test-container > dev")
	}
}

func TestSetLogFilterFromContext_ContainerWithoutSessions(t *testing.T) {
	m := newTestModel(t)
	m.selectedContainer = &container.Container{
		ID:       "abc123456789abcdef",
		Name:     "test-container",
		Sessions: []container.Session{},
	}
	m.selectedSessionIdx = 0

	m.setLogFilterFromContext()

	if m.logFilter != "container" {
		t.Errorf("logFilter = %q, want %q", m.logFilter, "container")
	}
	if m.logFilterLabel != "test-container" {
		t.Errorf("logFilterLabel = %q, want %q", m.logFilterLabel, "test-container")
	}
}

func TestSetLogFilterFromContext_ShortContainerID(t *testing.T) {
	m := newTestModel(t)
	m.selectedContainer = &container.Container{
		ID:   "abc12345",
		Name: "test-container",
	}

	m.setLogFilterFromContext()

	if m.logFilter != "container" {
		t.Errorf("logFilter = %q, want %q", m.logFilter, "container")
	}
	if m.logFilterLabel != "test-container" {
		t.Errorf("logFilterLabel = %q, want %q", m.logFilterLabel, "test-container")
	}
}

func TestPanelFocus_Constants(t *testing.T) {
	// Ensure FocusTree is the zero value
	var defaultFocus PanelFocus
	if defaultFocus != FocusTree {
		t.Errorf("zero value of PanelFocus should be FocusTree, got %d", defaultFocus)
	}
	if FocusTree != 0 {
		t.Errorf("FocusTree should be 0, got %d", FocusTree)
	}
	if FocusDetail != 1 {
		t.Errorf("FocusDetail should be 1, got %d", FocusDetail)
	}
	if FocusLogs != 2 {
		t.Errorf("FocusLogs should be 2, got %d", FocusLogs)
	}
}

func TestModel_NextFocus_OnlyTreeAvailable(t *testing.T) {
	m := newTestModel(t)
	// Default state: all panels closed
	m.panelFocus = FocusTree
	m.detailPanelOpen = false
	m.logPanelOpen = false
	m.logReady = false

	got := m.nextFocus()
	if got != FocusTree {
		t.Errorf("nextFocus() = %d, want FocusTree (%d)", got, FocusTree)
	}
}

func TestModel_NextFocus_DetailPanelOpen(t *testing.T) {
	m := newTestModel(t)
	m.panelFocus = FocusTree
	m.detailPanelOpen = true
	m.logPanelOpen = false

	got := m.nextFocus()
	if got != FocusDetail {
		t.Errorf("nextFocus() = %d, want FocusDetail (%d)", got, FocusDetail)
	}
}

func TestModel_NextFocus_LogPanelOpen(t *testing.T) {
	m := newTestModel(t)
	m.panelFocus = FocusTree
	m.detailPanelOpen = false
	m.logPanelOpen = true
	m.logReady = true

	got := m.nextFocus()
	if got != FocusLogs {
		t.Errorf("nextFocus() = %d, want FocusLogs (%d)", got, FocusLogs)
	}
}

func TestModel_NextFocus_AllPanelsOpen(t *testing.T) {
	m := newTestModel(t)
	m.panelFocus = FocusTree
	m.detailPanelOpen = true
	m.logPanelOpen = true
	m.logReady = true

	got := m.nextFocus()
	if got != FocusDetail {
		t.Errorf("nextFocus() from FocusTree = %d, want FocusDetail (%d)", got, FocusDetail)
	}

	m.panelFocus = FocusDetail
	got = m.nextFocus()
	if got != FocusLogs {
		t.Errorf("nextFocus() from FocusDetail = %d, want FocusLogs (%d)", got, FocusLogs)
	}

	m.panelFocus = FocusLogs
	got = m.nextFocus()
	if got != FocusTree {
		t.Errorf("nextFocus() from FocusLogs = %d, want FocusTree (%d)", got, FocusTree)
	}
}

func TestModel_NextFocus_DetailAndLogsOpen(t *testing.T) {
	m := newTestModel(t)
	m.detailPanelOpen = true
	m.logPanelOpen = true
	m.logReady = true

	m.panelFocus = FocusDetail
	got := m.nextFocus()
	if got != FocusLogs {
		t.Errorf("nextFocus() from FocusDetail = %d, want FocusLogs (%d)", got, FocusLogs)
	}
}

func TestModel_NextFocus_LogNotReady(t *testing.T) {
	m := newTestModel(t)
	m.panelFocus = FocusTree
	m.detailPanelOpen = false
	m.logPanelOpen = true
	m.logReady = false // not ready yet

	got := m.nextFocus()
	if got != FocusTree {
		t.Errorf("nextFocus() with logReady=false should stay on FocusTree, got %d", got)
	}
}

func TestModel_NextFocus_CirclesCycle(t *testing.T) {
	m := newTestModel(t)
	m.detailPanelOpen = true
	m.logPanelOpen = true
	m.logReady = true

	// Start at tree
	m.panelFocus = FocusTree
	sequence := []PanelFocus{FocusDetail, FocusLogs, FocusTree, FocusDetail}
	for _, expected := range sequence {
		got := m.nextFocus()
		m.panelFocus = got
		if got != expected {
			t.Errorf("nextFocus() = %d, want %d", got, expected)
		}
	}
}

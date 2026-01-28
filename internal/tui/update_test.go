package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/container"
)

// Tab switching tests removed - tabs replaced by tree navigation
// See tree_test.go for TestTreeNavigation_* tests

func TestLogPanelToggle_LKey(t *testing.T) {
	// Both "l" and "L" toggle the log panel
	tests := []struct {
		name          string
		key           string
		startOpen     bool
		wantOpen      bool
	}{
		{"press l opens log panel", "l", false, true},
		{"press l closes log panel", "l", true, false},
		{"press L opens log panel", "L", false, true},
		{"press L closes log panel", "L", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.logPanelOpen = tt.startOpen

			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			updated, _ := m.Update(msg)
			result := updated.(Model)

			if result.logPanelOpen != tt.wantOpen {
				t.Errorf("logPanelOpen = %v, want %v", result.logPanelOpen, tt.wantOpen)
			}
		})
	}
}

// Enter on container and session tab navigation tests removed -
// See tree_test.go for TestTreeNavigation_* tests

// Session tab backspace, enter, and escape tests removed -
// See tree_test.go for TestTreeNavigation_* tests

func TestContainerAction_ShowsLoading(t *testing.T) {
	m := newTestModel(t)

	// Add a container
	ctr := &container.Container{
		ID:    "abc123def456",
		Name:  "test-container",
		State: container.StateStopped,
	}
	m.containerList.SetItems(toListItems([]*container.Container{ctr}))

	// Press 's' to start
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}
	updated, cmd := m.Update(msg)
	m = updated.(Model)

	if m.statusLevel != StatusLoading {
		t.Errorf("statusLevel = %v, want %v", m.statusLevel, StatusLoading)
	}
	if !strings.Contains(m.statusMessage, "Starting") {
		t.Errorf("statusMessage = %q, should contain 'Starting'", m.statusMessage)
	}
	if cmd == nil {
		t.Error("should return command for spinner")
	}
}

func TestContainerActionMsg_Success(t *testing.T) {
	m := newTestModel(t)
	m.statusLevel = StatusLoading
	m.statusMessage = "Starting..."

	// Simulate success message
	msg := containerActionMsg{action: "start", id: "abc123", err: nil}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.statusLevel != StatusSuccess {
		t.Errorf("statusLevel = %v, want %v", m.statusLevel, StatusSuccess)
	}
}

func TestContainerActionMsg_Error(t *testing.T) {
	m := newTestModel(t)
	m.statusLevel = StatusLoading

	// Simulate error message
	msg := containerActionMsg{action: "start", id: "abc123", err: fmt.Errorf("connection refused")}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.statusLevel != StatusError {
		t.Errorf("statusLevel = %v, want %v", m.statusLevel, StatusError)
	}
	if m.err == nil {
		t.Error("err should be set")
	}
}

func TestEscape_ClearsError(t *testing.T) {
	m := newTestModel(t)
	m.statusLevel = StatusError
	m.statusMessage = "Something failed"
	m.err = fmt.Errorf("test error")

	// Press Escape
	msg := tea.KeyMsg{Type: tea.KeyEscape}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.statusLevel == StatusError {
		t.Error("statusLevel should not be Error after Escape")
	}
	if m.err != nil {
		t.Error("err should be nil after Escape")
	}
}

func TestContainerAction_SetsPending(t *testing.T) {
	m := newTestModel(t)

	// Add a container
	ctr := &container.Container{
		ID:    "abc123def456",
		Name:  "test-container",
		State: container.StateStopped,
	}
	m.containerList.SetItems(toListItems([]*container.Container{ctr}))

	// Press 's' to start
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if !m.isPending("abc123def456") {
		t.Error("container should be pending after start action")
	}
	if m.getPendingOperation("abc123def456") != "start" {
		t.Errorf("pending operation = %q, want 'start'", m.getPendingOperation("abc123def456"))
	}
}

func TestContainerActionMsg_ClearsPending(t *testing.T) {
	m := newTestModel(t)
	m.setPending("abc123", "start")

	// Simulate success message
	msg := containerActionMsg{action: "start", id: "abc123", err: nil}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.isPending("abc123") {
		t.Error("container should not be pending after action completes")
	}
}

func TestContainerActionMsg_ClearsPendingOnError(t *testing.T) {
	m := newTestModel(t)
	m.setPending("abc123", "start")

	// Simulate error message
	msg := containerActionMsg{action: "start", id: "abc123", err: fmt.Errorf("failed")}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.isPending("abc123") {
		t.Error("container should not be pending after error")
	}
}

// Task 7: Add L key toggle for log panel
func TestLKey_TogglesLogPanel(t *testing.T) {
	m := newTestModel(t)
	m.logPanelOpen = false

	// Press L to open
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if !m.logPanelOpen {
		t.Error("logPanelOpen should be true after L")
	}

	// Press L again to close
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if m.logPanelOpen {
		t.Error("logPanelOpen should be false after second L")
	}
}

func TestLKey_SetsFilterFromContext(t *testing.T) {
	m := newTestModel(t)
	m.selectedContainer = &container.Container{ID: "abc123456789", Name: "test"}

	// Press L
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.logFilter == "" {
		t.Error("logFilter should be set from context")
	}
	if !strings.Contains(m.logFilter, "abc123456789") {
		t.Errorf("logFilter = %q, should contain container ID", m.logFilter)
	}
}

// Task 8: Add log viewport navigation
func TestJKey_ScrollsLogPanel(t *testing.T) {
	m := newTestModel(t)
	m.logPanelOpen = true
	m.logReady = true

	// Initialize viewport
	m.logViewport.SetContent("line1\nline2\nline3\nline4\nline5")
	m.logViewport.Height = 2

	// Press j (down)
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	// Viewport should have scrolled down
	if m.logViewport.YOffset == 0 {
		t.Error("viewport should scroll down on j key")
	}
}

func TestKKey_ScrollsLogPanelUp(t *testing.T) {
	m := newTestModel(t)
	m.logPanelOpen = true
	m.logReady = true

	// Initialize viewport with some content scrolled
	m.logViewport.SetContent("line1\nline2\nline3\nline4\nline5")
	m.logViewport.Height = 2
	m.logViewport.SetYOffset(2)
	startOffset := m.logViewport.YOffset

	// Press k (up)
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	// Viewport should have scrolled up
	if m.logViewport.YOffset >= startOffset {
		t.Error("viewport should scroll up on k key")
	}
}

func TestGKey_GoesToTopOfLogs(t *testing.T) {
	m := newTestModel(t)
	m.logPanelOpen = true
	m.logReady = true

	// Initialize viewport with content scrolled
	m.logViewport.SetContent("line1\nline2\nline3\nline4\nline5")
	m.logViewport.Height = 2
	m.logViewport.SetYOffset(2)

	// Press g
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.logViewport.YOffset != 0 {
		t.Error("viewport should go to top on g key")
	}
	if m.logAutoScroll {
		t.Error("logAutoScroll should be false after g key")
	}
}

func TestCapitalGKey_GoesToBottomOfLogs(t *testing.T) {
	m := newTestModel(t)
	m.logPanelOpen = true
	m.logReady = true

	// Initialize viewport with content
	m.logViewport.SetContent("line1\nline2\nline3\nline4\nline5")
	m.logViewport.Height = 2

	// Press G
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if !m.logViewport.AtBottom() {
		t.Error("viewport should be at bottom on G key")
	}
	if !m.logAutoScroll {
		t.Error("logAutoScroll should be true after G key")
	}
}

// Task 9: Error state auto-opens filtered logs
func TestSetError_SetsLogFilter(t *testing.T) {
	m := newTestModel(t)
	m.selectedContainer = &container.Container{ID: "abc123456789", Name: "test"}

	m.setError("test failed", fmt.Errorf("test error"))

	if m.logFilter == "" {
		t.Error("logFilter should be set when error occurs")
	}
	if !strings.Contains(m.logFilter, "abc123456789") {
		t.Errorf("logFilter = %q, should contain container ID", m.logFilter)
	}
}

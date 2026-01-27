package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/container"
)

func TestTabSwitching_NumberKeys(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		startTab TabMode
		wantTab  TabMode
	}{
		{"press 1 switches to Containers", "1", TabSessions, TabContainers},
		{"press 2 switches to Sessions", "2", TabContainers, TabSessions},
		{"press 1 stays on Containers", "1", TabContainers, TabContainers},
		{"press 2 stays on Sessions", "2", TabSessions, TabSessions},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.currentTab = tt.startTab

			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			updated, _ := m.Update(msg)
			result := updated.(Model)

			if result.currentTab != tt.wantTab {
				t.Errorf("currentTab = %v, want %v", result.currentTab, tt.wantTab)
			}
		})
	}
}

func TestTabSwitching_HLKeys(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		startTab TabMode
		wantTab  TabMode
	}{
		{"press h switches left to Containers", "h", TabSessions, TabContainers},
		{"press l switches right to Sessions", "l", TabContainers, TabSessions},
		{"press h stays on Containers (left boundary)", "h", TabContainers, TabContainers},
		{"press l stays on Sessions (right boundary)", "l", TabSessions, TabSessions},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.currentTab = tt.startTab

			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			updated, _ := m.Update(msg)
			result := updated.(Model)

			if result.currentTab != tt.wantTab {
				t.Errorf("currentTab = %v, want %v", result.currentTab, tt.wantTab)
			}
		})
	}
}

func TestEnterOnContainer_SwitchesToSessionsTab(t *testing.T) {
	m := newTestModel(t)

	// Add a container to the list
	ctr := &container.Container{
		ID:    "abc123def456",
		Name:  "test-container",
		State: container.StateRunning,
	}
	m.containerList.SetItems(toListItems([]*container.Container{ctr}))
	m.currentTab = TabContainers

	// Press Enter
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	// Should switch to Sessions tab
	if result.currentTab != TabSessions {
		t.Errorf("currentTab = %v, want %v", result.currentTab, TabSessions)
	}

	// Should have selected the container
	if result.selectedContainer == nil {
		t.Fatal("selectedContainer should not be nil")
	}
	if result.selectedContainer.ID != ctr.ID {
		t.Errorf("selectedContainer.ID = %q, want %q", result.selectedContainer.ID, ctr.ID)
	}
}

func TestEnterOnContainer_RefreshesSessions(t *testing.T) {
	m := newTestModel(t)

	// Add a container
	ctr := &container.Container{
		ID:    "abc123def456",
		Name:  "test-container",
		State: container.StateRunning,
	}
	m.containerList.SetItems(toListItems([]*container.Container{ctr}))
	m.currentTab = TabContainers

	// Press Enter
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := m.Update(msg)

	// Should return a command (to refresh sessions)
	if cmd == nil {
		t.Error("Update should return a command to refresh sessions")
	}
}

func TestSessionsTab_UpDownNavigation(t *testing.T) {
	m := newTestModel(t)
	m.currentTab = TabSessions
	m.selectedContainer = &container.Container{
		ID:   "abc123",
		Name: "test-container",
		Sessions: []container.Session{
			{Name: "dev", ContainerID: "abc123"},
			{Name: "test", ContainerID: "abc123"},
			{Name: "prod", ContainerID: "abc123"},
		},
	}
	m.selectedSessionIdx = 0

	// Press down twice
	msg := tea.KeyMsg{Type: tea.KeyDown}
	updated, _ := m.Update(msg)
	m = updated.(Model)
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if m.selectedSessionIdx != 2 {
		t.Errorf("selectedSessionIdx = %d, want 2", m.selectedSessionIdx)
	}

	// Press up once
	msg = tea.KeyMsg{Type: tea.KeyUp}
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if m.selectedSessionIdx != 1 {
		t.Errorf("selectedSessionIdx = %d, want 1", m.selectedSessionIdx)
	}
}

func TestSessionsTab_JKNavigation(t *testing.T) {
	m := newTestModel(t)
	m.currentTab = TabSessions
	m.selectedContainer = &container.Container{
		ID:   "abc123",
		Name: "test-container",
		Sessions: []container.Session{
			{Name: "dev", ContainerID: "abc123"},
			{Name: "test", ContainerID: "abc123"},
		},
	}
	m.selectedSessionIdx = 0

	// Press j (down)
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.selectedSessionIdx != 1 {
		t.Errorf("selectedSessionIdx = %d, want 1 after 'j'", m.selectedSessionIdx)
	}

	// Press k (up)
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if m.selectedSessionIdx != 0 {
		t.Errorf("selectedSessionIdx = %d, want 0 after 'k'", m.selectedSessionIdx)
	}
}

func TestSessionsTab_Backspace_ReturnsToContainers(t *testing.T) {
	m := newTestModel(t)
	m.currentTab = TabSessions
	m.selectedContainer = &container.Container{
		ID:   "abc123",
		Name: "test-container",
	}

	// Press backspace
	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.currentTab != TabContainers {
		t.Errorf("currentTab = %v, want %v after backspace", m.currentTab, TabContainers)
	}
	if m.selectedContainer != nil {
		t.Error("selectedContainer should be nil after backspace")
	}
}

func TestSessionsTab_Enter_OpensSessionDetail(t *testing.T) {
	m := newTestModel(t)
	m.currentTab = TabSessions
	m.selectedContainer = &container.Container{
		ID:   "abc123",
		Name: "test-container",
		Sessions: []container.Session{
			{Name: "dev", ContainerID: "abc123"},
		},
	}
	m.selectedSessionIdx = 0

	// Press Enter
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if !m.sessionDetailOpen {
		t.Error("sessionDetailOpen should be true after Enter on session")
	}
}

func TestSessionDetail_Escape_Returns(t *testing.T) {
	m := newTestModel(t)
	m.currentTab = TabSessions
	m.selectedContainer = &container.Container{
		ID:       "abc123",
		Name:     "test-container",
		Sessions: []container.Session{{Name: "dev", ContainerID: "abc123"}},
	}
	m.sessionDetailOpen = true

	// Press Escape
	msg := tea.KeyMsg{Type: tea.KeyEscape}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.sessionDetailOpen {
		t.Error("sessionDetailOpen should be false after Escape")
	}
}

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
	m.currentTab = TabSessions

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
	m.logViewport.LineDown(2)
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
	m.logViewport.LineDown(2)

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
	m.currentTab = TabContainers

	m.setError("test failed", fmt.Errorf("test error"))

	if m.logFilter == "" {
		t.Error("logFilter should be set when error occurs")
	}
	if !strings.Contains(m.logFilter, "abc123456789") {
		t.Errorf("logFilter = %q, should contain container ID", m.logFilter)
	}
}

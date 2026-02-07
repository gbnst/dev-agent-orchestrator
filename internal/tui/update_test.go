package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/container"
	"devagent/internal/logging"
)

// Tab switching tests removed - tabs replaced by tree navigation
// See tree_test.go for TestTreeNavigation_* tests

func TestLogPanelToggle_LKey(t *testing.T) {
	// Both "l" and "L" toggle the log panel
	tests := []struct {
		name      string
		key       string
		startOpen bool
		wantOpen  bool
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

	// Add a container and select it
	ctr := &container.Container{
		ID:    "abc123def456",
		Name:  "test-container",
		State: container.StateStopped,
	}
	m.containerList.SetItems(toListItems([]*container.Container{ctr}))
	m.rebuildTreeItems()
	m.selectedIdx = 1 // Container (after All)
	m.syncSelectionFromTree()

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

	// Add a container and select it
	ctr := &container.Container{
		ID:    "abc123def456",
		Name:  "test-container",
		State: container.StateStopped,
	}
	m.containerList.SetItems(toListItems([]*container.Container{ctr}))
	m.rebuildTreeItems()
	m.selectedIdx = 1 // Container (after All)
	m.syncSelectionFromTree()

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

func TestLogFilter_SyncsOnTreeNavigation(t *testing.T) {
	m := newTestModel(t)

	// Set up a container in the list and tree
	c := &container.Container{ID: "abc123456789", Name: "test", State: container.StateRunning}
	m.containerList.SetItems([]list.Item{containerItem{container: c}})
	m.treeItems = []TreeItem{
		{Type: TreeItemAll},
		{Type: TreeItemContainer, ContainerID: c.ID},
	}
	m.selectedIdx = 1 // Container (after All)

	// syncSelectionFromTree now syncs the log filter
	m.syncSelectionFromTree()

	if m.logFilter != "test" {
		t.Errorf("logFilter = %q, want %q", m.logFilter, "test")
	}
	if m.logFilterLabel != "test" {
		t.Errorf("logFilterLabel = %q, want %q", m.logFilterLabel, "test")
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
	m.setLogFilterFromContext() // Must be explicit now

	if m.logFilter != "test" {
		t.Errorf("logFilter = %q, want %q", m.logFilter, "test")
	}
	if m.logFilterLabel != "test" {
		t.Errorf("logFilterLabel = %q, want %q", m.logFilterLabel, "test")
	}
}

// Panel focus tests

func TestPanelFocus_DefaultsToTree(t *testing.T) {
	m := newTestModel(t)
	if m.panelFocus != FocusTree {
		t.Errorf("default panelFocus = %d, want FocusTree (%d)", m.panelFocus, FocusTree)
	}
}

func TestTabKey_CyclesToDetail(t *testing.T) {
	m := newTestModel(t)
	m.detailPanelOpen = true
	m.logPanelOpen = false

	msg := tea.KeyMsg{Type: tea.KeyTab}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.panelFocus != FocusDetail {
		t.Errorf("panelFocus = %d, want FocusDetail (%d)", result.panelFocus, FocusDetail)
	}
}

func TestTabKey_CyclesToLogs(t *testing.T) {
	m := newTestModel(t)
	m.panelFocus = FocusDetail
	m.detailPanelOpen = true
	m.logPanelOpen = true
	m.logReady = true

	msg := tea.KeyMsg{Type: tea.KeyTab}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.panelFocus != FocusLogs {
		t.Errorf("panelFocus = %d, want FocusLogs (%d)", result.panelFocus, FocusLogs)
	}
}

func TestTabKey_CyclesBackToTree(t *testing.T) {
	m := newTestModel(t)
	m.panelFocus = FocusLogs
	m.logPanelOpen = true
	m.logReady = true

	msg := tea.KeyMsg{Type: tea.KeyTab}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.panelFocus != FocusTree {
		t.Errorf("panelFocus = %d, want FocusTree (%d)", result.panelFocus, FocusTree)
	}
}

func TestTabKey_SkipsClosedPanels(t *testing.T) {
	m := newTestModel(t)
	m.panelFocus = FocusTree
	m.detailPanelOpen = false
	m.logPanelOpen = true
	m.logReady = true

	msg := tea.KeyMsg{Type: tea.KeyTab}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.panelFocus != FocusLogs {
		t.Errorf("panelFocus = %d, want FocusLogs (%d) (should skip closed detail)", result.panelFocus, FocusLogs)
	}
}

func TestTabKey_NoOpWhenNoPanelsOpen(t *testing.T) {
	m := newTestModel(t)
	m.panelFocus = FocusTree
	m.detailPanelOpen = false
	m.logPanelOpen = false

	msg := tea.KeyMsg{Type: tea.KeyTab}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.panelFocus != FocusTree {
		t.Errorf("panelFocus = %d, want FocusTree (%d) (no panels to switch to)", result.panelFocus, FocusTree)
	}
}

func TestEscapeKey_DetailToTree(t *testing.T) {
	m := newTestModel(t)
	m.panelFocus = FocusDetail
	m.detailPanelOpen = true

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.panelFocus != FocusTree {
		t.Errorf("panelFocus = %d, want FocusTree (%d)", result.panelFocus, FocusTree)
	}
}

func TestEscapeKey_LogsToTree(t *testing.T) {
	m := newTestModel(t)
	m.panelFocus = FocusLogs
	m.logPanelOpen = true
	m.logReady = true

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.panelFocus != FocusTree {
		t.Errorf("panelFocus = %d, want FocusTree (%d)", result.panelFocus, FocusTree)
	}
}

func TestUpDown_SelectsLogsWhenDetailsNotOpen(t *testing.T) {
	m := newTestModel(t)
	m.panelFocus = FocusLogs
	m.logPanelOpen = true
	m.logReady = true
	m.logDetailsOpen = false
	m.selectedLogIndex = 0

	// Add 3 log entries
	for i := 0; i < 3; i++ {
		m.logEntries = append(m.logEntries, logging.LogEntry{
			Timestamp: time.Now(),
			Level:     "INFO",
			Scope:     "container.test",
			Message:   fmt.Sprintf("entry%d", i),
			Fields:    make(map[string]any),
		})
	}

	// Press down to select next log
	msg := tea.KeyMsg{Type: tea.KeyDown}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.selectedLogIndex != 1 {
		t.Errorf("selectedLogIndex = %d, want 1 (should navigate logs when details closed)", result.selectedLogIndex)
	}
}

func TestUpDown_NavigatesTreeWhenTreeFocused(t *testing.T) {
	m := newTestModel(t)
	m.panelFocus = FocusTree

	// Set up tree items
	containers := []*container.Container{
		{ID: "aaa111222333", Name: "container-1", State: container.StateRunning},
		{ID: "bbb444555666", Name: "container-2", State: container.StateStopped},
	}
	m.containerList.SetItems(toListItems(containers))
	m.rebuildTreeItems()
	m.selectedIdx = 0

	// Press down
	msg := tea.KeyMsg{Type: tea.KeyDown}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.selectedIdx != 1 {
		t.Errorf("selectedIdx = %d, want 1 (should navigate tree when tree focused)", result.selectedIdx)
	}
}

func TestClosingLogPanel_ReturnsFocusToTree(t *testing.T) {
	m := newTestModel(t)
	m.panelFocus = FocusLogs
	m.logPanelOpen = true
	m.logReady = true

	// Press l to close log panel
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.panelFocus != FocusTree {
		t.Errorf("panelFocus = %d, want FocusTree (%d) after closing log panel", result.panelFocus, FocusTree)
	}
	if result.logPanelOpen {
		t.Error("logPanelOpen should be false after pressing l")
	}
}

func TestClosingDetailPanel_ReturnsFocusToTree(t *testing.T) {
	m := newTestModel(t)
	m.panelFocus = FocusTree
	m.detailPanelOpen = true

	// Set up tree items so left arrow handler is active
	containers := []*container.Container{
		{ID: "aaa111222333", Name: "container-1", State: container.StateRunning},
	}
	m.containerList.SetItems(toListItems(containers))
	m.rebuildTreeItems()
	m.selectedIdx = 1 // Container (after All)

	// Press left to close detail panel
	msg := tea.KeyMsg{Type: tea.KeyLeft}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.detailPanelOpen {
		t.Error("detailPanelOpen should be false after pressing left")
	}
	if result.panelFocus != FocusTree {
		t.Errorf("panelFocus = %d, want FocusTree (%d) after closing detail panel", result.panelFocus, FocusTree)
	}
}

func TestNextFocus_SkipsClosedPanels(t *testing.T) {
	// This is tested more thoroughly in model_test.go but verify integration
	m := newTestModel(t)
	m.panelFocus = FocusTree
	m.detailPanelOpen = false
	m.logPanelOpen = false
	m.logReady = false

	got := m.nextFocus()
	if got != FocusTree {
		t.Errorf("nextFocus() = %d, want FocusTree when no panels open", got)
	}

	// With only logs open
	m.logPanelOpen = true
	m.logReady = true
	got = m.nextFocus()
	if got != FocusLogs {
		t.Errorf("nextFocus() = %d, want FocusLogs when only logs open", got)
	}
}

// Quit behavior tests

func TestCtrlD_Quits(t *testing.T) {
	m := newTestModel(t)

	msg := tea.KeyMsg{Type: tea.KeyCtrlD}
	_, cmd := m.Update(msg)

	if cmd == nil {
		t.Fatal("ctrl+d should return a command")
	}
	// tea.Quit returns a special quit message
	quitMsg := cmd()
	if _, ok := quitMsg.(tea.QuitMsg); !ok {
		t.Errorf("ctrl+d should return tea.Quit, got %T", quitMsg)
	}
}

func TestSingleCtrlC_DoesNotQuit(t *testing.T) {
	m := newTestModel(t)

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(msg)

	if cmd != nil {
		t.Error("single ctrl+c should not return a command")
	}
}

func TestDoubleCtrlC_Quits(t *testing.T) {
	m := newTestModel(t)

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}

	// First press
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.lastCtrlCTime.IsZero() {
		t.Fatal("lastCtrlCTime should be set after first ctrl+c")
	}

	// Second press (immediately after)
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("double ctrl+c should return a command")
	}
	quitMsg := cmd()
	if _, ok := quitMsg.(tea.QuitMsg); !ok {
		t.Errorf("double ctrl+c should return tea.Quit, got %T", quitMsg)
	}
}

func TestCtrlC_ExpiredWindow_DoesNotQuit(t *testing.T) {
	m := newTestModel(t)

	// Simulate a ctrl+c that happened long ago
	m.lastCtrlCTime = time.Now().Add(-1 * time.Second)

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(msg)

	if cmd != nil {
		t.Error("ctrl+c after expired window should not quit")
	}
}

func TestQKey_DoesNotQuit(t *testing.T) {
	m := newTestModel(t)

	// Set up tree items so we have context for the key handler
	containers := []*container.Container{
		{ID: "aaa111222333", Name: "container-1", State: container.StateRunning},
	}
	m.containerList.SetItems(toListItems(containers))
	m.rebuildTreeItems()
	m.selectedIdx = 0

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
	_, cmd := m.Update(msg)

	// q should not produce a quit command
	if cmd != nil {
		result := cmd()
		if _, ok := result.(tea.QuitMsg); ok {
			t.Error("q key should not quit the application")
		}
	}
}

// Escape behavior tests

func TestEscape_ClosesDetailPanel_WhenTreeFocused(t *testing.T) {
	m := newTestModel(t)
	m.panelFocus = FocusTree
	m.detailPanelOpen = true

	containers := []*container.Container{
		{ID: "aaa111222333", Name: "container-1", State: container.StateRunning},
	}
	m.containerList.SetItems(toListItems(containers))
	m.rebuildTreeItems()
	m.selectedIdx = 1

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.detailPanelOpen {
		t.Error("escape should close detail panel when tree focused")
	}
	if result.quitHintCount != 0 {
		t.Errorf("quitHintCount should be 0 after closing panel, got %d", result.quitHintCount)
	}
}

func TestEscape_ShowsQuitHint_AfterTwoPresses(t *testing.T) {
	m := newTestModel(t)
	m.panelFocus = FocusTree
	m.detailPanelOpen = false

	containers := []*container.Container{
		{ID: "aaa111222333", Name: "container-1", State: container.StateRunning},
	}
	m.containerList.SetItems(toListItems(containers))
	m.rebuildTreeItems()
	m.selectedIdx = 1

	msg := tea.KeyMsg{Type: tea.KeyEscape}

	// First press — nothing to close
	updated, _ := m.Update(msg)
	m = updated.(Model)
	if m.quitHintCount != 1 {
		t.Errorf("quitHintCount = %d after first esc, want 1", m.quitHintCount)
	}

	// Second press — should show hint and return a delayed clear command
	updated, cmd := m.Update(msg)
	m = updated.(Model)
	if m.statusMessage != "ctrl+c ctrl+c to quit" {
		t.Errorf("statusMessage = %q, want %q", m.statusMessage, "ctrl+c ctrl+c to quit")
	}
	if m.quitHintCount != 0 {
		t.Errorf("quitHintCount should reset to 0 after showing hint, got %d", m.quitHintCount)
	}
	if cmd == nil {
		t.Error("should return a delayed clear command")
	}
}

func TestClearStatusMsg_ClearsQuitHint(t *testing.T) {
	m := newTestModel(t)
	m.statusLevel = StatusInfo
	m.statusMessage = "ctrl+c ctrl+c to quit"

	updated, _ := m.Update(clearStatusMsg{})
	result := updated.(Model)

	if result.statusMessage != "" {
		t.Errorf("statusMessage = %q, want empty after clearStatusMsg", result.statusMessage)
	}
}

func TestClearStatusMsg_DoesNotClobberOtherStatus(t *testing.T) {
	m := newTestModel(t)
	m.statusLevel = StatusSuccess
	m.statusMessage = "Container started"

	updated, _ := m.Update(clearStatusMsg{})
	result := updated.(Model)

	if result.statusMessage != "Container started" {
		t.Errorf("statusMessage = %q, should not be cleared when not quit hint", result.statusMessage)
	}
	if result.statusLevel != StatusSuccess {
		t.Errorf("statusLevel = %v, should remain StatusSuccess", result.statusLevel)
	}
}

func TestEscape_ResetsCountOnOtherKey(t *testing.T) {
	m := newTestModel(t)
	m.panelFocus = FocusTree
	m.detailPanelOpen = false

	containers := []*container.Container{
		{ID: "aaa111222333", Name: "container-1", State: container.StateRunning},
	}
	m.containerList.SetItems(toListItems(containers))
	m.rebuildTreeItems()
	m.selectedIdx = 1

	// Press escape once
	msg := tea.KeyMsg{Type: tea.KeyEscape}
	updated, _ := m.Update(msg)
	m = updated.(Model)
	if m.quitHintCount != 1 {
		t.Fatalf("quitHintCount = %d after first esc, want 1", m.quitHintCount)
	}

	// Press a different key (r for refresh)
	otherMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}
	updated, _ = m.Update(otherMsg)
	m = updated.(Model)
	if m.quitHintCount != 0 {
		t.Errorf("quitHintCount should reset on non-escape key, got %d", m.quitHintCount)
	}
}

func TestEscape_ReturnsLogsFocusToTree(t *testing.T) {
	m := newTestModel(t)
	m.panelFocus = FocusLogs
	m.logPanelOpen = true
	m.logReady = true

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.panelFocus != FocusTree {
		t.Errorf("panelFocus = %d, want FocusTree after esc from logs", result.panelFocus)
	}
	if result.quitHintCount != 0 {
		t.Errorf("quitHintCount should be 0 after closing panel focus, got %d", result.quitHintCount)
	}
}

func TestEscape_ReturnsDetailFocusToTree(t *testing.T) {
	m := newTestModel(t)
	m.panelFocus = FocusDetail
	m.detailPanelOpen = true

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.panelFocus != FocusTree {
		t.Errorf("panelFocus = %d, want FocusTree after esc from detail", result.panelFocus)
	}
	if result.quitHintCount != 0 {
		t.Errorf("quitHintCount should be 0 after returning focus, got %d", result.quitHintCount)
	}
}

func TestQKey_ShowsQuitHint_AfterTwoPresses(t *testing.T) {
	m := newTestModel(t)
	m.panelFocus = FocusTree

	containers := []*container.Container{
		{ID: "aaa111222333", Name: "container-1", State: container.StateRunning},
	}
	m.containerList.SetItems(toListItems(containers))
	m.rebuildTreeItems()
	m.selectedIdx = 1

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}

	// First press
	updated, _ := m.Update(msg)
	m = updated.(Model)
	if m.quitHintCount != 1 {
		t.Errorf("quitHintCount = %d after first q, want 1", m.quitHintCount)
	}

	// Second press — should show hint
	updated, cmd := m.Update(msg)
	m = updated.(Model)
	if m.statusMessage != "ctrl+c ctrl+c to quit" {
		t.Errorf("statusMessage = %q, want %q", m.statusMessage, "ctrl+c ctrl+c to quit")
	}
	if cmd == nil {
		t.Error("should return a delayed clear command")
	}
}

func TestEscThenQ_ShowsQuitHint(t *testing.T) {
	m := newTestModel(t)
	m.panelFocus = FocusTree
	m.detailPanelOpen = false

	containers := []*container.Container{
		{ID: "aaa111222333", Name: "container-1", State: container.StateRunning},
	}
	m.containerList.SetItems(toListItems(containers))
	m.rebuildTreeItems()
	m.selectedIdx = 1

	// First press — esc with nothing to close
	escMsg := tea.KeyMsg{Type: tea.KeyEscape}
	updated, _ := m.Update(escMsg)
	m = updated.(Model)
	if m.quitHintCount != 1 {
		t.Fatalf("quitHintCount = %d after esc, want 1", m.quitHintCount)
	}

	// Second press — q
	qMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
	updated, cmd := m.Update(qMsg)
	m = updated.(Model)
	if m.statusMessage != "ctrl+c ctrl+c to quit" {
		t.Errorf("statusMessage = %q, want %q", m.statusMessage, "ctrl+c ctrl+c to quit")
	}
	if cmd == nil {
		t.Error("should return a delayed clear command")
	}
}

// Action menu tests

func TestTKey_OpensActionMenu_WhenRunningContainer(t *testing.T) {
	m := newTestModel(t)

	containers := []*container.Container{
		{ID: "aaa111222333", Name: "running-container", State: container.StateRunning},
	}
	m.containerList.SetItems(toListItems(containers))
	m.rebuildTreeItems()
	m.selectedIdx = 1 // Container (after All)
	m.syncSelectionFromTree()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if !result.actionMenuOpen {
		t.Error("action menu should open when 't' pressed on running container")
	}
}

func TestTKey_NoOp_WhenStoppedContainer(t *testing.T) {
	m := newTestModel(t)

	containers := []*container.Container{
		{ID: "aaa111222333", Name: "stopped-container", State: container.StateStopped},
	}
	m.containerList.SetItems(toListItems(containers))
	m.rebuildTreeItems()
	m.selectedIdx = 1 // Container (after All)
	m.syncSelectionFromTree()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.actionMenuOpen {
		t.Error("action menu should not open when 't' pressed on stopped container")
	}
}

func TestTKey_NoOp_WhenNoContainerSelected(t *testing.T) {
	m := newTestModel(t)

	containers := []*container.Container{
		{ID: "aaa111222333", Name: "container-1", State: container.StateRunning},
	}
	m.containerList.SetItems(toListItems(containers))
	m.rebuildTreeItems()
	m.selectedIdx = 0 // "All Containers"
	m.syncSelectionFromTree()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.actionMenuOpen {
		t.Error("action menu should not open when no container is selected")
	}
}

func TestEscapeKey_ClosesActionMenu(t *testing.T) {
	m := newTestModel(t)
	m.actionMenuOpen = true

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.actionMenuOpen {
		t.Error("action menu should close when escape is pressed")
	}
}

func TestActionMenu_BlocksOtherKeys(t *testing.T) {
	m := newTestModel(t)
	m.actionMenuOpen = true
	m.logPanelOpen = false

	// Press 'l' which would normally toggle log panel
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.logPanelOpen {
		t.Error("action menu should block other key handlers")
	}
	if !result.actionMenuOpen {
		t.Error("action menu should remain open after unhandled key")
	}
}

func TestIsActionMenuOpen(t *testing.T) {
	m := newTestModel(t)

	if m.IsActionMenuOpen() {
		t.Error("action menu should be closed by default")
	}

	m.actionMenuOpen = true
	if !m.IsActionMenuOpen() {
		t.Error("IsActionMenuOpen should return true when menu is open")
	}
}

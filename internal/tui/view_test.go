package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"devagent/internal/container"
	"devagent/internal/logging"
)

// Sessions tab content tests removed - sessions now shown in tree view
// See tree_test.go for tree rendering tests

// Tab and session tab tests removed - tabs replaced by tree view
// See tree_test.go for TestRenderDetailPanel_Session tests

func TestRenderStatusBar_Info(t *testing.T) {
	m := newTestModel(t)
	m.statusLevel = StatusInfo
	m.statusMessage = "Ready"

	result := m.renderStatusBar(80)

	if !strings.Contains(result, "Ready") {
		t.Error("status bar should contain message")
	}
}

func TestRenderStatusBar_Success(t *testing.T) {
	m := newTestModel(t)
	m.statusLevel = StatusSuccess
	m.statusMessage = "Container started"

	result := m.renderStatusBar(80)

	if !strings.Contains(result, "✓") {
		t.Error("success status should contain checkmark")
	}
	if !strings.Contains(result, "Container started") {
		t.Error("status bar should contain message")
	}
}

func TestRenderStatusBar_Error(t *testing.T) {
	m := newTestModel(t)
	m.statusLevel = StatusError
	m.statusMessage = "Failed to start"
	m.err = fmt.Errorf("connection refused")

	result := m.renderStatusBar(80)

	if !strings.Contains(result, "✗") {
		t.Error("error status should contain X mark")
	}
	if !strings.Contains(result, "Failed to start") {
		t.Error("status bar should contain message")
	}
	if !strings.Contains(result, "esc to clear") {
		t.Error("error status should contain '(esc to clear)' hint")
	}
}

func TestRenderStatusBar_Loading(t *testing.T) {
	m := newTestModel(t)
	m.statusLevel = StatusLoading
	m.statusMessage = "Starting container..."

	result := m.renderStatusBar(80)

	// Spinner renders differently each frame, just check message
	if !strings.Contains(result, "Starting container...") {
		t.Error("status bar should contain loading message")
	}
}

func TestRenderLogEntry(t *testing.T) {
	m := newTestModel(t)

	entry := logging.LogEntry{
		Timestamp: time.Date(2025, 1, 27, 10, 30, 0, 0, time.UTC),
		Level:     "INFO",
		Scope:     "container.abc123",
		Message:   "container started",
	}

	result := m.renderLogEntry(entry)

	if !strings.Contains(result, "10:30:00") {
		t.Error("should contain timestamp")
	}
	if !strings.Contains(result, "INFO") {
		t.Error("should contain level")
	}
	if !strings.Contains(result, "container.abc123") {
		t.Error("should contain scope")
	}
	if !strings.Contains(result, "container started") {
		t.Error("should contain message")
	}
}

// Tests for renderIsolationSection

func TestRenderIsolationSection_RunningWithCache(t *testing.T) {
	m := newTestModel(t)
	info := &container.IsolationInfo{
		MemoryLimit:     "1GB",
		CPULimit:        "2",
		PidsLimit:       100,
		NetworkIsolated: true,
		NetworkName:     "test-network",
	}

	result := m.renderIsolationSection(container.StateRunning, info)
	output := strings.Join(result, "\n")

	// Should show actual values from renderIsolationInfo
	if !strings.Contains(output, "Memory:    1GB") {
		t.Error("should show memory limit")
	}
	if !strings.Contains(output, "CPUs:      2") {
		t.Error("should show CPU limit")
	}
	if !strings.Contains(output, "PIDs:      100") {
		t.Error("should show PIDs limit")
	}
	if !strings.Contains(output, "Network Isolation:") {
		t.Error("should show network isolation section")
	}
	if !strings.Contains(output, "Status:    Enabled") {
		t.Error("should show network isolation enabled")
	}
}

func TestRenderIsolationSection_RunningNoCache(t *testing.T) {
	m := newTestModel(t)

	result := m.renderIsolationSection(container.StateRunning, nil)
	output := strings.Join(result, "\n")

	// Should show Loading... for all sections
	if !strings.Contains(output, "Resource Limits:") {
		t.Error("should show Resource Limits header")
	}
	if !strings.Contains(output, "Security:") {
		t.Error("should show Security header")
	}
	if !strings.Contains(output, "Network Isolation:") {
		t.Error("should show Network Isolation header")
	}
	// Count Loading... occurrences (should be 3, one per section)
	loadingCount := strings.Count(output, "Loading...")
	if loadingCount != 3 {
		t.Errorf("expected 3 Loading... messages, got %d", loadingCount)
	}
}

func TestRenderIsolationSection_NotRunning(t *testing.T) {
	m := newTestModel(t)

	// Test with stopped state
	result := m.renderIsolationSection(container.StateStopped, nil)
	output := strings.Join(result, "\n")

	// Should show Unknown placeholders
	if !strings.Contains(output, "Resource Limits:") {
		t.Error("should show Resource Limits header")
	}
	if !strings.Contains(output, "Memory:    Unknown") {
		t.Error("should show Unknown for memory")
	}
	if !strings.Contains(output, "CPUs:      Unknown") {
		t.Error("should show Unknown for CPUs")
	}
	if !strings.Contains(output, "PIDs:      Unknown") {
		t.Error("should show Unknown for PIDs")
	}
	if !strings.Contains(output, "Security:") {
		t.Error("should show Security header")
	}
	if !strings.Contains(output, "Capabilities: Unknown") {
		t.Error("should show Unknown for capabilities")
	}
	if !strings.Contains(output, "Network Isolation:") {
		t.Error("should show Network Isolation header")
	}
	if !strings.Contains(output, "Status:    Unknown") {
		t.Error("should show Unknown for network status")
	}
	// Should NOT contain Loading...
	if strings.Contains(output, "Loading...") {
		t.Error("stopped container should not show Loading...")
	}
}

func TestRenderIsolationSection_CreatedState(t *testing.T) {
	m := newTestModel(t)

	// Test with created state (not running)
	result := m.renderIsolationSection(container.StateCreated, nil)
	output := strings.Join(result, "\n")

	// Should show Unknown placeholders (same as stopped)
	if !strings.Contains(output, "Memory:    Unknown") {
		t.Error("created container should show Unknown for memory")
	}
	if strings.Contains(output, "Loading...") {
		t.Error("created container should not show Loading...")
	}
}

// Tests for renderIsolationInfo consistency

func TestRenderIsolationInfo_NoLimits(t *testing.T) {
	m := newTestModel(t)
	info := &container.IsolationInfo{
		// No limits set
		NetworkIsolated: false,
	}

	result := m.renderIsolationInfo(info)
	output := strings.Join(result, "\n")

	// Should still show headers for consistency
	if !strings.Contains(output, "Resource Limits:") {
		t.Error("should show Resource Limits header even with no limits")
	}
	if !strings.Contains(output, "None configured") {
		t.Error("should show 'None configured' when no limits set")
	}
	if !strings.Contains(output, "Security:") {
		t.Error("should show Security header even with no capabilities")
	}
	if !strings.Contains(output, "Default capabilities") {
		t.Error("should show 'Default capabilities' when no caps modified")
	}
}

func TestRenderIsolationInfo_WithAllData(t *testing.T) {
	m := newTestModel(t)
	info := &container.IsolationInfo{
		MemoryLimit:     "2GB",
		CPULimit:        "4",
		PidsLimit:       200,
		DroppedCaps:     []string{"NET_RAW", "SYS_ADMIN"},
		AddedCaps:       []string{"NET_BIND_SERVICE"},
		NetworkIsolated: true,
		NetworkName:     "isolated-net",
		AllowedDomains:  []string{"github.com", "api.example.com"},
	}

	result := m.renderIsolationInfo(info)
	output := strings.Join(result, "\n")

	// Resource limits
	if !strings.Contains(output, "Memory:    2GB") {
		t.Error("should show memory limit")
	}
	if !strings.Contains(output, "CPUs:      4") {
		t.Error("should show CPU limit")
	}
	if !strings.Contains(output, "PIDs:      200") {
		t.Error("should show PIDs limit")
	}

	// Security
	if !strings.Contains(output, "Dropped Capabilities:") {
		t.Error("should show dropped capabilities header")
	}
	if !strings.Contains(output, "NET_RAW") {
		t.Error("should show dropped cap NET_RAW")
	}
	if !strings.Contains(output, "SYS_ADMIN") {
		t.Error("should show dropped cap SYS_ADMIN")
	}
	if !strings.Contains(output, "Added Capabilities:") {
		t.Error("should show added capabilities header")
	}
	if !strings.Contains(output, "NET_BIND_SERVICE") {
		t.Error("should show added cap NET_BIND_SERVICE")
	}

	// Network
	if !strings.Contains(output, "Status:    Enabled") {
		t.Error("should show network isolation enabled")
	}
	if !strings.Contains(output, "Network:   isolated-net") {
		t.Error("should show network name")
	}
	if !strings.Contains(output, "Allowed Domains:") {
		t.Error("should show allowed domains header")
	}
	if !strings.Contains(output, "github.com") {
		t.Error("should show allowed domain github.com")
	}
}

func TestRenderWorktreeTreeItem_Pending(t *testing.T) {
	m := newTestModel(t)
	m.setPendingWorktree("/path/to/worktree", "start")

	item := TreeItem{
		Type:         TreeItemWorktree,
		ProjectPath:  "/path/to/worktree",
		WorktreeName: "feature-branch",
	}

	result := m.renderWorktreeTreeItem(item, ">", false)

	// Should contain spinner frame (not static ◌)
	if !strings.Contains(result, "feature-branch") {
		t.Error("should contain worktree name")
	}
	if strings.Contains(result, "◌") {
		t.Error("should not contain static indicator when pending")
	}
}

func TestRenderWorktreeTreeItem_ContainerlessWorktree(t *testing.T) {
	m := newTestModel(t)

	item := TreeItem{
		Type:         TreeItemWorktree,
		ProjectPath:  "/path/to/worktree",
		WorktreeName: "main",
	}

	result := m.renderWorktreeTreeItem(item, ">", false)

	if !strings.Contains(result, "main") {
		t.Error("should contain worktree name")
	}
	if !strings.Contains(result, "◌") {
		t.Error("should contain static ◌ indicator for containerless worktree")
	}
}

func TestContextualHelp_ContainerlessWorktree(t *testing.T) {
	m := newTestModel(t)

	// Create a worktree tree item with no containers
	item := TreeItem{
		Type:         TreeItemWorktree,
		ProjectPath:  "/path/to/worktree",
		WorktreeName: "feature-branch",
	}

	m.selectedIdx = 0
	m.treeItems = []TreeItem{item}

	// Get the help text
	help := m.renderContextualHelp()

	// Should include "s: start" for containerless worktree
	if !strings.Contains(help, "s: start") {
		t.Error("help text should include 's: start' for containerless worktree")
	}
}

func TestContextualHelp_WorktreeWithContainer(t *testing.T) {
	m := newTestModel(t)

	// Add a container for the worktree
	containers := []*container.Container{
		{
			ID:          "abc123",
			Name:        "test-container",
			State:       container.StateRunning,
			ProjectPath: "/path/to/worktree",
		},
	}

	// Update model with containers
	updated, _ := m.Update(containersRefreshedMsg{containers: containers})
	m = updated.(Model)

	// Create a worktree tree item
	item := TreeItem{
		Type:         TreeItemWorktree,
		ProjectPath:  "/path/to/worktree",
		WorktreeName: "main",
	}

	m.selectedIdx = 0
	m.treeItems = []TreeItem{item}

	// Get the help text
	help := m.renderContextualHelp()

	// Should NOT include "s: start" for worktree with containers
	if strings.Contains(help, "s: start") {
		t.Error("help text should NOT include 's: start' for worktree with containers")
	}
	// Should include other worktree commands
	if !strings.Contains(help, "c: create container") {
		t.Error("help text should include 'c: create container'")
	}
}

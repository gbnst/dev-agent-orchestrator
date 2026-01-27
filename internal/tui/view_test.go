package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"devagent/internal/container"
	"devagent/internal/logging"
)

func TestRenderSessionsTabContent_NoContainer(t *testing.T) {
	m := newTestModel(t)
	m.currentTab = TabSessions
	m.selectedContainer = nil

	layout := ComputeLayout(80, 24, false)
	content := m.renderSessionsTabContent(layout)

	if !strings.Contains(content, "Select a container") {
		t.Error("should show 'Select a container' when no container selected")
	}
}

func TestRenderSessionsTabContent_WithContainer(t *testing.T) {
	m := newTestModel(t)
	m.currentTab = TabSessions
	m.selectedContainer = &container.Container{
		ID:   "abc123",
		Name: "test-container",
		Sessions: []container.Session{
			{Name: "dev", ContainerID: "abc123", Windows: 2},
			{Name: "test", ContainerID: "abc123", Windows: 1},
		},
	}
	m.selectedSessionIdx = 0

	layout := ComputeLayout(80, 24, false)
	content := m.renderSessionsTabContent(layout)

	if !strings.Contains(content, "test-container") {
		t.Error("should show container name")
	}
	if !strings.Contains(content, "dev") {
		t.Error("should show first session")
	}
	if !strings.Contains(content, "test") {
		t.Error("should show second session")
	}
}

func TestRenderSessionsTabContent_EmptySessions(t *testing.T) {
	m := newTestModel(t)
	m.currentTab = TabSessions
	m.selectedContainer = &container.Container{
		ID:       "abc123",
		Name:     "test-container",
		Sessions: []container.Session{},
	}

	layout := ComputeLayout(80, 24, false)
	content := m.renderSessionsTabContent(layout)

	if !strings.Contains(content, "No sessions") {
		t.Error("should show 'No sessions' when container has no sessions")
	}
}

func TestRenderTabs(t *testing.T) {
	styles := NewStyles("mocha")

	tests := []struct {
		name       string
		currentTab TabMode
		width      int
		wantActive string
	}{
		{
			name:       "containers tab active",
			currentTab: TabContainers,
			width:      80,
			wantActive: "1 Containers",
		},
		{
			name:       "sessions tab active",
			currentTab: TabSessions,
			width:      80,
			wantActive: "2 Sessions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderTabs(tt.currentTab, tt.width, styles)

			if !strings.Contains(result, tt.wantActive) {
				t.Errorf("renderTabs() should contain %q, got %q", tt.wantActive, result)
			}
			if !strings.Contains(result, "1 Containers") {
				t.Errorf("renderTabs() should always contain '1 Containers'")
			}
			if !strings.Contains(result, "2 Sessions") {
				t.Errorf("renderTabs() should always contain '2 Sessions'")
			}
		})
	}
}

func TestRenderTabs_FillsWidth(t *testing.T) {
	styles := NewStyles("mocha")

	result := renderTabs(TabContainers, 80, styles)
	// Should contain gap fill characters
	if !strings.Contains(result, "─") {
		t.Error("renderTabs() should contain gap fill characters")
	}
}

func TestRenderSessionDetail_ShowsSessionInfo(t *testing.T) {
	m := newTestModel(t)
	m.currentTab = TabSessions
	m.selectedContainer = &container.Container{
		ID:   "abc123",
		Name: "test-container",
		Sessions: []container.Session{
			{Name: "dev", ContainerID: "abc123", Windows: 2, Attached: false},
		},
	}
	m.selectedSessionIdx = 0
	m.sessionDetailOpen = true

	layout := ComputeLayout(80, 24, false)
	content := m.renderSessionsTabContent(layout)

	if !strings.Contains(content, "dev") {
		t.Error("should show session name")
	}
	if !strings.Contains(content, "test-container") {
		t.Error("should show container name")
	}
	if !strings.Contains(content, "2") {
		t.Error("should show window count")
	}
}

func TestRenderSessionDetail_ShowsStatus(t *testing.T) {
	m := newTestModel(t)
	m.currentTab = TabSessions
	m.selectedContainer = &container.Container{
		ID:   "abc123",
		Name: "test-container",
		Sessions: []container.Session{
			{Name: "dev", ContainerID: "abc123", Attached: true},
		},
	}
	m.selectedSessionIdx = 0
	m.sessionDetailOpen = true

	layout := ComputeLayout(80, 24, false)
	content := m.renderSessionsTabContent(layout)

	if !strings.Contains(content, "Attached") {
		t.Error("should show attached status")
	}
}

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

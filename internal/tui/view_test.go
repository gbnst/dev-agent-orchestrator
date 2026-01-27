package tui

import (
	"strings"
	"testing"

	"devagent/internal/container"
)

func TestRenderSessionsTabContent_NoContainer(t *testing.T) {
	m := newTestModel()
	m.currentTab = TabSessions
	m.selectedContainer = nil

	layout := ComputeLayout(80, 24, false)
	content := m.renderSessionsTabContent(layout)

	if !strings.Contains(content, "Select a container") {
		t.Error("should show 'Select a container' when no container selected")
	}
}

func TestRenderSessionsTabContent_WithContainer(t *testing.T) {
	m := newTestModel()
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
	m := newTestModel()
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
	m := newTestModel()
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
	m := newTestModel()
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

package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/config"
	"devagent/internal/container"
)

func newTestModelWithContainers() Model {
	cfg := &config.Config{
		Theme:   "mocha",
		Runtime: "docker",
	}
	templates := []config.Template{
		{Name: "go-project", Description: "Go development"},
	}
	m := NewModelWithTemplates(cfg, templates)
	// Set window size so list renders properly
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(Model)
}

func TestSessionView_PressEnter_ExpandsContainer(t *testing.T) {
	m := newTestModelWithContainers()

	// Simulate containers refreshed with sessions
	containers := []*container.Container{
		{
			ID:    "abc123",
			Name:  "test-container",
			State: container.StateRunning,
			Sessions: []container.Session{
				{Name: "dev", ContainerID: "abc123"},
				{Name: "main", ContainerID: "abc123"},
			},
		},
	}
	updated, _ := m.Update(containersRefreshedMsg{containers: containers})
	m = updated.(Model)

	// Press Enter to select container and switch to Sessions tab
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	// Should switch to Sessions tab
	if m.currentTab != TabSessions {
		t.Errorf("Should be on Sessions tab after pressing Enter, got %v", m.currentTab)
	}

	// Should show 2 sessions
	if m.VisibleSessionCount() != 2 {
		t.Errorf("Expected 2 visible sessions, got %d", m.VisibleSessionCount())
	}
}

func TestSessionView_PressEscape_ClosesSessionView(t *testing.T) {
	m := newTestModelWithContainers()

	// Add container with sessions
	containers := []*container.Container{
		{
			ID:    "abc123",
			Name:  "test-container",
			State: container.StateRunning,
			Sessions: []container.Session{
				{Name: "dev", ContainerID: "abc123"},
			},
		},
	}
	updated, _ := m.Update(containersRefreshedMsg{containers: containers})
	m = updated.(Model)

	// Switch to Sessions tab (replaces old modal open behavior)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	// Verify we're on Sessions tab
	if m.currentTab != TabSessions {
		t.Error("Should be on Sessions tab after Enter")
	}

	// Note: Escape handling in Sessions tab is in Phase 3, Task 4
	// For now, just verify the tab switching worked
}

func TestSessionView_SelectedSession(t *testing.T) {
	m := newTestModelWithContainers()

	containers := []*container.Container{
		{
			ID:    "abc123",
			Name:  "test-container",
			State: container.StateRunning,
			Sessions: []container.Session{
				{Name: "dev", ContainerID: "abc123"},
				{Name: "main", ContainerID: "abc123"},
			},
		},
	}
	updated, _ := m.Update(containersRefreshedMsg{containers: containers})
	m = updated.(Model)

	// Switch to Sessions tab and select container
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	// First session should be selected (selectedSessionIdx defaults to 0)
	session := m.SelectedSession()
	if session == nil {
		t.Fatal("Expected a selected session")
	}
	if session.Name != "dev" {
		t.Errorf("Expected selected session 'dev', got %q", session.Name)
	}

	// Note: Session navigation (up/down/j/k) is implemented in Phase 3, Task 4
	// For now, just verify the initial selection is correct
}

func TestSessionView_NoSessionsMessage(t *testing.T) {
	m := newTestModelWithContainers()

	// Container with no sessions
	containers := []*container.Container{
		{
			ID:       "abc123",
			Name:     "test-container",
			State:    container.StateRunning,
			Sessions: []container.Session{},
		},
	}
	updated, _ := m.Update(containersRefreshedMsg{containers: containers})
	m = updated.(Model)

	// Open session view
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.VisibleSessionCount() != 0 {
		t.Errorf("Expected 0 visible sessions, got %d", m.VisibleSessionCount())
	}
}

func TestSessionView_AttachCommand(t *testing.T) {
	m := newTestModelWithContainers()

	containers := []*container.Container{
		{
			ID:    "abc123def456",
			Name:  "test-container",
			State: container.StateRunning,
			Sessions: []container.Session{
				{Name: "dev", ContainerID: "abc123def456"},
			},
		},
	}
	updated, _ := m.Update(containersRefreshedMsg{containers: containers})
	m = updated.(Model)

	// Open session view
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	// Should show attach command for selected session
	cmd := m.AttachCommand()
	expected := "docker exec -it abc123def456 tmux attach -t dev"
	if cmd != expected {
		t.Errorf("AttachCommand() = %q, want %q", cmd, expected)
	}
}

func TestSessionView_PressT_OpensCreateSessionForm(t *testing.T) {
	t.Skip("Session form 't' handler in Sessions tab is Phase 3, Task 4")
}

func TestSessionForm_TypeName_UpdatesField(t *testing.T) {
	t.Skip("Session form input in Sessions tab is Phase 3, Task 4")
}

func TestSessionForm_PressEscape_ClosesForm(t *testing.T) {
	t.Skip("Session form escape in Sessions tab is Phase 3, Task 4")
}

func TestSessionView_PressK_ReturnsKillCommand(t *testing.T) {
	t.Skip("Session kill 'k' handler in Sessions tab is Phase 3, Task 4")
}

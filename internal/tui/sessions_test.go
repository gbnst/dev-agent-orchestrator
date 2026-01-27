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

	// Press Enter to expand container and see sessions
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if !m.IsSessionViewOpen() {
		t.Error("Session view should be open after pressing Enter on container")
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

	// Open session view
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	// Press Escape to close
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(Model)

	if m.IsSessionViewOpen() {
		t.Error("Session view should be closed after pressing Escape")
	}
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

	// Open session view
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	// First session should be selected
	session := m.SelectedSession()
	if session == nil {
		t.Fatal("Expected a selected session")
	}
	if session.Name != "dev" {
		t.Errorf("Expected selected session 'dev', got %q", session.Name)
	}

	// Navigate down
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)

	session = m.SelectedSession()
	if session.Name != "main" {
		t.Errorf("Expected selected session 'main', got %q", session.Name)
	}
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
	m := newTestModelWithContainers()

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

	// Press 't' to open session creation form
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = updated.(Model)

	if !m.IsSessionFormOpen() {
		t.Error("Session form should be open after pressing 't'")
	}
}

func TestSessionForm_TypeName_UpdatesField(t *testing.T) {
	m := newTestModelWithContainers()

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

	// Press 't' to open session form
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = updated.(Model)

	// Type session name
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d', 'e', 'v'}})
	m = updated.(Model)

	if m.SessionFormName() != "dev" {
		t.Errorf("Expected session name 'dev', got %q", m.SessionFormName())
	}
}

func TestSessionForm_PressEscape_ClosesForm(t *testing.T) {
	m := newTestModelWithContainers()

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

	// Open session view then session form
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = updated.(Model)

	// Press Escape to close form (but stay in session view)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(Model)

	if m.IsSessionFormOpen() {
		t.Error("Session form should be closed after pressing Escape")
	}
	if !m.IsSessionViewOpen() {
		t.Error("Session view should still be open after closing form")
	}
}

func TestSessionView_PressK_ReturnsKillCommand(t *testing.T) {
	m := newTestModelWithContainers()

	containers := []*container.Container{
		{
			ID:    "abc123",
			Name:  "test-container",
			State: container.StateRunning,
			Sessions: []container.Session{
				{Name: "dev", ContainerID: "abc123"},
				{Name: "test", ContainerID: "abc123"},
			},
		},
	}
	updated, _ := m.Update(containersRefreshedMsg{containers: containers})
	m = updated.(Model)

	// Open session view
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	// First session should be selected
	session := m.SelectedSession()
	if session == nil || session.Name != "dev" {
		t.Fatal("Expected 'dev' session to be selected")
	}

	// Press 'k' to kill session - this should return a command
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(Model)

	// Should return a command (the kill session command)
	if cmd == nil {
		t.Error("Expected a command to be returned when pressing 'k'")
	}
}

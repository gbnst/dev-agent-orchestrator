package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/config"
	"devagent/internal/container"
	"devagent/internal/logging"
)

func newTestModelWithContainers(t *testing.T) Model {
	cfg := &config.Config{
		Theme:   "mocha",
		Runtime: "docker",
	}
	templates := []config.Template{
		{Name: "go-project"},
	}
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test-sessions.log"
	lm, _ := logging.NewManager(logging.Config{
		FilePath:       logPath,
		MaxSizeMB:      1,
		MaxBackups:     1,
		MaxAgeDays:     1,
		ChannelBufSize: 100,
		Level:          "debug",
	})
	m := NewModelWithTemplates(cfg, templates, lm)
	// Set window size so list renders properly
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(Model)
}

func TestSessionView_PressEnter_ExpandsContainer(t *testing.T) {
	m := newTestModelWithContainers(t)

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

	// Build tree items for the container
	m.rebuildTreeItems()
	m.selectedIdx = 1 // Select container (after All)
	m.syncSelectionFromTree()

	// Press Enter to expand container (new tree behavior)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	// Container should be expanded in tree
	if !m.expandedContainers["abc123"] {
		t.Error("Container should be expanded after pressing Enter")
	}

	// Tree should now show All + container + 2 sessions = 4 items
	if len(m.treeItems) != 4 {
		t.Errorf("Expected 4 tree items (All + 1 container + 2 sessions), got %d", len(m.treeItems))
	}
}

func TestSessionView_PressEscape_ClosesSessionView(t *testing.T) {
	m := newTestModelWithContainers(t)

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

	// Build tree and open detail panel
	m.rebuildTreeItems()
	m.selectedIdx = 1 // Container (after All)
	m.syncSelectionFromTree()
	m.detailPanelOpen = true

	// Press Escape to close detail panel (new tree behavior)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(Model)

	// Detail panel should be closed
	if m.detailPanelOpen {
		t.Error("Detail panel should be closed after Escape")
	}
}

func TestSessionView_SelectedSession(t *testing.T) {
	m := newTestModelWithContainers(t)

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

	// Select container (after All) and expand it
	m.selectedIdx = 1
	m.syncSelectionFromTree()
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
}

func TestSessionView_NoSessionsMessage(t *testing.T) {
	m := newTestModelWithContainers(t)

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

	// Select container (after All) and expand it
	m.selectedIdx = 1
	m.syncSelectionFromTree()
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.VisibleSessionCount() != 0 {
		t.Errorf("Expected 0 visible sessions, got %d", m.VisibleSessionCount())
	}
}

func TestSessionView_AttachCommand(t *testing.T) {
	m := newTestModelWithContainers(t)

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

	// Select container (after All) and expand it
	m.selectedIdx = 1
	m.syncSelectionFromTree()
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	// Should show attach command for selected session
	// The command uses full runtime path (to bypass shell aliases), user flag, and container name
	cmd := m.AttachCommand()
	// Check the command contains the expected parts (path varies by system)
	if !strings.Contains(cmd, "docker") {
		t.Errorf("AttachCommand() = %q, expected to contain 'docker'", cmd)
	}
	// Should include -u vscode (default remote user)
	if !strings.Contains(cmd, "-u vscode") {
		t.Errorf("AttachCommand() = %q, expected to contain '-u vscode'", cmd)
	}
	if !strings.Contains(cmd, "test-container tmux attach -t dev") {
		t.Errorf("AttachCommand() = %q, expected to contain 'test-container tmux attach -t dev'", cmd)
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

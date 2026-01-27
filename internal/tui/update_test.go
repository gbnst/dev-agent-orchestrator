package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/container"
)

func TestTabSwitching_NumberKeys(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		startTab   TabMode
		wantTab    TabMode
	}{
		{"press 1 switches to Containers", "1", TabSessions, TabContainers},
		{"press 2 switches to Sessions", "2", TabContainers, TabSessions},
		{"press 1 stays on Containers", "1", TabContainers, TabContainers},
		{"press 2 stays on Sessions", "2", TabSessions, TabSessions},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel()
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
		name       string
		key        string
		startTab   TabMode
		wantTab    TabMode
	}{
		{"press h switches left to Containers", "h", TabSessions, TabContainers},
		{"press l switches right to Sessions", "l", TabContainers, TabSessions},
		{"press h stays on Containers (left boundary)", "h", TabContainers, TabContainers},
		{"press l stays on Sessions (right boundary)", "l", TabSessions, TabSessions},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel()
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
	m := newTestModel()

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
	m := newTestModel()

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
	m := newTestModel()
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
	m := newTestModel()
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
	m := newTestModel()
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

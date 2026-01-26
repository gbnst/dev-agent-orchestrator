package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/container"
)

// Message types for container operations.
type containersRefreshedMsg struct {
	containers []*container.Container
}

type containerErrorMsg struct {
	err error
}

type containerActionMsg struct {
	action string
	id     string
	err    error
}

type tickMsg struct {
	time time.Time
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update list size (leave room for header and help)
		listHeight := m.height - 8
		if listHeight < 0 {
			listHeight = 0
		}
		m.containerList.SetSize(m.width-4, listHeight)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "r":
			// Refresh containers
			return m, m.refreshContainers()

		case "c":
			// Create container (placeholder - will be implemented in Phase 2b)
			return m, nil

		case "s":
			// Start selected container
			if item, ok := m.containerList.SelectedItem().(containerItem); ok {
				return m, m.startContainer(item.container.ID)
			}

		case "x":
			// Stop selected container
			if item, ok := m.containerList.SelectedItem().(containerItem); ok {
				return m, m.stopContainer(item.container.ID)
			}

		case "d":
			// Destroy selected container
			if item, ok := m.containerList.SelectedItem().(containerItem); ok {
				return m, m.destroyContainer(item.container.ID)
			}
		}

		// Forward to list for navigation
		var cmd tea.Cmd
		m.containerList, cmd = m.containerList.Update(msg)
		return m, cmd

	case containersRefreshedMsg:
		m.err = nil
		items := toListItems(msg.containers)
		m.containerList.SetItems(items)
		return m, nil

	case containerErrorMsg:
		m.err = msg.err
		return m, nil

	case containerActionMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		// Refresh after action
		return m, m.refreshContainers()

	case tickMsg:
		// Periodic refresh
		return m, tea.Batch(
			m.refreshContainers(),
			m.tick(),
		)
	}

	return m, nil
}

// startContainer returns a command to start a container.
func (m Model) startContainer(id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := m.manager.Start(ctx, id)
		return containerActionMsg{action: "start", id: id, err: err}
	}
}

// stopContainer returns a command to stop a container.
func (m Model) stopContainer(id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := m.manager.Stop(ctx, id)
		return containerActionMsg{action: "stop", id: id, err: err}
	}
}

// destroyContainer returns a command to destroy a container.
func (m Model) destroyContainer(id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := m.manager.Destroy(ctx, id)
		return containerActionMsg{action: "destroy", id: id, err: err}
	}
}

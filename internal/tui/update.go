package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/container"
)

// containerCreateMsg is sent when container creation completes.
type containerCreateMsg struct {
	container *container.Container
	err       error
}

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
		// Handle form input when form is open
		if m.formOpen {
			return m.handleFormKey(msg)
		}

		// Handle session view navigation
		if m.sessionViewOpen {
			return m.handleSessionViewKey(msg)
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "r":
			// Refresh containers
			return m, m.refreshContainers()

		case "c":
			// Open container creation form
			m.openForm()
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

		case "enter":
			// Open session view for selected container
			m.openSessionView()
			return m, nil
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

	case sessionActionMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		// Refresh sessions after action
		return m, m.refreshSessions()

	case sessionsRefreshedMsg:
		// Update sessions for the container
		if m.selectedContainer != nil && m.selectedContainer.ID == msg.containerID {
			m.selectedContainer.Sessions = msg.sessions
		}
		return m, nil
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

// handleFormKey processes key events when the form is open.
func (m Model) handleFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle special keys by type first
	switch msg.Type {
	case tea.KeyEscape, tea.KeyCtrlC:
		m.resetForm()
		return m, nil

	case tea.KeyEnter:
		if !m.validateForm() {
			return m, nil
		}
		// Submit the form
		cmd := m.createContainer()
		m.resetForm()
		return m, cmd

	case tea.KeyTab:
		// Cycle through fields
		m.formFocusedField = FormField((int(m.formFocusedField) + 1) % int(fieldCount))
		return m, nil

	case tea.KeyUp:
		// Template selection
		if m.formFocusedField == FieldTemplate && m.formTemplateIdx > 0 {
			m.formTemplateIdx--
		}
		return m, nil

	case tea.KeyDown:
		// Template selection
		if m.formFocusedField == FieldTemplate && m.formTemplateIdx < len(m.templates)-1 {
			m.formTemplateIdx++
		}
		return m, nil

	case tea.KeyBackspace:
		// Delete character from focused text field
		switch m.formFocusedField {
		case FieldProjectPath:
			if len(m.formProjectPath) > 0 {
				m.formProjectPath = m.formProjectPath[:len(m.formProjectPath)-1]
			}
		case FieldContainerName:
			if len(m.formContainerName) > 0 {
				m.formContainerName = m.formContainerName[:len(m.formContainerName)-1]
			}
		}
		return m, nil

	case tea.KeyRunes:
		// Clear any previous error when typing
		m.formError = ""
		// Text input for focused field
		switch m.formFocusedField {
		case FieldProjectPath:
			m.formProjectPath += string(msg.Runes)
		case FieldContainerName:
			m.formContainerName += string(msg.Runes)
		}
		return m, nil
	}

	// Handle any other keys that have runes (fallback for text input)
	if len(msg.Runes) > 0 {
		m.formError = ""
		switch m.formFocusedField {
		case FieldProjectPath:
			m.formProjectPath += string(msg.Runes)
		case FieldContainerName:
			m.formContainerName += string(msg.Runes)
		}
		return m, nil
	}

	return m, nil
}

// createContainer returns a command to create a container with form values.
func (m Model) createContainer() tea.Cmd {
	templateName := ""
	if len(m.templates) > m.formTemplateIdx {
		templateName = m.templates[m.formTemplateIdx].Name
	}
	projectPath := m.formProjectPath
	containerName := m.formContainerName

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		_, err := m.manager.Create(ctx, container.CreateOptions{
			ProjectPath: projectPath,
			Template:    templateName,
			Name:        containerName,
		})
		return containerActionMsg{action: "create", id: containerName, err: err}
	}
}

// handleSessionViewKey processes key events when the session view is open.
func (m Model) handleSessionViewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If session form is open, handle form input
	if m.sessionFormOpen {
		return m.handleSessionFormKey(msg)
	}

	switch msg.Type {
	case tea.KeyEscape:
		m.closeSessionView()
		return m, nil

	case tea.KeyUp:
		if m.selectedSessionIdx > 0 {
			m.selectedSessionIdx--
		}
		return m, nil

	case tea.KeyDown:
		if m.selectedContainer != nil && m.selectedSessionIdx < len(m.selectedContainer.Sessions)-1 {
			m.selectedSessionIdx++
		}
		return m, nil
	}

	switch msg.String() {
	case "q":
		m.closeSessionView()
		return m, nil

	case "t":
		// Open session creation form
		m.openSessionForm()
		return m, nil

	case "k":
		// Kill selected session
		session := m.SelectedSession()
		if session != nil && m.selectedContainer != nil {
			return m, m.killSession(m.selectedContainer.ID, session.Name)
		}
		return m, nil
	}

	return m, nil
}

// handleSessionFormKey processes key events when the session form is open.
func (m Model) handleSessionFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.closeSessionForm()
		return m, nil

	case tea.KeyEnter:
		// Submit form - create session
		if m.sessionFormName != "" && m.selectedContainer != nil {
			cmd := m.createSession(m.selectedContainer.ID, m.sessionFormName)
			m.closeSessionForm()
			return m, cmd
		}
		return m, nil

	case tea.KeyBackspace:
		if len(m.sessionFormName) > 0 {
			m.sessionFormName = m.sessionFormName[:len(m.sessionFormName)-1]
		}
		return m, nil

	case tea.KeyRunes:
		m.sessionFormName += string(msg.Runes)
		return m, nil
	}

	return m, nil
}

// sessionActionMsg is sent when a session action completes.
type sessionActionMsg struct {
	action      string
	containerID string
	sessionName string
	err         error
}

// createSession returns a command to create a tmux session in a container.
func (m Model) createSession(containerID, sessionName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := m.manager.CreateSession(ctx, containerID, sessionName)
		return sessionActionMsg{
			action:      "create",
			containerID: containerID,
			sessionName: sessionName,
			err:         err,
		}
	}
}

// killSession returns a command to kill a tmux session in a container.
func (m Model) killSession(containerID, sessionName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := m.manager.KillSession(ctx, containerID, sessionName)
		return sessionActionMsg{
			action:      "kill",
			containerID: containerID,
			sessionName: sessionName,
			err:         err,
		}
	}
}

// pattern: Imperative Shell

package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/container"
	"devagent/internal/logging"
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

// logEntriesMsg delivers log entries from the logging channel.
type logEntriesMsg struct {
	entries []logging.LogEntry
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Use Layout for consistent height calculation
		layout := ComputeLayout(m.width, m.height, m.logPanelOpen, m.detailPanelOpen)
		listHeight := layout.ContentListHeight()

		m.containerList.SetSize(m.width-4, listHeight)

		// Initialize or update log viewport
		if m.logPanelOpen {
			if !m.logReady {
				m.logViewport = viewport.New(layout.Logs.Width, layout.Logs.Height-1)
				m.logReady = true
			} else {
				m.logViewport.Width = layout.Logs.Width
				m.logViewport.Height = layout.Logs.Height - 1
			}
			m.updateLogViewportContent()
		}

		return m, nil

	case spinner.TickMsg:
		if m.statusLevel == StatusLoading {
			var cmd tea.Cmd
			m.statusSpinner, cmd = m.statusSpinner.Update(msg)

			// Update list delegate with new spinner frame
			m.containerDelegate = m.containerDelegate.WithSpinnerState(m.statusSpinner.View(), m.pendingOperations)
			m.containerList.SetDelegate(m.containerDelegate)

			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		// Debug all key presses
		m.logger.Debug("key pressed", "key", msg.String(), "type", msg.Type, "hasSelectedContainer", m.selectedContainer != nil, "formOpen", m.formOpen, "sessionViewOpen", m.sessionViewOpen, "sessionFormOpen", m.sessionFormOpen)

		// Clear error with Escape
		if msg.Type == tea.KeyEscape && m.statusLevel == StatusError {
			m.clearStatus()
			return m, nil
		}

		// Handle form input when form is open
		if m.formOpen {
			return m.handleFormKey(msg)
		}

		// Handle session form input when session form is open
		if m.sessionFormOpen {
			return m.handleSessionFormKey(msg)
		}

		// Handle session view navigation
		if m.sessionViewOpen {
			return m.handleSessionViewKey(msg)
		}

		// Handle tree navigation when tree items exist and tree is focused
		if len(m.treeItems) > 0 && m.panelFocus == FocusTree {
			switch msg.Type {
			case tea.KeyUp:
				m.moveTreeSelectionUp()
				return m, nil
			case tea.KeyDown:
				m.moveTreeSelectionDown()
				return m, nil
			case tea.KeyEnter:
				// Toggle expand/collapse for containers
				if m.selectedIdx >= 0 && m.selectedIdx < len(m.treeItems) {
					item := m.treeItems[m.selectedIdx]
					if item.Type == TreeItemContainer {
						m.toggleTreeExpand()
						return m, nil
					}
				}
			case tea.KeyRight:
				// Open detail panel
				m.detailPanelOpen = true
				return m, nil
			case tea.KeyLeft:
				// Close detail panel
				if m.detailPanelOpen {
					m.detailPanelOpen = false
					return m, nil
				}
			case tea.KeyEscape:
				// Close detail panel (if open)
				if m.detailPanelOpen {
					m.detailPanelOpen = false
					return m, nil
				}
			}
		}

		// Handle log viewport navigation when log panel is focused
		if m.panelFocus == FocusLogs && m.logPanelOpen && m.logReady {
			switch msg.Type {
			case tea.KeyUp:
				if m.logViewport.YOffset > 0 {
					m.logViewport.SetYOffset(m.logViewport.YOffset - 1)
				}
				m.logAutoScroll = false
				return m, nil
			case tea.KeyDown:
				m.logViewport.SetYOffset(m.logViewport.YOffset + 1)
				m.logAutoScroll = m.logViewport.AtBottom()
				return m, nil
			case tea.KeyEscape:
				m.panelFocus = FocusTree
				return m, nil
			}
		}

		// Handle detail panel Escape to return focus to tree
		if m.panelFocus == FocusDetail {
			if msg.Type == tea.KeyEscape {
				m.panelFocus = FocusTree
				return m, nil
			}
		}

		switch msg.String() {
		case "tab":
			m.panelFocus = m.nextFocus()
			return m, nil

		case "q", "ctrl+c":
			m.logger.Debug("quit command received")
			return m, tea.Quit

		case "r":
			// Refresh containers
			m.logger.Debug("refresh containers requested")
			return m, m.refreshContainers()

		case "c":
			// Open container creation form
			m.logger.Debug("opening container creation form")
			m.openForm()
			return m, nil

		case "s":
			// Start selected container
			if item, ok := m.containerList.SelectedItem().(containerItem); ok {
				m.logger.Info("starting container", "containerID", item.container.ID, "name", item.container.Name)
				m.setPending(item.container.ID, "start")
				cmd := m.setLoading("Starting " + item.container.Name + "...")
				return m, tea.Batch(cmd, m.startContainer(item.container.ID))
			}

		case "x":
			// Stop selected container
			if item, ok := m.containerList.SelectedItem().(containerItem); ok {
				m.logger.Info("stopping container", "containerID", item.container.ID, "name", item.container.Name)
				m.setPending(item.container.ID, "stop")
				cmd := m.setLoading("Stopping " + item.container.Name + "...")
				return m, tea.Batch(cmd, m.stopContainer(item.container.ID))
			}

		case "d":
			// Destroy selected container
			if item, ok := m.containerList.SelectedItem().(containerItem); ok {
				m.logger.Info("destroying container", "containerID", item.container.ID, "name", item.container.Name)
				m.setPending(item.container.ID, "destroy")
				cmd := m.setLoading("Destroying " + item.container.Name + "...")
				return m, tea.Batch(cmd, m.destroyContainer(item.container.ID))
			}

		case "t":
			// Create session in selected container
			if m.selectedContainer != nil {
				m.logger.Debug("opening session form")
				m.openSessionForm()
				return m, nil
			}

		case "l", "L":
			// Toggle log panel
			m.logger.Debug("toggling log panel", "visible", !m.logPanelOpen)
			m.logPanelOpen = !m.logPanelOpen
			if !m.logPanelOpen && m.panelFocus == FocusLogs {
				m.panelFocus = FocusTree
			}
			if m.logPanelOpen {
				// Recalculate layout and initialize viewport if needed
				layout := ComputeLayout(m.width, m.height, m.logPanelOpen, m.detailPanelOpen)
				if !m.logReady {
					m.logViewport = viewport.New(layout.Logs.Width, layout.Logs.Height-1)
					m.logReady = true
				}
				m.updateLogViewportContent()
			}
			// Recalculate list size for split layout
			layout := ComputeLayout(m.width, m.height, m.logPanelOpen, m.detailPanelOpen)
			m.containerList.SetSize(m.width-4, layout.ContentListHeight())
			return m, nil

		case "j":
			// Scroll logs down when panel is open
			if m.logPanelOpen && m.logReady {
				m.logViewport.SetYOffset(m.logViewport.YOffset + 1)
				m.logAutoScroll = m.logViewport.AtBottom()
				return m, nil
			}
			// Fall through to container list navigation if not handled

		case "k":
			// Scroll logs up when panel is open
			if m.logPanelOpen && m.logReady {
				if m.logViewport.YOffset > 0 {
					m.logViewport.SetYOffset(m.logViewport.YOffset - 1)
				}
				m.logAutoScroll = false
				return m, nil
			}
			// Fall through to container list navigation

		case "g":
			// Go to top of logs when panel is open
			if m.logPanelOpen && m.logReady {
				m.logViewport.GotoTop()
				m.logAutoScroll = false
				return m, nil
			}

		case "G":
			// Go to bottom of logs when panel is open
			if m.logPanelOpen && m.logReady {
				m.logViewport.GotoBottom()
				m.logAutoScroll = true
				return m, nil
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
		// Rebuild tree items after container refresh
		m.rebuildTreeItems()
		// Sync selection after rebuild
		m.syncSelectionFromTree()
		return m, nil

	case containerErrorMsg:
		m.logger.Error("container operation error", "error", msg.err)
		m.err = msg.err
		return m, nil

	case containerActionMsg:
		// Clear pending state regardless of success/error
		m.clearPending(msg.id)

		if msg.err != nil {
			m.logger.Error("container action failed", "action", msg.action, "containerID", msg.id, "error", msg.err)
			m.setError(fmt.Sprintf("Failed to %s container", msg.action), msg.err)
			return m, nil
		}
		m.logger.Info("container action completed", "action", msg.action, "containerID", msg.id)
		actionNames := map[string]string{
			"create":  "created",
			"start":   "started",
			"stop":    "stopped",
			"destroy": "destroyed",
		}
		m.setSuccess(fmt.Sprintf("Container %s", actionNames[msg.action]))
		return m, m.refreshContainers()

	case tickMsg:
		// Periodic refresh
		m.logger.Debug("periodic refresh triggered")
		cmds := []tea.Cmd{
			m.refreshContainers(),
			m.tick(),
		}
		// Also refresh sessions if we have a selected container
		if m.selectedContainer != nil {
			cmds = append(cmds, m.refreshSessions())
		}
		return m, tea.Batch(cmds...)

	case sessionActionMsg:
		if msg.err != nil {
			m.logger.Error("session action failed", "action", msg.action, "containerID", msg.containerID, "session", msg.sessionName, "error", msg.err)
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

	case logEntriesMsg:
		for _, entry := range msg.entries {
			m.addLogEntry(entry)
		}
		if m.logPanelOpen && m.logReady {
			m.updateLogViewportContent()
		}
		// Continue consuming logs (logManager added in Phase 7)
		if m.logManager != nil {
			return m, m.consumeLogEntries(m.logManager)
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
		containerName := m.formContainerName
		m.logger.Info("creating container", "name", containerName)
		m.setPending(containerName, "create")
		loadingCmd := m.setLoading("Creating " + containerName + "...")
		createCmd := m.createContainer()
		m.resetForm()
		return m, tea.Batch(loadingCmd, createCmd)

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

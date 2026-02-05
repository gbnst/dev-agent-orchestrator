// pattern: Imperative Shell

package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/container"
	"devagent/internal/logging"
)

// doubleCtrlCWindow is the maximum time between two ctrl+c presses to trigger quit.
const doubleCtrlCWindow = 500 * time.Millisecond

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

// clearStatusMsg is sent after a timed delay to clear the status bar.
type clearStatusMsg struct{}

// formAutoCloseMsg is sent to auto-close the form after completion.
type formAutoCloseMsg struct{}

// isolationInfoMsg is sent when isolation info is fetched for the selected container.
type isolationInfoMsg struct {
	info        *container.IsolationInfo
	containerID string
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

		// Initialize or update detail viewport
		if m.detailPanelOpen {
			// Account for panel header (1 line) and border padding (2 lines)
			detailHeight := layout.Detail.Height - 3
			if detailHeight < 1 {
				detailHeight = 1
			}
			// Account for border padding on width
			detailWidth := layout.Detail.Width - 4
			if detailWidth < 1 {
				detailWidth = 1
			}
			if !m.detailReady {
				m.detailViewport = viewport.New(detailWidth, detailHeight)
				m.detailReady = true
			} else {
				m.detailViewport.Width = detailWidth
				m.detailViewport.Height = detailHeight
			}
			m.updateDetailViewportContent()
		}

		return m, nil

	case spinner.TickMsg:
		var cmds []tea.Cmd

		// Update status spinner if loading
		if m.statusLevel == StatusLoading {
			var cmd tea.Cmd
			m.statusSpinner, cmd = m.statusSpinner.Update(msg)
			cmds = append(cmds, cmd)

			// Update list delegate with new spinner frame
			m.containerDelegate = m.containerDelegate.WithSpinnerState(m.statusSpinner.View(), m.pendingOperations)
			m.containerList.SetDelegate(m.containerDelegate)
		}

		// Update form status spinner if submitting
		if m.formSubmitting {
			var cmd tea.Cmd
			m.formStatusSpinner, cmd = m.formStatusSpinner.Update(msg)
			cmds = append(cmds, cmd)
		}

		if len(cmds) > 0 {
			return m, tea.Batch(cmds...)
		}
		return m, nil

	case tea.KeyMsg:
		// Debug all key presses
		m.logger.Debug("key pressed", "key", msg.String(), "type", msg.Type, "hasSelectedContainer", m.selectedContainer != nil, "formOpen", m.formOpen, "sessionViewOpen", m.sessionViewOpen, "sessionFormOpen", m.sessionFormOpen)

		// Handle quit shortcuts first (ctrl+d always, ctrl+c double-press)
		if msg.Type == tea.KeyCtrlD {
			m.logger.Debug("quit via ctrl+d")
			return m, tea.Quit
		}
		if msg.Type == tea.KeyCtrlC {
			now := time.Now()
			if !m.lastCtrlCTime.IsZero() && now.Sub(m.lastCtrlCTime) <= doubleCtrlCWindow {
				m.logger.Debug("quit via double ctrl+c")
				return m, tea.Quit
			}
			m.lastCtrlCTime = now
			return m, nil
		}

		// Clear error with Escape
		if msg.Type == tea.KeyEscape && m.statusLevel == StatusError {
			m.clearStatus()
			m.quitHintCount = 0
			return m, nil
		}

		// Handle confirmation dialog
		if m.confirmOpen {
			return m.handleConfirmKey(msg)
		}

		// Handle action menu
		if m.actionMenuOpen {
			return m.handleActionMenuKey(msg)
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
				return m, m.fetchIsolationInfoIfNeeded()
			case tea.KeyDown:
				m.moveTreeSelectionDown()
				return m, m.fetchIsolationInfoIfNeeded()
			case tea.KeyEnter:
				// Toggle expand/collapse for containers, open detail for others
				if m.selectedIdx >= 0 && m.selectedIdx < len(m.treeItems) {
					item := m.treeItems[m.selectedIdx]
					if item.Type == TreeItemContainer {
						m.toggleTreeExpand()
						return m, nil
					}
				}
			case tea.KeyRight:
				// Open detail panel and initialize viewport
				m.detailPanelOpen = true
				m.initDetailViewport()
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
					m.quitHintCount = 0
					return m, nil
				}
				// Nothing to close — track for hint
				m.quitHintCount++
				if m.quitHintCount >= 2 {
					m.statusLevel = StatusInfo
					m.statusMessage = "ctrl+c ctrl+c to quit"
					m.quitHintCount = 0
					return m, tea.Tick(4*time.Second, func(time.Time) tea.Msg {
						return clearStatusMsg{}
					})
				}
				return m, nil
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
				m.quitHintCount = 0
				return m, nil
			}
		}

		// Handle detail panel Escape/Left to return focus to tree or close panel
		if m.panelFocus == FocusDetail && m.detailPanelOpen {
			if msg.Type == tea.KeyEscape {
				m.panelFocus = FocusTree
				m.quitHintCount = 0
				return m, nil
			}
			if msg.Type == tea.KeyLeft {
				m.detailPanelOpen = false
				m.panelFocus = FocusTree
				return m, nil
			}
		}

		// Handle detail panel scrolling when focused and viewport ready
		if m.panelFocus == FocusDetail && m.detailPanelOpen && m.detailReady {
			switch msg.Type {
			case tea.KeyUp:
				m.detailViewport.ScrollUp(1)
				return m, nil
			case tea.KeyDown:
				m.detailViewport.ScrollDown(1)
				return m, nil
			case tea.KeyPgUp:
				m.detailViewport.HalfPageUp()
				return m, nil
			case tea.KeyPgDown:
				m.detailViewport.HalfPageDown()
				return m, nil
			}

			// Also handle j/k/g/G for vim-style navigation
			switch msg.String() {
			case "j":
				m.detailViewport.ScrollDown(1)
				return m, nil
			case "k":
				m.detailViewport.ScrollUp(1)
				return m, nil
			case "g":
				m.detailViewport.GotoTop()
				return m, nil
			case "G":
				m.detailViewport.GotoBottom()
				return m, nil
			}
		}

		// "q" triggers the same quit hint as escape
		if msg.String() == "q" {
			m.quitHintCount++
			if m.quitHintCount >= 2 {
				m.statusLevel = StatusInfo
				m.statusMessage = "ctrl+c ctrl+c to quit"
				m.quitHintCount = 0
				return m, tea.Tick(4*time.Second, func(time.Time) tea.Msg {
					return clearStatusMsg{}
				})
			}
			return m, nil
		}

		// Reset quit hint count on any other key
		m.quitHintCount = 0

		switch msg.String() {
		case "tab":
			m.panelFocus = m.nextFocus()
			return m, nil

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
			// Start selected container (no-op when All Containers is selected)
			if m.selectedContainer == nil {
				break
			}
			if item, ok := m.containerList.SelectedItem().(containerItem); ok {
				m.logger.Info("starting container", "containerID", item.container.ID, "name", item.container.Name)
				m.setPending(item.container.ID, "start")
				cmd := m.setLoading("Starting " + item.container.Name + "...")
				return m, tea.Batch(cmd, m.startContainer(item.container.ID))
			}

		case "x":
			// Stop selected container (no-op when All Containers is selected)
			if m.selectedContainer == nil {
				break
			}
			if item, ok := m.containerList.SelectedItem().(containerItem); ok {
				m.logger.Info("stopping container", "containerID", item.container.ID, "name", item.container.Name)
				m.setPending(item.container.ID, "stop")
				cmd := m.setLoading("Stopping " + item.container.Name + "...")
				return m, tea.Batch(cmd, m.stopContainer(item.container.ID))
			}

		case "d":
			// Destroy selected container (no-op when All Containers is selected)
			if m.selectedContainer == nil {
				break
			}
			if item, ok := m.containerList.SelectedItem().(containerItem); ok {
				// Show confirmation dialog
				m.confirmOpen = true
				m.confirmAction = "destroy_container"
				m.confirmTarget = item.container.ID
				m.confirmMessage = fmt.Sprintf("Destroy container '%s'?", item.container.Name)
				return m, nil
			}

		case "t":
			// Open action menu for selected container
			if m.selectedContainer != nil && m.selectedContainer.State == container.StateRunning {
				m.logger.Debug("opening action menu")
				m.actionMenuOpen = true
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
			// Kill selected session from tree view
			if m.selectedIdx >= 0 && m.selectedIdx < len(m.treeItems) && m.treeItems[m.selectedIdx].Type == TreeItemSession {
				session := m.SelectedSession()
				if session != nil && m.selectedContainer != nil {
					// Show confirmation dialog
					m.confirmOpen = true
					m.confirmAction = "kill_session"
					m.confirmTarget = session.Name
					m.confirmMessage = fmt.Sprintf("Kill session '%s'?", session.Name)
					return m, nil
				}
			}
			// Scroll logs up when panel is open
			if m.logPanelOpen && m.logReady {
				if m.logViewport.YOffset > 0 {
					m.logViewport.SetYOffset(m.logViewport.YOffset - 1)
				}
				m.logAutoScroll = false
				return m, nil
			}

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
		// Sync selection after rebuild, but preserve selectedContainer if session view is open
		// to prevent the modal from "rotating" through containers during periodic refresh
		if !m.sessionViewOpen {
			m.syncSelectionFromTree()
		}
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
			m.setLogFilterFromContext() // Explicit: filter logs to the container context on error
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
			m.refreshAllSessions(),
		}
		return m, tea.Batch(cmds...)

	case sessionActionMsg:
		if msg.err != nil {
			m.logger.Error("session action failed", "action", msg.action, "containerID", msg.containerID, "session", msg.sessionName, "error", msg.err)
			m.err = msg.err
			return m, nil
		}
		// Show confirmation dialog for session creation
		if msg.action == "create" {
			m.sessionCreatedOpen = true
			m.sessionCreatedName = msg.sessionName
		}
		// Refresh sessions after action
		return m, m.refreshSessions()

	case sessionsRefreshedMsg:
		// Update sessions for the container
		if m.selectedContainer != nil && m.selectedContainer.ID == msg.containerID {
			m.selectedContainer.Sessions = msg.sessions
		}
		return m, nil

	case allSessionsRefreshedMsg:
		// Update sessions for all containers in the list
		for i, item := range m.containerList.Items() {
			ci, ok := item.(containerItem)
			if !ok {
				continue
			}
			if sessions, found := msg.sessionsByContainer[ci.container.ID]; found {
				ci.container.Sessions = sessions
				items := m.containerList.Items()
				items[i] = ci
			}
		}
		m.rebuildTreeItems()
		// Preserve selectedContainer if session view is open
		if !m.sessionViewOpen {
			m.syncSelectionFromTree()
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

	case clearStatusMsg:
		// Only clear if still showing the quit hint (don't clobber other status)
		if m.statusLevel == StatusInfo && m.statusMessage == "ctrl+c ctrl+c to quit" {
			m.clearStatus()
		}
		return m, nil

	case isolationInfoMsg:
		// Update cached isolation info if it's still for the selected container
		if m.selectedContainer != nil && m.selectedContainer.ID == msg.containerID {
			m.cachedIsolationInfo = msg.info
			// Refresh detail viewport to show the new info
			if m.detailReady && m.detailPanelOpen {
				m.updateDetailViewportContent()
			}
		}
		return m, nil

	case formProgressMsg:
		// Handle individual progress update
		switch msg.step.Status {
		case "started":
			m.setFormCurrentStep(msg.step.Message)
		case "completed":
			m.addFormStatusStep(true, msg.step.Message)
			m.formCurrentStep = ""
		case "failed":
			m.addFormStatusStep(false, msg.step.Message)
			m.formCurrentStep = ""
		}
		// Continue waiting for more progress
		return m, waitForProgress(m.formProgressChan, "")

	case formCreationDoneMsg:
		// Clear pending operation
		m.clearPending(msg.id)

		// Close the progress channel
		if m.formProgressChan != nil {
			close(m.formProgressChan)
			m.formProgressChan = nil
		}

		// Handle completion
		if msg.err != nil {
			m.logger.Error("container creation failed", "error", msg.err)
			m.addFormStatusStep(false, "Creation failed: "+msg.err.Error())
			closeCmd := m.finishFormSubmission(false)
			return m, closeCmd
		}

		m.logger.Info("container action completed", "action", "create", "containerID", msg.id)
		closeCmd := m.finishFormSubmission(true)
		refreshCmd := m.refreshContainers()
		return m, tea.Batch(closeCmd, refreshCmd)

	case formAutoCloseMsg:
		// Auto-close the form after completion delay
		if m.formCompleted {
			m.resetForm()
		}
		return m, nil

	case formTitlePulseMsg:
		// Cycle the title pulse if still submitting
		if m.formSubmitting {
			m.formTitlePulse = (m.formTitlePulse + 1) % 4
			return m, tickTitlePulse()
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

		err := m.manager.StartWithCompose(ctx, id)
		return containerActionMsg{action: "start", id: id, err: err}
	}
}

// stopContainer returns a command to stop a container.
func (m Model) stopContainer(id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := m.manager.StopWithCompose(ctx, id)
		return containerActionMsg{action: "stop", id: id, err: err}
	}
}

// destroyContainer returns a command to destroy a container.
func (m Model) destroyContainer(id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := m.manager.DestroyWithCompose(ctx, id)
		return containerActionMsg{action: "destroy", id: id, err: err}
	}
}

// handleFormKey processes key events when the form is open.
func (m Model) handleFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If form is submitting, only allow Escape to cancel
	if m.formSubmitting {
		if msg.Type == tea.KeyEscape {
			// Cancel the submission - the container creation is async so we just close
			m.resetForm()
			return m, nil
		}
		// Block all other input during submission
		return m, nil
	}

	// If form is completed (showing result), Enter or Escape closes it
	if m.formCompleted {
		if msg.Type == tea.KeyEnter || msg.Type == tea.KeyEscape {
			m.resetForm()
		}
		return m, nil
	}

	// Handle special keys by type first
	switch msg.Type {
	case tea.KeyEscape:
		m.resetForm()
		return m, nil

	case tea.KeyEnter:
		if errMsg := m.validateForm(); errMsg != "" {
			m.formError = errMsg
			return m, nil
		}
		// Submit the form - keep it open with progress
		containerName := m.formContainerName
		m.logger.Info("creating container", "name", containerName)
		m.setPending(containerName, "create")
		spinnerCmd := m.startFormSubmission()
		// Create the progress channel and store it in the model
		m.formProgressChan = make(chan formProgressMsg, 20)
		createCmd := m.createContainerWithProgress()
		return m, tea.Batch(spinnerCmd, createCmd)

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

// formProgressMsg delivers a single progress update during container creation.
type formProgressMsg struct {
	step container.ProgressStep
}

// formCreationDoneMsg is sent when container creation completes.
type formCreationDoneMsg struct {
	err error
	id  string
}

// createContainerWithProgress returns a command to create a container with progress reporting.
// The caller must set m.formProgressChan before calling this function.
func (m Model) createContainerWithProgress() tea.Cmd {
	templateName := ""
	if len(m.templates) > m.formTemplateIdx {
		templateName = m.templates[m.formTemplateIdx].Name
	}
	// Trim whitespace from form inputs to avoid invalid container names
	projectPath := strings.TrimSpace(m.formProjectPath)
	containerName := strings.TrimSpace(m.formContainerName)

	// Capture the channel for use in goroutine
	progressChan := m.formProgressChan

	// Start container creation in background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		_, err := m.manager.CreateWithCompose(ctx, container.CreateOptions{
			ProjectPath: projectPath,
			Template:    templateName,
			Name:        containerName,
			OnProgress: func(step container.ProgressStep) {
				// Send progress to channel (non-blocking)
				select {
				case progressChan <- formProgressMsg{step: step}:
				default:
				}
			},
		})

		// Send completion or error message (mutually exclusive)
		if err != nil {
			select {
			case progressChan <- formProgressMsg{step: container.ProgressStep{
				Step:    "error",
				Status:  "failed",
				Message: err.Error(),
			}}:
			default:
			}
		} else {
			select {
			case progressChan <- formProgressMsg{step: container.ProgressStep{
				Step:   "done",
				Status: "completed",
			}}:
			default:
			}
		}
	}()

	// Return command to wait for first progress message
	return waitForProgress(progressChan, containerName)
}

// waitForProgress returns a command that waits for the next progress message.
func waitForProgress(progressChan chan formProgressMsg, containerName string) tea.Cmd {
	return func() tea.Msg {
		if progressChan == nil {
			return formCreationDoneMsg{id: containerName, err: nil}
		}

		msg, ok := <-progressChan
		if !ok {
			// Channel closed
			return formCreationDoneMsg{id: containerName, err: nil}
		}

		// Check for completion signals
		if msg.step.Step == "done" {
			return formCreationDoneMsg{id: containerName, err: nil}
		}
		if msg.step.Step == "error" {
			return formCreationDoneMsg{id: containerName, err: fmt.Errorf("%s", msg.step.Message)}
		}

		return msg
	}
}

// fetchIsolationInfoIfNeeded returns a command to fetch isolation info if a running container is selected.
func (m Model) fetchIsolationInfoIfNeeded() tea.Cmd {
	if m.selectedContainer == nil {
		return nil
	}
	if m.selectedContainer.State != container.StateRunning {
		return nil
	}
	return m.fetchIsolationInfo(m.selectedContainer.ID)
}

// fetchIsolationInfo returns a command to fetch isolation info for a container.
func (m Model) fetchIsolationInfo(containerID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Find the container
		c, ok := m.manager.Get(containerID)
		if !ok {
			return isolationInfoMsg{info: nil, containerID: containerID}
		}

		info, err := m.manager.GetContainerIsolationInfo(ctx, c)
		if err != nil {
			return isolationInfoMsg{info: nil, containerID: containerID}
		}

		return isolationInfoMsg{info: info, containerID: containerID}
	}
}

// handleSessionViewKey processes key events when the session view is open.
func (m Model) handleSessionViewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If session form is open, handle form input
	if m.sessionFormOpen {
		return m.handleSessionFormKey(msg)
	}

	// If session created confirmation is open, handle it
	if m.sessionCreatedOpen {
		return m.handleSessionCreatedKey(msg)
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
	case "t":
		// Open session creation form
		m.openSessionForm()
		return m, nil

	case "k":
		// Kill selected session - show confirmation dialog
		session := m.SelectedSession()
		if session != nil && m.selectedContainer != nil {
			m.confirmOpen = true
			m.confirmAction = "kill_session"
			m.confirmTarget = session.Name
			m.confirmMessage = fmt.Sprintf("Kill session '%s'?", session.Name)
			return m, nil
		}
		return m, nil
	}

	return m, nil
}

// handleSessionCreatedKey processes key events when the session created confirmation is open.
func (m Model) handleSessionCreatedKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		// Close confirmation and go back to main view
		m.sessionCreatedOpen = false
		m.sessionCreatedName = ""
		m.closeSessionView()
		return m, nil
	}

	switch msg.String() {
	case "k":
		// Kill the just-created session
		if m.selectedContainer != nil && m.sessionCreatedName != "" {
			cmd := m.killSession(m.selectedContainer.ID, m.sessionCreatedName)
			m.sessionCreatedOpen = false
			m.sessionCreatedName = ""
			m.closeSessionView()
			return m, cmd
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

// handleConfirmKey processes key events when the confirmation dialog is open.
func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		// Cancel the action
		m.confirmOpen = false
		m.confirmAction = ""
		m.confirmTarget = ""
		m.confirmMessage = ""
		return m, nil

	case tea.KeyEnter:
		// Confirm and execute the action
		action := m.confirmAction
		target := m.confirmTarget

		// Close dialog
		m.confirmOpen = false
		m.confirmAction = ""
		m.confirmTarget = ""
		m.confirmMessage = ""

		// Execute the confirmed action
		switch action {
		case "destroy_container":
			// Find the container to get its name for the loading message
			var containerName string
			for _, item := range m.containerList.Items() {
				if ci, ok := item.(containerItem); ok && ci.container.ID == target {
					containerName = ci.container.Name
					break
				}
			}
			m.logger.Info("destroying container", "containerID", target, "name", containerName)
			m.setPending(target, "destroy")
			cmd := m.setLoading("Destroying " + containerName + "...")
			return m, tea.Batch(cmd, m.destroyContainer(target))

		case "kill_session":
			if m.selectedContainer != nil {
				m.logger.Info("killing session", "containerID", m.selectedContainer.ID, "session", target)
				return m, m.killSession(m.selectedContainer.ID, target)
			}
		}
		return m, nil
	}

	// 'y' also confirms
	if msg.String() == "y" || msg.String() == "Y" {
		// Simulate Enter press to confirm
		return m.handleConfirmKey(tea.KeyMsg{Type: tea.KeyEnter})
	}

	// 'n' also cancels
	if msg.String() == "n" || msg.String() == "N" {
		return m.handleConfirmKey(tea.KeyMsg{Type: tea.KeyEscape})
	}

	return m, nil
}

// handleActionMenuKey processes key events when the action menu is open.
func (m Model) handleActionMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.closeActionMenu()
		return m, nil
	}
	return m, nil
}

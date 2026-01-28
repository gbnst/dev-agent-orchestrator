// pattern: Imperative Shell

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"devagent/internal/container"
	"devagent/internal/logging"
)

// View renders the TUI.
func (m Model) View() string {
	// Session detail is a modal overlay (keep this one centered for now)
	if m.sessionViewOpen {
		return m.renderSessionView()
	}

	// Compute layout regions
	layout := ComputeLayout(m.width, m.height, m.logPanelOpen, m.detailPanelOpen)

	// Build header
	header := m.styles.TitleStyle().Width(layout.Header.Width).Render("Development Agent Orchestrator")

	// Build content: tree view + optional detail panel
	var content string
	if m.formOpen {
		// Container creation form replaces content area
		content = m.renderCreateForm()
	} else {
		// Render tree view (always shown)
		treeView := m.renderTree(layout)

		// Optionally render detail panel
		if m.detailPanelOpen {
			detailPanel := m.renderDetailPanel(layout)
			content = lipgloss.JoinHorizontal(lipgloss.Top, treeView, detailPanel)
		} else {
			content = treeView
		}
	}

	// Build status bar with operation feedback and contextual help
	statusBar := lipgloss.NewStyle().Width(layout.StatusBar.Width).Render(m.renderStatusBar(layout.StatusBar.Width))

	// Error display (if any)
	var errorDisplay string
	if m.err != nil {
		errorDisplay = m.styles.ErrorStyle().Render("Error: " + m.err.Error())
	}

	// Compose full layout
	parts := []string{header, content}

	// Add log panel if open
	if m.logPanelOpen {
		separator := lipgloss.NewStyle().
			Width(layout.Separator.Width).
			Foreground(lipgloss.Color(m.styles.flavor.Surface1().Hex)).
			Render(strings.Repeat("─", layout.Separator.Width))
		parts = append(parts, separator)
		parts = append(parts, m.renderLogPanel(layout))
	}

	if errorDisplay != "" {
		parts = append(parts, errorDisplay)
	}
	parts = append(parts, statusBar)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// renderCreateForm renders the container creation form as a left-justified input area.
func (m Model) renderCreateForm() string {
	title := m.styles.TitleStyle().Render("Create Container")

	// Template selection - compact horizontal display
	templateLabel := "Template: "
	if m.formFocusedField == FieldTemplate {
		templateLabel = m.styles.AccentStyle().Render("▸ Template: ")
	}

	var templateValue string
	if len(m.templates) > 0 && m.formTemplateIdx < len(m.templates) {
		tmpl := m.templates[m.formTemplateIdx]
		templateValue = m.styles.AccentStyle().Render(tmpl.Name)
		if m.formFocusedField == FieldTemplate {
			templateValue += m.styles.HelpStyle().Render(fmt.Sprintf(" (↑↓ to change, %d/%d)", m.formTemplateIdx+1, len(m.templates)))
		}
	} else {
		templateValue = m.styles.ErrorStyle().Render("No templates available")
	}
	templateLine := templateLabel + templateValue

	// Project path input - single line
	projectPathLabel := "Project Path: "
	if m.formFocusedField == FieldProjectPath {
		projectPathLabel = m.styles.AccentStyle().Render("▸ Project Path: ")
	}
	projectPathValue := m.formProjectPath
	if projectPathValue == "" && m.formFocusedField != FieldProjectPath {
		projectPathValue = m.styles.SubtitleStyle().Render("(required)")
	}
	if m.formFocusedField == FieldProjectPath {
		projectPathValue += "_" // cursor
	}
	projectPathLine := projectPathLabel + projectPathValue

	// Container name input - single line
	nameLabel := "Name: "
	if m.formFocusedField == FieldContainerName {
		nameLabel = m.styles.AccentStyle().Render("▸ Name: ")
	}
	nameValue := m.formContainerName
	if nameValue == "" && m.formFocusedField != FieldContainerName {
		nameValue = m.styles.SubtitleStyle().Render("(optional, auto-generated)")
	}
	if m.formFocusedField == FieldContainerName {
		nameValue += "_" // cursor
	}
	nameLine := nameLabel + nameValue

	// Error display
	var errorLine string
	if m.formError != "" {
		errorLine = m.styles.ErrorStyle().Render("Error: " + m.formError)
	}

	// Help text
	help := m.styles.HelpStyle().Render("Tab: next field • Enter: create • Esc: cancel")

	parts := []string{
		title,
		"",
		templateLine,
		projectPathLine,
		nameLine,
	}

	if errorLine != "" {
		parts = append(parts, errorLine)
	}

	parts = append(parts, "", help)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// renderSessionView renders the session list for a container.
func (m Model) renderSessionView() string {
	if m.sessionFormOpen {
		return m.renderSessionForm()
	}

	containerName := ""
	if m.selectedContainer != nil {
		containerName = m.selectedContainer.Name
	}

	title := m.styles.TitleStyle().Render("Sessions")
	subtitle := m.styles.SubtitleStyle().Render(fmt.Sprintf("Container: %s", containerName))

	var content string
	if m.selectedContainer == nil || len(m.selectedContainer.Sessions) == 0 {
		content = m.styles.InfoStyle().Render("No sessions. Press 't' to create one.")
	} else {
		var lines []string
		for i, session := range m.selectedContainer.Sessions {
			indicator := "  "
			if i == m.selectedSessionIdx {
				indicator = m.styles.AccentStyle().Render("▸ ")
			}

			status := ""
			if session.Attached {
				status = m.styles.AccentStyle().Render(" (attached)")
			}

			line := fmt.Sprintf("%s%s%s", indicator, session.Name, status)
			if i == m.selectedSessionIdx {
				line = m.styles.AccentStyle().Render(fmt.Sprintf("▸ %s%s", session.Name, status))
			}
			lines = append(lines, line)
		}
		content = lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	// Show attach command for selected session
	var attachCmd string
	if cmd := m.AttachCommand(); cmd != "" {
		attachCmd = m.styles.InfoStyle().Render(fmt.Sprintf("Attach: %s", cmd))
	}

	// Error display
	var errorDisplay string
	if m.err != nil {
		errorDisplay = m.styles.ErrorStyle().Render(fmt.Sprintf("Error: %v", m.err))
	}

	help := m.styles.HelpStyle().Render("t: create session • k: kill session • ↑↓: navigate • Esc: back • q: quit")

	parts := []string{
		title,
		subtitle,
		"",
		content,
	}

	if attachCmd != "" {
		parts = append(parts, "", attachCmd)
	}

	if errorDisplay != "" {
		parts = append(parts, "", errorDisplay)
	}

	parts = append(parts, "", help)

	view := lipgloss.JoinVertical(lipgloss.Left, parts...)
	boxed := m.styles.BoxStyle().Render(view)

	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			boxed,
		)
	}

	return boxed
}

// renderSessionForm renders the session creation form as a left-justified input area.
func (m Model) renderSessionForm() string {
	containerName := ""
	if m.selectedContainer != nil {
		containerName = m.selectedContainer.Name
	}

	// Header line
	header := m.styles.TitleStyle().Render("Create Session") + "  " +
		m.styles.SubtitleStyle().Render(fmt.Sprintf("in %s", containerName))

	// Input line with label and value
	label := m.styles.AccentStyle().Render("Session Name: ")
	value := m.sessionFormName + "_" // cursor

	// Help line
	help := m.styles.HelpStyle().Render("Enter: create • Esc: cancel")

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		label+value,
		"",
		help,
	)
}

// renderStatusBar renders the status bar with operation feedback and help.
func (m Model) renderStatusBar(width int) string {
	var statusIcon string
	var messageStyle lipgloss.Style

	switch m.statusLevel {
	case StatusLoading:
		statusIcon = m.statusSpinner.View()
		messageStyle = m.styles.InfoStatusStyle()
	case StatusSuccess:
		statusIcon = m.styles.SuccessStyle().Render("✓")
		messageStyle = m.styles.SuccessStyle()
	case StatusError:
		statusIcon = m.styles.ErrorStyle().Render("✗")
		messageStyle = m.styles.ErrorStyle()
	default: // StatusInfo
		statusIcon = ""
		messageStyle = m.styles.InfoStatusStyle()
	}

	// Build status message
	var statusText string
	if statusIcon != "" {
		statusText = statusIcon + " " + messageStyle.Render(m.statusMessage)
	} else if m.statusMessage != "" {
		statusText = messageStyle.Render(m.statusMessage)
	}

	// Add error hint if in error state
	if m.statusLevel == StatusError && m.err != nil {
		statusText += m.styles.HelpStyle().Render(" (esc to clear)")
	}

	// Build help text
	help := m.renderContextualHelp()

	// Calculate spacing
	statusWidth := lipgloss.Width(statusText)
	helpWidth := lipgloss.Width(help)
	spacerWidth := width - statusWidth - helpWidth - 2 // 2 for padding

	if spacerWidth < 1 {
		spacerWidth = 1
	}

	spacer := strings.Repeat(" ", spacerWidth)

	return lipgloss.JoinHorizontal(lipgloss.Bottom,
		statusText,
		spacer,
		help,
	)
}

// renderContextualHelp returns help text based on current state and panel focus.
func (m Model) renderContextualHelp() string {
	var help string
	switch m.panelFocus {
	case FocusDetail:
		help = "tab: next panel • esc: tree • l: logs • q: quit"
	case FocusLogs:
		help = "↑/↓: scroll • g/G: top/bottom • tab: next panel • esc: tree • q: quit"
	default: // FocusTree
		if m.detailPanelOpen {
			help = "←/esc: close detail • ↑/↓: navigate • tab: next panel • l: logs • q: quit"
		} else if len(m.treeItems) > 0 {
			help = "↑/↓: navigate • enter: expand • →: details • c: create • s/x/d: start/stop/destroy • t: session • tab: next panel • l: logs • q: quit"
		} else {
			help = "c: create container • l: logs • q: quit"
		}
	}
	return m.styles.HelpStyle().Render(help)
}

// renderLogEntry formats a single log entry for display.
func (m Model) renderLogEntry(entry logging.LogEntry) string {
	// Timestamp
	ts := m.styles.LogTimestampStyle().Render(entry.Timestamp.Format("15:04:05"))

	// Level badge
	var level string
	switch entry.Level {
	case "DEBUG":
		level = m.styles.LogDebugStyle().Render("DEBUG")
	case "INFO":
		level = m.styles.LogInfoStyle().Render("INFO")
	case "WARN":
		level = m.styles.LogWarnStyle().Render("WARN")
	case "ERROR":
		level = m.styles.LogErrorStyle().Render("ERROR")
	default:
		level = m.styles.LogInfoStyle().Render(entry.Level)
	}

	// Scope
	scope := m.styles.LogScopeStyle().Render("[" + entry.Scope + "]")

	// Message
	message := entry.Message

	return fmt.Sprintf("%s %s %s %s", ts, level, scope, message)
}

// renderLogPanel renders the log panel content.
func (m Model) renderLogPanel(layout Layout) string {
	// Header with focus indicator
	filterInfo := "all logs"
	if m.logFilterLabel != "" {
		filterInfo = m.logFilterLabel
	}
	headerStyle := m.styles.PanelHeaderUnfocusedStyle()
	if m.panelFocus == FocusLogs {
		headerStyle = m.styles.PanelHeaderFocusedStyle()
	}
	header := headerStyle.Width(layout.Logs.Width).Render(fmt.Sprintf(" Logs (%s)", filterInfo))

	// Build log content
	entries := m.filteredLogEntries()
	var lines []string
	for _, entry := range entries {
		lines = append(lines, m.renderLogEntry(entry))
	}

	if len(lines) == 0 {
		lines = []string{m.styles.InfoStyle().Render("No log entries")}
	}

	content := strings.Join(lines, "\n")

	// Use viewport if ready, otherwise render directly
	if m.logReady {
		return lipgloss.JoinVertical(lipgloss.Left,
			header,
			m.logViewport.View(),
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		lipgloss.NewStyle().
			Width(layout.Logs.Width).
			Height(layout.Logs.Height-1).
			Render(content),
	)
}

// renderTree renders the tree view with containers and their sessions.
func (m Model) renderTree(layout Layout) string {
	headerStyle := m.styles.PanelHeaderUnfocusedStyle()
	if m.panelFocus == FocusTree {
		headerStyle = m.styles.PanelHeaderFocusedStyle()
	}
	header := headerStyle.Width(layout.Tree.Width).Render(" Containers")

	if len(m.treeItems) == 0 {
		body := lipgloss.NewStyle().
			Width(layout.Tree.Width).
			Height(layout.Tree.Height - 1).
			Padding(1).
			Render(m.styles.InfoStyle().Render("No containers. Press 'c' to create one."))
		return lipgloss.JoinVertical(lipgloss.Left, header, body)
	}

	var lines []string
	for i, item := range m.treeItems {
		isSelected := i == m.selectedIdx
		line := m.renderTreeItem(item, isSelected)
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")

	body := lipgloss.NewStyle().
		Width(layout.Tree.Width).
		Height(layout.Tree.Height - 1).
		Render(content)

	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

// renderTreeItem renders a single tree item (container or session).
func (m Model) renderTreeItem(item TreeItem, selected bool) string {
	cursor := "  "
	if selected {
		cursor = "> "
	}

	if item.Type == TreeItemContainer {
		return m.renderContainerTreeItem(item, cursor)
	}
	return m.renderSessionTreeItem(item, cursor)
}

// renderContainerTreeItem renders a container in the tree.
func (m Model) renderContainerTreeItem(item TreeItem, cursor string) string {
	// Find the container to get its details
	var c *container.Container
	for _, listItem := range m.containerList.Items() {
		if ci, ok := listItem.(containerItem); ok {
			if ci.container.ID == item.ContainerID {
				c = ci.container
				break
			}
		}
	}

	if c == nil {
		return cursor + "▸ (unknown container)"
	}

	// Expand/collapse indicator
	indicator := "▸"
	if item.Expanded {
		indicator = "▾"
	}

	// State indicator
	var stateIcon string
	switch c.State {
	case container.StateRunning:
		stateIcon = m.styles.SuccessStyle().Render("●")
	case container.StateStopped:
		stateIcon = m.styles.InfoStyle().Render("○")
	default:
		stateIcon = m.styles.InfoStyle().Render("◌")
	}

	name := c.Name
	state := string(c.State)

	return fmt.Sprintf("%s%s %s %s [%s]", cursor, indicator, stateIcon, name, state)
}

// renderSessionTreeItem renders a session in the tree (indented under container).
func (m Model) renderSessionTreeItem(item TreeItem, cursor string) string {
	// Find the session
	var sess *container.Session
	for _, listItem := range m.containerList.Items() {
		if ci, ok := listItem.(containerItem); ok {
			if ci.container.ID == item.ContainerID {
				for i := range ci.container.Sessions {
					if ci.container.Sessions[i].Name == item.SessionName {
						sess = &ci.container.Sessions[i]
						break
					}
				}
				break
			}
		}
	}

	if sess == nil {
		return cursor + "    └─ (unknown session)"
	}

	// Tree connector
	connector := "├─"
	// TODO: Could track if this is the last session to use "└─"

	// Attached indicator
	attachedIndicator := ""
	if sess.Attached {
		attachedIndicator = " (attached)"
	}

	return fmt.Sprintf("%s    %s %s%s", cursor, connector, sess.Name, attachedIndicator)
}

// renderDetailPanel renders the detail panel for the selected item.
func (m Model) renderDetailPanel(layout Layout) string {
	if layout.Detail.Width == 0 {
		return ""
	}

	// Panel header with focus indicator
	headerStyle := m.styles.PanelHeaderUnfocusedStyle()
	if m.panelFocus == FocusDetail {
		headerStyle = m.styles.PanelHeaderFocusedStyle()
	}
	header := headerStyle.Width(layout.Detail.Width).Render(" Details")

	// Body styling (keep left border, subtract 1 for header)
	panelStyle := lipgloss.NewStyle().
		Width(layout.Detail.Width-2).
		Height(layout.Detail.Height-1).
		Padding(1).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color(m.styles.flavor.Surface1().Hex))

	// Check if we have a selection
	if m.selectedIdx < 0 || m.selectedIdx >= len(m.treeItems) || m.selectedContainer == nil {
		return lipgloss.JoinVertical(lipgloss.Left,
			header,
			panelStyle.Render(m.styles.InfoStyle().Render("Select an item to view details")),
		)
	}

	item := m.treeItems[m.selectedIdx]

	var content string
	if item.Type == TreeItemContainer {
		content = m.renderContainerDetailContent()
	} else {
		content = m.renderSessionDetailContent()
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, panelStyle.Render(content))
}

// renderContainerDetailContent renders detail content for a container.
func (m Model) renderContainerDetailContent() string {
	if m.selectedContainer == nil {
		return "No container selected"
	}

	c := m.selectedContainer

	// Build info lines (panel header replaces TitleStyle header)
	lines := []string{
		fmt.Sprintf("Name:     %s", c.Name),
		fmt.Sprintf("ID:       %s", c.ID),
		fmt.Sprintf("State:    %s", string(c.State)),
		fmt.Sprintf("Template: %s", c.Template),
		fmt.Sprintf("Project:  %s", c.ProjectPath),
		fmt.Sprintf("Sessions: %d", len(c.Sessions)),
	}

	// List sessions if any
	if len(c.Sessions) > 0 {
		lines = append(lines, "", "Sessions:")
		for _, sess := range c.Sessions {
			attached := ""
			if sess.Attached {
				attached = " (attached)"
			}
			lines = append(lines, fmt.Sprintf("  • %s%s", sess.Name, attached))
		}
	}

	return strings.Join(lines, "\n")
}

// renderSessionDetailContent renders detail content for a session.
func (m Model) renderSessionDetailContent() string {
	if m.selectedContainer == nil {
		return "No session selected"
	}

	sess := m.SelectedSession()
	if sess == nil {
		return "No session selected"
	}

	// Attached status
	attachedStr := "No"
	if sess.Attached {
		attachedStr = "Yes"
	}

	// Build info lines (panel header replaces TitleStyle header)
	lines := []string{
		fmt.Sprintf("Name:      %s", sess.Name),
		fmt.Sprintf("Container: %s", m.selectedContainer.Name),
		fmt.Sprintf("Windows:   %d", sess.Windows),
		fmt.Sprintf("Attached:  %s", attachedStr),
	}

	// Add attach command hint
	lines = append(lines, "", "To attach:")
	lines = append(lines, fmt.Sprintf("  %s", m.AttachCommand()))

	return strings.Join(lines, "\n")
}

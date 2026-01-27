package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// View renders the TUI.
func (m Model) View() string {
	// If form is open, render the form instead
	if m.formOpen {
		return m.renderCreateForm()
	}

	// If session view is open, render sessions
	if m.sessionViewOpen {
		return m.renderSessionView()
	}

	title := m.styles.TitleStyle().Render("devagent")
	subtitle := m.styles.SubtitleStyle().Render("Development Agent Orchestrator")

	themeInfo := m.styles.InfoStyle().Render(
		fmt.Sprintf("Theme: %s", m.styles.AccentStyle().Render(m.themeName)),
	)

	var content string
	if len(m.containerList.Items()) == 0 {
		content = m.renderEmptyState()
	} else {
		content = m.containerList.View()
	}

	// Error display
	var errorDisplay string
	if m.err != nil {
		errorDisplay = m.styles.ErrorStyle().Render(fmt.Sprintf("Error: %v", m.err))
	}

	help := m.styles.HelpStyle().Render("c: create • s: start • x: stop • d: destroy • r: refresh • q: quit")

	parts := []string{
		title,
		subtitle,
		themeInfo,
		"",
		content,
	}

	if errorDisplay != "" {
		parts = append(parts, "", errorDisplay)
	}

	parts = append(parts, help)

	view := lipgloss.JoinVertical(lipgloss.Left, parts...)

	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			view,
		)
	}

	return view
}

// renderEmptyState renders the placeholder when no containers exist.
func (m Model) renderEmptyState() string {
	return m.styles.BoxStyle().Render(
		lipgloss.JoinVertical(
			lipgloss.Center,
			"No containers running",
			"",
			m.styles.InfoStyle().Render("Press 'c' to create a new container"),
		),
	)
}

// renderCreateForm renders the container creation form.
func (m Model) renderCreateForm() string {
	title := m.styles.TitleStyle().Render("Create Container")

	// Template selection
	templateLabel := "Template"
	if m.formFocusedField == FieldTemplate {
		templateLabel = m.styles.AccentStyle().Render("▸ Template")
	}

	var templateList string
	for i, tmpl := range m.templates {
		indicator := "  "
		if i == m.formTemplateIdx {
			indicator = "● "
		}
		line := fmt.Sprintf("%s%s - %s", indicator, tmpl.Name, tmpl.Description)
		if i == m.formTemplateIdx {
			line = m.styles.AccentStyle().Render(line)
		}
		if i > 0 {
			templateList += "\n"
		}
		templateList += line
	}
	if len(m.templates) == 0 {
		templateList = m.styles.ErrorStyle().Render("No templates available")
	}

	// Project path input
	projectPathLabel := "Project Path"
	if m.formFocusedField == FieldProjectPath {
		projectPathLabel = m.styles.AccentStyle().Render("▸ Project Path")
	}
	projectPathValue := m.formProjectPath
	if projectPathValue == "" {
		projectPathValue = m.styles.SubtitleStyle().Render("(enter path)")
	}
	if m.formFocusedField == FieldProjectPath {
		projectPathValue += "_" // cursor
	}

	// Container name input
	nameLabel := "Name (optional)"
	if m.formFocusedField == FieldContainerName {
		nameLabel = m.styles.AccentStyle().Render("▸ Name (optional)")
	}
	nameValue := m.formContainerName
	if nameValue == "" {
		nameValue = m.styles.SubtitleStyle().Render("(auto-generated)")
	}
	if m.formFocusedField == FieldContainerName {
		nameValue += "_" // cursor
	}

	// Error display
	var errorDisplay string
	if m.formError != "" {
		errorDisplay = m.styles.ErrorStyle().Render(fmt.Sprintf("Error: %s", m.formError))
	}

	// Help text
	help := m.styles.HelpStyle().Render("Tab: next field • ↑↓: select template • Enter: create • Esc: cancel")

	parts := []string{
		title,
		"",
		templateLabel,
		templateList,
		"",
		projectPathLabel,
		projectPathValue,
		"",
		nameLabel,
		nameValue,
	}

	if errorDisplay != "" {
		parts = append(parts, "", errorDisplay)
	}

	parts = append(parts, "", help)

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	boxed := m.styles.BoxStyle().Render(content)

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

// renderSessionForm renders the session creation form.
func (m Model) renderSessionForm() string {
	title := m.styles.TitleStyle().Render("Create Session")

	containerName := ""
	if m.selectedContainer != nil {
		containerName = m.selectedContainer.Name
	}
	subtitle := m.styles.SubtitleStyle().Render(fmt.Sprintf("Container: %s", containerName))

	nameLabel := m.styles.AccentStyle().Render("▸ Session Name")
	nameValue := m.sessionFormName
	if nameValue == "" {
		nameValue = m.styles.SubtitleStyle().Render("(enter name)")
	}
	nameValue += "_" // cursor

	help := m.styles.HelpStyle().Render("Enter: create • Esc: cancel")

	parts := []string{
		title,
		subtitle,
		"",
		nameLabel,
		nameValue,
		"",
		help,
	}

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	boxed := m.styles.BoxStyle().Render(content)

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

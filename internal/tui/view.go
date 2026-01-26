package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// View renders the TUI.
func (m Model) View() string {
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

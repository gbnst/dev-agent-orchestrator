package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	title := m.styles.TitleStyle().Render("devagent")
	subtitle := m.styles.SubtitleStyle().Render("Development Agent Orchestrator")

	themeInfo := m.styles.InfoStyle().Render(
		fmt.Sprintf("Theme: %s", m.styles.AccentStyle().Render(m.themeName)),
	)

	placeholder := m.styles.BoxStyle().Render("No containers running")

	help := m.styles.HelpStyle().Render("q: quit")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		subtitle,
		themeInfo,
		"",
		placeholder,
		help,
	)

	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			content,
		)
	}

	return content
}

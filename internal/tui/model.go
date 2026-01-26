package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	width     int
	height    int
	themeName string
	styles    *Styles
}

func NewModel(themeName string) Model {
	return Model{
		themeName: themeName,
		styles:    NewStyles(themeName),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

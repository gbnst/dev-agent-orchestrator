package tui

import (
	catppuccin "github.com/catppuccin/go"
	"github.com/charmbracelet/lipgloss"
)

type Styles struct {
	flavor catppuccin.Flavor
}

func NewStyles(themeName string) *Styles {
	flavor := flavorFromName(themeName)
	return &Styles{flavor: flavor}
}

func flavorFromName(name string) catppuccin.Flavor {
	switch name {
	case "latte":
		return catppuccin.Latte
	case "frappe":
		return catppuccin.Frappe
	case "macchiato":
		return catppuccin.Macchiato
	case "mocha":
		return catppuccin.Mocha
	default:
		return catppuccin.Mocha
	}
}

func (s *Styles) TitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(s.flavor.Mauve().Hex)).
		MarginBottom(1)
}

func (s *Styles) SubtitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(s.flavor.Subtext0().Hex)).
		MarginBottom(1)
}

func (s *Styles) HelpStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(s.flavor.Overlay0().Hex)).
		MarginTop(1)
}

func (s *Styles) BoxStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(s.flavor.Surface1().Hex)).
		Padding(1, 2)
}

func (s *Styles) InfoStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(s.flavor.Text().Hex))
}

func (s *Styles) AccentStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(s.flavor.Teal().Hex))
}

func (s *Styles) ErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(s.flavor.Red().Hex)).
		Bold(true)
}

// ActiveTabStyle returns the style for the currently selected tab.
func (s *Styles) ActiveTabStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(s.flavor.Mauve().Hex)).
		Padding(0, 2)
}

// InactiveTabStyle returns the style for non-selected tabs.
func (s *Styles) InactiveTabStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(s.flavor.Overlay0().Hex)).
		Padding(0, 2)
}

// TabGapFill returns the character used to fill the tab bar gap.
func (s *Styles) TabGapFill() string {
	return "─"
}

// TabGapStyle returns the style for the tab bar gap fill.
func (s *Styles) TabGapStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(s.flavor.Surface1().Hex))
}

// SuccessStyle returns the style for success messages.
func (s *Styles) SuccessStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(s.flavor.Green().Hex))
}

// InfoStatusStyle returns the style for info status messages.
func (s *Styles) InfoStatusStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(s.flavor.Text().Hex))
}

// StatusBarStyle returns the style for the status bar container.
func (s *Styles) StatusBarStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(s.flavor.Subtext0().Hex))
}

// pattern: Imperative Shell

package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"devagent/internal/container"
)

// containerItem wraps a container for display in a list.
type containerItem struct {
	container *container.Container
}

// Title returns the container name for display.
func (i containerItem) Title() string {
	return i.container.Name
}

// Description returns container details for display.
func (i containerItem) Description() string {
	stateStr := string(i.container.State)
	if i.container.ProjectPath != "" {
		return fmt.Sprintf("%s | %s | %s", i.container.ID[:12], stateStr, i.container.ProjectPath)
	}
	return fmt.Sprintf("%s | %s", i.container.ID[:12], stateStr)
}

// FilterValue returns the value to filter on.
func (i containerItem) FilterValue() string {
	return i.container.Name
}

// containerDelegate handles rendering of container items in a list.
type containerDelegate struct {
	styles       *Styles
	spinnerFrame string
	pendingOps   map[string]string
}

// newContainerDelegate creates a new container delegate with the given styles.
func newContainerDelegate(styles *Styles) containerDelegate {
	return containerDelegate{
		styles:     styles,
		pendingOps: make(map[string]string),
	}
}

// WithSpinnerState returns a delegate with updated spinner state.
func (d containerDelegate) WithSpinnerState(spinnerFrame string, pendingOps map[string]string) containerDelegate {
	d.spinnerFrame = spinnerFrame
	d.pendingOps = pendingOps
	return d
}

// Height returns the height of a single item.
func (d containerDelegate) Height() int {
	return 2
}

// Spacing returns the spacing between items.
func (d containerDelegate) Spacing() int {
	return 1
}

// Update handles item-specific updates.
func (d containerDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

// Render renders a single container item.
func (d containerDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ci, ok := item.(containerItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()

	// Title style
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(d.styles.flavor.Text().Hex))

	// Description style
	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(d.styles.flavor.Subtext0().Hex))

	// State color based on status
	var stateColor lipgloss.Color
	switch ci.container.State {
	case container.StateRunning:
		stateColor = lipgloss.Color(d.styles.flavor.Green().Hex)
	case container.StateStopped:
		stateColor = lipgloss.Color(d.styles.flavor.Red().Hex)
	default:
		stateColor = lipgloss.Color(d.styles.flavor.Yellow().Hex)
	}

	if isSelected {
		titleStyle = titleStyle.
			Bold(true).
			Foreground(lipgloss.Color(d.styles.flavor.Mauve().Hex))
		descStyle = descStyle.
			Foreground(lipgloss.Color(d.styles.flavor.Overlay0().Hex))
	}

	// Render indicator
	indicator := "  "
	if isSelected {
		indicator = lipgloss.NewStyle().
			Foreground(lipgloss.Color(d.styles.flavor.Mauve().Hex)).
			Render("▸ ")
	}

	// State indicator with spinner support
	var stateIndicator string
	if _, isPending := d.pendingOps[ci.container.ID]; isPending && d.spinnerFrame != "" {
		// Show spinner for pending operations
		stateIndicator = lipgloss.NewStyle().
			Foreground(lipgloss.Color(d.styles.flavor.Teal().Hex)).
			Render(d.spinnerFrame)
	} else {
		// Show state bullet
		stateIndicator = lipgloss.NewStyle().
			Foreground(stateColor).
			Render("●")
	}

	title := titleStyle.Render(ci.container.Name)
	desc := descStyle.Render(ci.Description())

	_, _ = fmt.Fprintf(w, "%s%s %s\n%s%s", indicator, stateIndicator, title, "    ", desc)
}

// toListItems converts containers to list items.
func toListItems(containers []*container.Container) []list.Item {
	items := make([]list.Item, len(containers))
	for i, c := range containers {
		items[i] = containerItem{container: c}
	}
	return items
}

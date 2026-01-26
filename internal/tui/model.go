package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/config"
	"devagent/internal/container"
)

// Model represents the TUI application state.
type Model struct {
	width     int
	height    int
	themeName string
	styles    *Styles

	cfg           *config.Config
	templates     []config.Template
	manager       *container.Manager
	containerList list.Model

	err error
}

// NewModel creates a new TUI model with the given configuration.
func NewModel(cfg *config.Config) Model {
	templates, _ := config.LoadTemplates()

	mgr := container.NewManager(cfg, templates)

	// Create container list
	delegate := newContainerDelegate(NewStyles(cfg.Theme))
	containerList := list.New([]list.Item{}, delegate, 0, 0)
	containerList.SetShowTitle(false)
	containerList.SetShowStatusBar(false)
	containerList.SetFilteringEnabled(false)
	containerList.SetShowHelp(false)

	return Model{
		themeName:     cfg.Theme,
		styles:        NewStyles(cfg.Theme),
		cfg:           cfg,
		templates:     templates,
		manager:       mgr,
		containerList: containerList,
	}
}

// Init returns the initial command to run.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.refreshContainers(),
		m.tick(),
	)
}

// refreshContainers returns a command to refresh the container list.
func (m Model) refreshContainers() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := m.manager.Refresh(ctx); err != nil {
			return containerErrorMsg{err: err}
		}

		return containersRefreshedMsg{containers: m.manager.List()}
	}
}

// tick returns a command for periodic refresh.
func (m Model) tick() tea.Cmd {
	return tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
		return tickMsg{time: t}
	})
}

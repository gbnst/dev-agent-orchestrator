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

	// Form state for container creation
	formOpen          bool
	formTemplateIdx   int
	formProjectPath   string
	formContainerName string
	formFocusedField  FormField
	formError         string

	// Session view state
	sessionViewOpen     bool
	selectedContainer   *container.Container
	selectedSessionIdx  int

	// Session creation form state
	sessionFormOpen bool
	sessionFormName string

	err error
}

// NewModel creates a new TUI model with the given configuration.
func NewModel(cfg *config.Config) Model {
	templates, _ := config.LoadTemplates()
	return NewModelWithTemplates(cfg, templates)
}

// NewModelWithTemplates creates a new TUI model with explicit templates (for testing).
func NewModelWithTemplates(cfg *config.Config, templates []config.Template) Model {
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

// sessionsRefreshedMsg is sent when session list is updated.
type sessionsRefreshedMsg struct {
	containerID string
	sessions    []container.Session
}

// refreshSessions returns a command to refresh sessions for the selected container.
func (m Model) refreshSessions() tea.Cmd {
	if m.selectedContainer == nil {
		return nil
	}
	containerID := m.selectedContainer.ID

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		sessions, err := m.manager.ListSessions(ctx, containerID)
		if err != nil {
			return containerErrorMsg{err: err}
		}

		return sessionsRefreshedMsg{containerID: containerID, sessions: sessions}
	}
}

// ContainerCount returns the number of containers in the list.
// This is an accessor for E2E testing.
func (m Model) ContainerCount() int {
	return len(m.containerList.Items())
}

// GetContainerByName returns a container by name if it exists.
// This is an accessor for E2E testing.
func (m Model) GetContainerByName(name string) (*container.Container, bool) {
	for _, item := range m.containerList.Items() {
		if ci, ok := item.(containerItem); ok {
			if ci.container.Name == name {
				return ci.container, true
			}
		}
	}
	return nil, false
}

// Manager returns the container manager.
// This is an accessor for E2E testing.
func (m Model) Manager() *container.Manager {
	return m.manager
}

// FormOpen returns whether the create form is open.
// This is an accessor for E2E testing.
func (m Model) FormOpen() bool {
	return m.formOpen
}

// IsSessionViewOpen returns whether the session view is open.
func (m Model) IsSessionViewOpen() bool {
	return m.sessionViewOpen
}

// VisibleSessionCount returns the number of sessions in the selected container.
func (m Model) VisibleSessionCount() int {
	if m.selectedContainer == nil {
		return 0
	}
	return len(m.selectedContainer.Sessions)
}

// SelectedSession returns the currently selected session, or nil if none.
func (m Model) SelectedSession() *container.Session {
	if m.selectedContainer == nil || len(m.selectedContainer.Sessions) == 0 {
		return nil
	}
	if m.selectedSessionIdx >= len(m.selectedContainer.Sessions) {
		return nil
	}
	return &m.selectedContainer.Sessions[m.selectedSessionIdx]
}

// AttachCommand returns the command to attach to the selected session.
func (m Model) AttachCommand() string {
	session := m.SelectedSession()
	if session == nil {
		return ""
	}
	runtime := m.cfg.DetectedRuntime()
	return session.AttachCommand(runtime)
}

// openSessionView opens the session view for the selected container.
func (m *Model) openSessionView() {
	if item, ok := m.containerList.SelectedItem().(containerItem); ok {
		m.sessionViewOpen = true
		m.selectedContainer = item.container
		m.selectedSessionIdx = 0
	}
}

// closeSessionView closes the session view.
func (m *Model) closeSessionView() {
	m.sessionViewOpen = false
	m.selectedContainer = nil
	m.selectedSessionIdx = 0
	m.sessionFormOpen = false
	m.sessionFormName = ""
}

// IsSessionFormOpen returns whether the session creation form is open.
func (m Model) IsSessionFormOpen() bool {
	return m.sessionFormOpen
}

// SessionFormName returns the current session form name value.
func (m Model) SessionFormName() string {
	return m.sessionFormName
}

// openSessionForm opens the session creation form.
func (m *Model) openSessionForm() {
	m.sessionFormOpen = true
	m.sessionFormName = ""
}

// closeSessionForm closes the session creation form.
func (m *Model) closeSessionForm() {
	m.sessionFormOpen = false
	m.sessionFormName = ""
}

// pattern: Imperative Shell

package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"devagent/internal/config"
	"devagent/internal/container"
	"devagent/internal/logging"
)

// TreeItemType represents whether a tree item is a container or session.
type TreeItemType int

const (
	TreeItemContainer TreeItemType = iota
	TreeItemSession
)

// TreeItem represents a selectable item in the tree view.
type TreeItem struct {
	Type        TreeItemType
	ContainerID string
	SessionName string // empty for containers
	Expanded    bool   // only meaningful for containers
}

// IsContainer returns true if this is a container item.
func (t TreeItem) IsContainer() bool { return t.Type == TreeItemContainer }

// IsSession returns true if this is a session item.
func (t TreeItem) IsSession() bool { return t.Type == TreeItemSession }

// StatusLevel represents the current status type for the status bar.
type StatusLevel int

const (
	StatusInfo StatusLevel = iota
	StatusSuccess
	StatusError
	StatusLoading
)

// String returns the status level name.
func (s StatusLevel) String() string {
	switch s {
	case StatusInfo:
		return "info"
	case StatusSuccess:
		return "success"
	case StatusError:
		return "error"
	case StatusLoading:
		return "loading"
	default:
		return "info"
	}
}

// PanelFocus represents which panel currently has keyboard focus.
type PanelFocus int

const (
	FocusTree   PanelFocus = iota
	FocusDetail
	FocusLogs
)

// Model represents the TUI application state.
type Model struct {
	width     int
	height    int
	themeName string
	styles    *Styles

	cfg                *config.Config
	templates          []config.Template
	manager            *container.Manager
	containerList      list.Model
	containerDelegate  containerDelegate

	// Form state for container creation
	formOpen          bool
	formTemplateIdx   int
	formProjectPath   string
	formContainerName string
	formFocusedField  FormField
	formError         string

	// Session view state
	sessionViewOpen    bool
	selectedContainer  *container.Container
	selectedSessionIdx int

	// Session creation form state
	sessionFormOpen bool
	sessionFormName string

	// Log panel
	logPanelOpen bool

	// Tree view state
	treeItems          []TreeItem
	selectedIdx        int
	expandedContainers map[string]bool
	detailPanelOpen    bool
	panelFocus         PanelFocus

	// Status bar
	statusMessage string
	statusLevel   StatusLevel
	statusSpinner spinner.Model

	// Pending operations (containerID -> operation type)
	pendingOperations map[string]string

	// Log panel
	logEntries    []logging.LogEntry
	logViewport   viewport.Model
	logFilter      string
	logFilterLabel string
	logAutoScroll bool
	logReady      bool // viewport initialized
	logManager    *logging.Manager
	logger        *logging.ScopedLogger

	err error
}

// NewModel creates a new TUI model with the given configuration.
func NewModel(cfg *config.Config, logManager *logging.Manager) Model {
	templates, _ := config.LoadTemplates()
	return NewModelWithTemplates(cfg, templates, logManager)
}

// NewModelWithTemplates creates a new TUI model with explicit templates (for testing).
func NewModelWithTemplates(cfg *config.Config, templates []config.Template, logManager *logging.Manager) Model {
	// Create container manager with logger
	mgr := container.NewManagerWithConfigAndLogger(cfg, templates, logManager)

	// Create container list
	delegate := newContainerDelegate(NewStyles(cfg.Theme))
	containerList := list.New([]list.Item{}, delegate, 0, 0)
	containerList.SetShowTitle(false)
	containerList.SetShowStatusBar(false)
	containerList.SetFilteringEnabled(false)
	containerList.SetShowHelp(false)

	// Initialize status spinner
	styles := NewStyles(cfg.Theme)
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(styles.flavor.Teal().Hex))

	logger := logManager.For("tui")
	logger.Debug("TUI model initialized")

	m := Model{
		themeName:         cfg.Theme,
		styles:            styles,
		cfg:               cfg,
		templates:         templates,
		manager:           mgr,
		containerList:     containerList,
		containerDelegate: delegate,
		statusSpinner:     s,
		pendingOperations: make(map[string]string),
		logEntries:        make([]logging.LogEntry, 0, maxLogEntries),
		logAutoScroll:     true,
		logManager:        logManager,
		logger:            logger,
	}
	return m
}

// Init returns the initial command to run.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.refreshContainers(),
		m.tick(),
		m.consumeLogEntries(m.logManager),
	)
}

// refreshContainers returns a command to refresh the container list.
func (m Model) refreshContainers() tea.Cmd {
	return func() tea.Msg {
		m.logger.Debug("refreshing containers")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := m.manager.Refresh(ctx); err != nil {
			m.logger.Error("container refresh failed", "error", err)
			return containerErrorMsg{err: err}
		}

		containers := m.manager.List()
		m.logger.Debug("containers refreshed", "count", len(containers))
		return containersRefreshedMsg{containers: containers}
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
		m.logger.Debug("refreshing sessions", "containerID", containerID)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		sessions, err := m.manager.ListSessions(ctx, containerID)
		if err != nil {
			m.logger.Error("session refresh failed", "containerID", containerID, "error", err)
			return containerErrorMsg{err: err}
		}

		m.logger.Debug("sessions refreshed", "containerID", containerID, "count", len(sessions))
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
	if session == nil || m.selectedContainer == nil {
		return ""
	}
	// Use manager's runtime path to bypass shell aliases (e.g., alias docker=podman)
	runtimePath := m.manager.RuntimePath()
	// Use container name instead of ID since docker ps returns truncated IDs
	return fmt.Sprintf("%s exec -it %s tmux attach -t %s", runtimePath, m.selectedContainer.Name, session.Name)
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

// setLoading sets the status to loading with a spinner.
func (m *Model) setLoading(message string) tea.Cmd {
	m.statusLevel = StatusLoading
	m.statusMessage = message
	return m.statusSpinner.Tick
}

// setSuccess sets the status to success.
func (m *Model) setSuccess(message string) {
	m.statusLevel = StatusSuccess
	m.statusMessage = message
	m.err = nil
}

// setError sets the status to error.
func (m *Model) setError(message string, err error) {
	m.statusLevel = StatusError
	m.statusMessage = message
	m.err = err
	// Store current context for log filtering when L is pressed
	m.setLogFilterFromContext()
}

// clearStatus resets the status bar to default.
func (m *Model) clearStatus() {
	m.statusLevel = StatusInfo
	m.statusMessage = ""
	m.err = nil
}

// setPending marks a container as having a pending operation.
func (m *Model) setPending(containerID, operation string) {
	if m.pendingOperations == nil {
		m.pendingOperations = make(map[string]string)
	}
	m.pendingOperations[containerID] = operation
}

// clearPending removes a container from pending operations.
func (m *Model) clearPending(containerID string) {
	delete(m.pendingOperations, containerID)
}

// isPending returns true if the container has a pending operation.
func (m Model) isPending(containerID string) bool {
	_, ok := m.pendingOperations[containerID]
	return ok
}

// getPendingOperation returns the pending operation type for a container.
func (m Model) getPendingOperation(containerID string) string {
	return m.pendingOperations[containerID]
}

// Ring buffer constant
const maxLogEntries = 1000

// addLogEntry adds an entry to the ring buffer, dropping oldest if full.
func (m *Model) addLogEntry(entry logging.LogEntry) {
	m.logEntries = append(m.logEntries, entry)
	if len(m.logEntries) > maxLogEntries {
		// Drop oldest entries
		m.logEntries = m.logEntries[len(m.logEntries)-maxLogEntries:]
	}
}

// filteredLogEntries returns entries matching the current filter.
func (m Model) filteredLogEntries() []logging.LogEntry {
	if m.logFilter == "" {
		return m.logEntries
	}

	var filtered []logging.LogEntry
	for _, entry := range m.logEntries {
		if entry.MatchesScope(m.logFilter) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// setLogFilterFromContext sets the log filter based on current UI state.
// Filter scopes match the logger scopes used by the container and tmux
// packages: "container" for all container operations, "tmux" for sessions.
func (m *Model) setLogFilterFromContext() {
	if m.selectedContainer == nil {
		m.logFilter = ""
		m.logFilterLabel = ""
		return
	}

	// Check if we're on a session item in the tree
	if m.selectedIdx >= 0 && m.selectedIdx < len(m.treeItems) {
		item := m.treeItems[m.selectedIdx]
		if item.Type == TreeItemSession {
			if session := m.SelectedSession(); session != nil {
				m.logFilter = "tmux"
				m.logFilterLabel = fmt.Sprintf("%s > %s", m.selectedContainer.Name, session.Name)
				return
			}
		}
	}

	// Default to container filter
	m.logFilter = "container"
	m.logFilterLabel = m.selectedContainer.Name
}

// consumeLogEntries reads entries from the log manager channel.
// Call this to start/continue log consumption.
func (m Model) consumeLogEntries(logMgr interface{ Entries() <-chan logging.LogEntry }) tea.Cmd {
	return func() tea.Msg {
		// Batch read up to 50 entries
		var entries []logging.LogEntry
		for i := 0; i < 50; i++ {
			select {
			case entry, ok := <-logMgr.Entries():
				if !ok {
					// Channel closed
					return logEntriesMsg{entries: entries}
				}
				entries = append(entries, entry)
			default:
				// No more entries ready
				return logEntriesMsg{entries: entries}
			}
		}
		return logEntriesMsg{entries: entries}
	}
}

// updateLogViewportContent refreshes the viewport with current filtered entries.
// Uses renderLogEntry for consistent formatting across all log displays.
func (m *Model) updateLogViewportContent() {
	entries := m.filteredLogEntries()
	var lines []string
	for _, entry := range entries {
		// Use renderLogEntry for consistent formatting with view rendering
		lines = append(lines, m.renderLogEntry(entry))
	}

	content := strings.Join(lines, "\n")
	m.logViewport.SetContent(content)

	if m.logAutoScroll {
		m.logViewport.GotoBottom()
	}
}

// rebuildTreeItems rebuilds the flat list of visible tree items based on
// container expansion state. Call this after containers change or expansion toggles.
func (m *Model) rebuildTreeItems() {
	m.treeItems = nil

	for _, item := range m.containerList.Items() {
		ci, ok := item.(containerItem)
		if !ok {
			continue
		}
		c := ci.container

		expanded := m.expandedContainers[c.ID]
		m.treeItems = append(m.treeItems, TreeItem{
			Type:        TreeItemContainer,
			ContainerID: c.ID,
			Expanded:    expanded,
		})

		if expanded {
			for _, session := range c.Sessions {
				m.treeItems = append(m.treeItems, TreeItem{
					Type:        TreeItemSession,
					ContainerID: c.ID,
					SessionName: session.Name,
				})
			}
		}
	}
}

// toggleTreeExpand toggles expansion of a container in the tree view.
// If the selected item is a session, this does nothing.
func (m *Model) toggleTreeExpand() {
	if m.selectedIdx < 0 || m.selectedIdx >= len(m.treeItems) {
		return
	}
	item := m.treeItems[m.selectedIdx]
	if item.Type != TreeItemContainer {
		return
	}

	if m.expandedContainers == nil {
		m.expandedContainers = make(map[string]bool)
	}

	// Toggle expansion state
	m.expandedContainers[item.ContainerID] = !m.expandedContainers[item.ContainerID]
	m.rebuildTreeItems()
}

// syncSelectionFromTree updates selectedContainer and selectedSessionIdx
// based on the current tree selection (selectedIdx), and keeps the log
// filter in sync so it always matches the active display scope.
func (m *Model) syncSelectionFromTree() {
	if m.selectedIdx < 0 || m.selectedIdx >= len(m.treeItems) {
		m.selectedContainer = nil
		m.selectedSessionIdx = 0
		m.setLogFilterFromContext()
		return
	}

	item := m.treeItems[m.selectedIdx]

	// Find the container for this item
	for _, listItem := range m.containerList.Items() {
		if ci, ok := listItem.(containerItem); ok {
			if ci.container.ID == item.ContainerID {
				m.selectedContainer = ci.container

				// If it's a session, find the session index
				if item.Type == TreeItemSession {
					for i, sess := range ci.container.Sessions {
						if sess.Name == item.SessionName {
							m.selectedSessionIdx = i
							m.setLogFilterFromContext()
							return
						}
					}
				} else {
					m.selectedSessionIdx = 0
				}
				m.setLogFilterFromContext()
				return
			}
		}
	}
}

// moveTreeSelectionUp moves selection up in the tree.
func (m *Model) moveTreeSelectionUp() {
	if m.selectedIdx > 0 {
		m.selectedIdx--
		m.syncSelectionFromTree()
	}
}

// moveTreeSelectionDown moves selection down in the tree.
func (m *Model) moveTreeSelectionDown() {
	if m.selectedIdx < len(m.treeItems)-1 {
		m.selectedIdx++
		m.syncSelectionFromTree()
	}
}

// nextFocus returns the next panel focus, skipping panels that aren't open.
func (m *Model) nextFocus() PanelFocus {
	switch m.panelFocus {
	case FocusTree:
		if m.detailPanelOpen {
			return FocusDetail
		}
		if m.logPanelOpen && m.logReady {
			return FocusLogs
		}
		return FocusTree
	case FocusDetail:
		if m.logPanelOpen && m.logReady {
			return FocusLogs
		}
		return FocusTree
	case FocusLogs:
		return FocusTree
	default:
		return FocusTree
	}
}

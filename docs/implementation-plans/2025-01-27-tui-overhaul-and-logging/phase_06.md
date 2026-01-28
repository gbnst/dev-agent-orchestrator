# TUI Overhaul and Logging Infrastructure - Phase 6: Log Panel

**Goal:** Implement toggleable split-panel log viewer with scope-based filtering.

**Architecture:** L key toggles log panel. When open, layout splits 40% content / 60% logs. Log entries consumed from Phase 1 LogManager.Entries() channel via batched reads. Ring buffer stores last 1000 entries. Scope filter set based on current context (all, container, session). Viewport provides scrollable view with j/k navigation.

**Tech Stack:** Go 1.24+, Bubbletea, Bubbles viewport component

**Scope:** 7 phases from original design (this is phase 6 of 7)

**Codebase verified:** 2025-01-27

---

## Phase Overview

This phase creates the log panel that displays entries from Phase 1's logging infrastructure. We:
1. Add log state (entries buffer, viewport, filter) to Model
2. Create log consumption command that reads from LogManager channel
3. Implement renderLogPanel with scope filtering and level badges
4. Add L key toggle and integrate with layout split

**Dependencies:** Phase 1 (logging infrastructure), Phase 2 (layout system)

**Key integration points:**
- Phase 1: LogManager.Entries() returns `<-chan LogEntry`
- Phase 2: Layout.Logs region when logPanelOpen=true
- Phase 4: Error state triggers filter + auto-open

---

<!-- START_SUBCOMPONENT_A (tasks 1-3) -->
## Subcomponent A: Log State and Storage

<!-- START_TASK_1 -->
### Task 1: Add log state to Model

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model.go`

**Step 1: Write the test**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model_test.go`:

```go
func TestModel_LogEntries(t *testing.T) {
	m := newTestModel()

	// Initially empty
	if len(m.logEntries) != 0 {
		t.Error("logEntries should be empty initially")
	}

	// Add entries
	entry1 := logging.LogEntry{Message: "test1", Scope: "app"}
	entry2 := logging.LogEntry{Message: "test2", Scope: "container.abc"}

	m.addLogEntry(entry1)
	m.addLogEntry(entry2)

	if len(m.logEntries) != 2 {
		t.Errorf("logEntries length = %d, want 2", len(m.logEntries))
	}
}

func TestModel_LogEntriesRingBuffer(t *testing.T) {
	m := newTestModel()

	// Add more than max entries
	for i := 0; i < 1050; i++ {
		m.addLogEntry(logging.LogEntry{Message: fmt.Sprintf("entry %d", i), Scope: "app"})
	}

	// Should be capped at 1000
	if len(m.logEntries) > 1000 {
		t.Errorf("logEntries length = %d, should be capped at 1000", len(m.logEntries))
	}
}

func TestModel_FilteredLogEntries(t *testing.T) {
	m := newTestModel()

	m.addLogEntry(logging.LogEntry{Message: "app log", Scope: "app"})
	m.addLogEntry(logging.LogEntry{Message: "container log", Scope: "container.abc123"})
	m.addLogEntry(logging.LogEntry{Message: "session log", Scope: "session.abc123.dev"})

	// No filter = all entries
	m.logFilter = ""
	if len(m.filteredLogEntries()) != 3 {
		t.Errorf("no filter should return all entries, got %d", len(m.filteredLogEntries()))
	}

	// Container filter
	m.logFilter = "container.abc123"
	filtered := m.filteredLogEntries()
	if len(filtered) != 1 {
		t.Errorf("container filter should return 1 entry, got %d", len(filtered))
	}
}
```

Add import for logging package:
```go
import "devagent/internal/logging"
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestModel_Log
```

Expected: FAIL - logEntries undefined

**Step 3: Add log state to Model**

Add imports and fields to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model.go`:

```go
import (
	"github.com/charmbracelet/bubbles/viewport"
	"devagent/internal/logging"
)
```

Add fields to Model struct:

```go
	// Log panel
	logPanelOpen  bool
	logEntries    []logging.LogEntry
	logViewport   viewport.Model
	logFilter     string
	logAutoScroll bool
	logReady      bool // viewport initialized
```

Add helper methods:

```go
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
func (m *Model) setLogFilterFromContext() {
	switch {
	case m.selectedContainer != nil && m.currentTab == TabSessions:
		if session := m.SelectedSession(); session != nil {
			m.logFilter = fmt.Sprintf("session.%s.%s", m.selectedContainer.ID[:12], session.Name)
		} else {
			m.logFilter = fmt.Sprintf("container.%s", m.selectedContainer.ID[:12])
		}
	case m.selectedContainer != nil:
		m.logFilter = fmt.Sprintf("container.%s", m.selectedContainer.ID[:12])
	default:
		m.logFilter = ""
	}
}
```

Initialize in NewModelWithTemplates:

```go
	m.logEntries = make([]logging.LogEntry, 0, maxLogEntries)
	m.logAutoScroll = true
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestModel_Log
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/model.go internal/tui/model_test.go && git commit -m "feat(tui): add log panel state and ring buffer to Model"
```
<!-- END_TASK_1 -->

<!-- START_TASK_2 -->
### Task 2: Add log level badge styles

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/styles.go`

**Step 1: Write the test**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/styles_test.go`:

```go
func TestStyles_LogLevelBadges(t *testing.T) {
	styles := NewStyles("mocha")

	levels := []struct {
		level string
		fn    func() lipgloss.Style
	}{
		{"DEBUG", styles.LogDebugStyle},
		{"INFO", styles.LogInfoStyle},
		{"WARN", styles.LogWarnStyle},
		{"ERROR", styles.LogErrorStyle},
	}

	for _, l := range levels {
		t.Run(l.level, func(t *testing.T) {
			style := l.fn()
			// Should have foreground color
			if style.GetForeground() == lipgloss.NoColor{} {
				t.Errorf("%s style should have foreground color", l.level)
			}
		})
	}
}

func TestStyles_LogHeaderStyle(t *testing.T) {
	styles := NewStyles("mocha")
	style := styles.LogHeaderStyle()

	if style.GetForeground() == lipgloss.NoColor{} {
		t.Error("LogHeaderStyle should have foreground color")
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestStyles_Log
```

Expected: FAIL - LogDebugStyle undefined

**Step 3: Add log styles**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/styles.go`:

```go
// LogHeaderStyle returns the style for log panel header.
func (s *Styles) LogHeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(s.flavor.Subtext0().Hex)).
		Bold(true)
}

// LogDebugStyle returns the style for DEBUG level logs.
func (s *Styles) LogDebugStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(s.flavor.Overlay0().Hex))
}

// LogInfoStyle returns the style for INFO level logs.
func (s *Styles) LogInfoStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(s.flavor.Blue().Hex))
}

// LogWarnStyle returns the style for WARN level logs.
func (s *Styles) LogWarnStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(s.flavor.Yellow().Hex))
}

// LogErrorStyle returns the style for ERROR level logs.
func (s *Styles) LogErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(s.flavor.Red().Hex)).
		Bold(true)
}

// LogTimestampStyle returns the style for log timestamps.
func (s *Styles) LogTimestampStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(s.flavor.Overlay0().Hex))
}

// LogScopeStyle returns the style for log scope.
func (s *Styles) LogScopeStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(s.flavor.Teal().Hex))
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestStyles_Log
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/styles.go internal/tui/styles_test.go && git commit -m "feat(tui): add log level badge and header styles"
```
<!-- END_TASK_2 -->

<!-- START_TASK_3 -->
### Task 3: Add log entries message type

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update.go`

**Step 1: Add message type**

Add to the message types section at the top of update.go:

```go
// logEntriesMsg delivers log entries from the logging channel.
type logEntriesMsg struct {
	entries []logging.LogEntry
}
```

Add import:
```go
import "devagent/internal/logging"
```

**Step 2: Add log consumption command**

Add to model.go (or a new logs.go file):

```go
// consumeLogEntries reads entries from the log manager channel.
// Call this to start/continue log consumption.
func (m Model) consumeLogEntries(logMgr *logging.Manager) tea.Cmd {
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
```

**Step 3: Verify build**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go build ./...
```

Expected: Build succeeds

**Step 4: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/update.go internal/tui/model.go && git commit -m "feat(tui): add logEntriesMsg and consumption command"
```
<!-- END_TASK_3 -->
<!-- END_SUBCOMPONENT_A -->

<!-- START_SUBCOMPONENT_B (tasks 4-6) -->
## Subcomponent B: Log Panel Rendering

<!-- START_TASK_4 -->
### Task 4: Create renderLogPanel method

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/view.go`

**Step 1: Write the test**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/view_test.go`:

```go
func TestRenderLogEntry(t *testing.T) {
	m := newTestModel()

	entry := logging.LogEntry{
		Timestamp: time.Date(2025, 1, 27, 10, 30, 0, 0, time.UTC),
		Level:     "INFO",
		Scope:     "container.abc123",
		Message:   "container started",
	}

	result := m.renderLogEntry(entry)

	if !strings.Contains(result, "10:30:00") {
		t.Error("should contain timestamp")
	}
	if !strings.Contains(result, "INFO") {
		t.Error("should contain level")
	}
	if !strings.Contains(result, "container.abc123") {
		t.Error("should contain scope")
	}
	if !strings.Contains(result, "container started") {
		t.Error("should contain message")
	}
}
```

Add imports:
```go
import (
	"time"
	"devagent/internal/logging"
)
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestRenderLogEntry
```

Expected: FAIL - renderLogEntry undefined

**Step 3: Add log rendering methods**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/view.go`:

```go
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
	// Header
	filterInfo := "all logs"
	if m.logFilter != "" {
		filterInfo = "filtered: " + m.logFilter
	}
	header := m.styles.LogHeaderStyle().Render(fmt.Sprintf("Logs (%s)", filterInfo))

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
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestRenderLogEntry
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/view.go internal/tui/view_test.go && git commit -m "feat(tui): add log entry and log panel rendering"
```
<!-- END_TASK_4 -->

<!-- START_TASK_5 -->
### Task 5: Integrate log panel with View()

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/view.go`

**Step 1: Update View() to render log panel when open**

Find the part of View() that composes parts and add log panel rendering. After content, before status bar:

```go
	// Compose full layout
	parts := []string{header, tabs, content}

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
```

**Step 2: Run tests**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v
```

Expected: All tests pass

**Step 3: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/view.go && git commit -m "feat(tui): integrate log panel with main View()"
```
<!-- END_TASK_5 -->

<!-- START_TASK_6 -->
### Task 6: Initialize and update viewport

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update.go`

**Step 1: Handle viewport initialization in WindowSizeMsg**

Update the `tea.WindowSizeMsg` case to initialize the log viewport:

```go
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Use Layout for consistent height calculation
		layout := ComputeLayout(m.width, m.height, m.logPanelOpen)
		listHeight := layout.ContentListHeight()

		m.containerList.SetSize(m.width-4, listHeight)

		// Initialize or update log viewport
		if m.logPanelOpen {
			if !m.logReady {
				m.logViewport = viewport.New(layout.Logs.Width, layout.Logs.Height-1)
				m.logReady = true
			} else {
				m.logViewport.Width = layout.Logs.Width
				m.logViewport.Height = layout.Logs.Height - 1
			}
			m.updateLogViewportContent()
		}

		return m, nil
```

**Step 2: Add viewport content update method**

Add to model.go:

```go
// updateLogViewportContent refreshes the viewport with current filtered entries.
func (m *Model) updateLogViewportContent() {
	entries := m.filteredLogEntries()
	var lines []string
	for _, entry := range entries {
		lines = append(lines, m.renderLogEntry(entry))
	}

	content := strings.Join(lines, "\n")
	m.logViewport.SetContent(content)

	if m.logAutoScroll {
		m.logViewport.GotoBottom()
	}
}
```

Note: `renderLogEntry` is defined as a method on Model in view.go, so it's accessible from model.go since both are in the same package.

**Step 3: Handle logEntriesMsg**

Add handler in Update():

```go
	case logEntriesMsg:
		for _, entry := range msg.entries {
			m.addLogEntry(entry)
		}
		if m.logPanelOpen && m.logReady {
			m.updateLogViewportContent()
		}
		// Continue consuming logs (logManager added in Phase 7)
		if m.logManager != nil {
			return m, m.consumeLogEntries(m.logManager)
		}
		return m, nil
```

**Note:** The `logManager` field is added to Model in Phase 7. For Phase 6 to compile, add a placeholder field to Model:

```go
// Placeholder - will be properly wired in Phase 7
logManager interface{ Entries() <-chan logging.LogEntry }
```

This allows Phase 6 code to compile. Phase 7 will replace this with the actual `*logging.Manager` type.

**Step 4: Verify build**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go build ./...
```

Expected: Build succeeds

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/update.go internal/tui/model.go && git commit -m "feat(tui): initialize and update log viewport"
```
<!-- END_TASK_6 -->
<!-- END_SUBCOMPONENT_B -->

<!-- START_SUBCOMPONENT_C (tasks 7-9) -->
## Subcomponent C: Log Panel Toggle and Navigation

<!-- START_TASK_7 -->
### Task 7: Add L key toggle

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update.go`

**Step 1: Write the test**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update_test.go`:

```go
func TestLKey_TogglesLogPanel(t *testing.T) {
	m := newTestModel()
	m.logPanelOpen = false

	// Press L to open
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if !m.logPanelOpen {
		t.Error("logPanelOpen should be true after L")
	}

	// Press L again to close
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if m.logPanelOpen {
		t.Error("logPanelOpen should be false after second L")
	}
}

func TestLKey_SetsFilterFromContext(t *testing.T) {
	m := newTestModel()
	m.selectedContainer = &container.Container{ID: "abc123456789", Name: "test"}
	m.currentTab = TabSessions

	// Press L
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.logFilter == "" {
		t.Error("logFilter should be set from context")
	}
	if !strings.Contains(m.logFilter, "container.abc123456789") && !strings.Contains(m.logFilter, "abc123456789") {
		t.Errorf("logFilter = %q, should contain container ID", m.logFilter)
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestLKey
```

Expected: FAIL - L key not handled yet

**Step 3: Add L key handler**

Add to the key switch in Update():

```go
		case "l", "L":
			m.logPanelOpen = !m.logPanelOpen
			if m.logPanelOpen {
				m.setLogFilterFromContext()
				// Recalculate layout and initialize viewport if needed
				layout := ComputeLayout(m.width, m.height, m.logPanelOpen)
				if !m.logReady {
					m.logViewport = viewport.New(layout.Logs.Width, layout.Logs.Height-1)
					m.logReady = true
				}
				m.updateLogViewportContent()
			}
			// Recalculate list size for split layout
			layout := ComputeLayout(m.width, m.height, m.logPanelOpen)
			m.containerList.SetSize(m.width-4, layout.ContentListHeight())
			return m, nil
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestLKey
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/update.go internal/tui/update_test.go && git commit -m "feat(tui): add L key to toggle log panel"
```
<!-- END_TASK_7 -->

<!-- START_TASK_8 -->
### Task 8: Add log viewport navigation

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update.go`

**Step 1: Add viewport navigation when log panel is focused**

In the key handling section, add viewport navigation. The viewport has built-in key handling, but we need to delegate to it:

```go
		// Delegate to viewport when log panel is open and focused
		// For now, j/k scroll logs when panel is open
		case "j":
			if m.logPanelOpen && m.logReady {
				m.logViewport.LineDown(1)
				m.logAutoScroll = m.logViewport.AtBottom()
				return m, nil
			}
			// Fall through to container list navigation if not handled
		case "k":
			if m.logPanelOpen && m.logReady {
				m.logViewport.LineUp(1)
				m.logAutoScroll = false
				return m, nil
			}
			// Fall through to container list navigation
		case "g":
			if m.logPanelOpen && m.logReady {
				m.logViewport.GotoTop()
				m.logAutoScroll = false
				return m, nil
			}
		case "G":
			if m.logPanelOpen && m.logReady {
				m.logViewport.GotoBottom()
				m.logAutoScroll = true
				return m, nil
			}
```

**Step 2: Update viewport on viewport messages**

Add handler for viewport messages:

```go
	// Handle viewport updates
	case tea.MouseMsg:
		if m.logPanelOpen && m.logReady {
			var cmd tea.Cmd
			m.logViewport, cmd = m.logViewport.Update(msg)
			m.logAutoScroll = m.logViewport.AtBottom()
			return m, cmd
		}
```

**Step 3: Verify build**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go build ./...
```

Expected: Build succeeds

**Step 4: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/update.go && git commit -m "feat(tui): add log viewport navigation (j/k/g/G)"
```
<!-- END_TASK_8 -->

<!-- START_TASK_9 -->
### Task 9: Error state auto-opens filtered logs

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update.go`

**Step 1: Modify error handling to offer log panel**

Update the setError method or error handling to set a filter scope:

```go
// setError sets the status to error and stores the error scope for log filtering.
func (m *Model) setError(message string, err error) {
	m.statusLevel = StatusError
	m.statusMessage = message
	m.err = err
	// Store current context for log filtering when L is pressed
	m.setLogFilterFromContext()
}
```

The help text in status bar already shows "(L for full error log)" per Phase 4.

**Step 2: Run tests**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v
```

Expected: All tests pass

**Step 3: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/model.go && git commit -m "feat(tui): set log filter context on error for L key access"
```
<!-- END_TASK_9 -->
<!-- END_SUBCOMPONENT_C -->

<!-- START_TASK_10 -->
### Task 10: Run all tests and verify phase complete

**Step 1: Run all TUI tests**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v
```

Expected: All tests pass

**Step 2: Run full test suite**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./...
```

Expected: All existing tests still pass

**Step 3: Verify build**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go build ./...
```

Expected: Build succeeds
<!-- END_TASK_10 -->

---

## Phase Completion Checklist

- [ ] L key toggles log panel (40% main / 60% logs split)
- [ ] Logs filter based on current context (all, container, session)
- [ ] Ring buffer stores last 1000 entries
- [ ] New entries appear in real-time (live tail)
- [ ] Viewport scrollable with j/k keys
- [ ] g goes to top, G goes to bottom
- [ ] Auto-scroll follows tail when at bottom
- [ ] Error state + L opens logs filtered to error scope
- [ ] All existing tests still pass

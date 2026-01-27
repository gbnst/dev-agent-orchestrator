# TUI Overhaul and Logging Infrastructure - Phase 4: Status Bar

**Goal:** Add persistent status bar with operation feedback and contextual help.

**Architecture:** Status bar at bottom of screen shows operation progress (spinner), success (green checkmark), error (red X), or info. Errors display message plus hint. Contextual help aligned right. Status automatically clears after timeout or manual Esc.

**Tech Stack:** Go 1.24+, Bubbletea, Bubbles spinner component

**Scope:** 7 phases from original design (this is phase 4 of 7)

**Codebase verified:** 2025-01-27

---

## Phase Overview

This phase adds a proper status bar replacing the simple help text. We:
1. Add status state (message, level, spinner) to Model
2. Add operation tracking for showing spinner during actions
3. Create status bar rendering with spinner, icons, and help
4. Wire container actions to status updates
5. Add Esc to clear error state

**Dependencies:** Phase 2 (layout system)

**Current patterns from investigation:**
- Error stored in `m.err` at `internal/tui/model.go:43`
- `containerActionMsg` with action/id/err at `internal/tui/update.go:27-31`
- `ErrorStyle()` already exists at `internal/tui/styles.go:68-72`
- Catppuccin palette available for success/info colors

---

<!-- START_SUBCOMPONENT_A (tasks 1-3) -->
## Subcomponent A: Status State and Styles

<!-- START_TASK_1 -->
### Task 1: Add status types and state to Model

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model.go`

**Step 1: Write the test**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model_test.go`:

```go
func TestStatusLevel_String(t *testing.T) {
	tests := []struct {
		level StatusLevel
		want  string
	}{
		{StatusInfo, "info"},
		{StatusSuccess, "success"},
		{StatusError, "error"},
		{StatusLoading, "loading"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("StatusLevel.String() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestStatusLevel
```

Expected: FAIL - StatusLevel undefined

**Step 3: Add status types to model.go**

Add after the TabMode type in `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model.go`:

```go
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
```

Add these fields to the Model struct:

```go
	// Status bar
	statusMessage string
	statusLevel   StatusLevel
	statusSpinner spinner.Model
```

Add the import at the top of the file:
```go
import "github.com/charmbracelet/bubbles/spinner"
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestStatusLevel
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/model.go internal/tui/model_test.go && git commit -m "feat(tui): add StatusLevel type and status state to Model"
```
<!-- END_TASK_1 -->

<!-- START_TASK_2 -->
### Task 2: Initialize spinner in model

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model.go`

**Step 1: Update NewModel or NewModelWithTemplates**

Find the model initialization function and add spinner setup. Add after other field initialization:

```go
	// Initialize status spinner
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(styles.flavor.Teal().Hex))
	m.statusSpinner = s
```

You'll need to add the lipgloss import if not present:
```go
import "github.com/charmbracelet/lipgloss"
```

**Step 2: Verify build**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go build ./...
```

Expected: Build succeeds

**Step 3: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/model.go && git commit -m "feat(tui): initialize spinner in model"
```
<!-- END_TASK_2 -->

<!-- START_TASK_3 -->
### Task 3: Add status styles

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/styles.go`

**Step 1: Write the test**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/styles_test.go`:

```go
func TestStyles_StatusStyles(t *testing.T) {
	styles := NewStyles("mocha")

	// SuccessStyle should use green
	successStyle := styles.SuccessStyle()
	if successStyle.GetForeground() == lipgloss.NoColor{} {
		t.Error("SuccessStyle should have foreground color")
	}

	// InfoStatusStyle should exist
	infoStyle := styles.InfoStatusStyle()
	if infoStyle.GetForeground() == lipgloss.NoColor{} {
		t.Error("InfoStatusStyle should have foreground color")
	}

	// ErrorStyle already exists, just verify it works
	errorStyle := styles.ErrorStyle()
	if !errorStyle.GetBold() {
		t.Error("ErrorStyle should be bold")
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestStyles_Status
```

Expected: FAIL - SuccessStyle undefined

**Step 3: Add status styles**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/styles.go`:

```go
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
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestStyles_Status
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/styles.go internal/tui/styles_test.go && git commit -m "feat(tui): add success and info status styles"
```
<!-- END_TASK_3 -->
<!-- END_SUBCOMPONENT_A -->

<!-- START_SUBCOMPONENT_B (tasks 4-5) -->
## Subcomponent B: Status Bar Rendering

<!-- START_TASK_4 -->
### Task 4: Create renderStatusBar method

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/view.go`

**Step 1: Write the test**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/view_test.go`:

```go
func TestRenderStatusBar_Info(t *testing.T) {
	m := newTestModel()
	m.statusLevel = StatusInfo
	m.statusMessage = "Ready"

	result := m.renderStatusBar(80)

	if !strings.Contains(result, "Ready") {
		t.Error("status bar should contain message")
	}
}

func TestRenderStatusBar_Success(t *testing.T) {
	m := newTestModel()
	m.statusLevel = StatusSuccess
	m.statusMessage = "Container started"

	result := m.renderStatusBar(80)

	if !strings.Contains(result, "✓") {
		t.Error("success status should contain checkmark")
	}
	if !strings.Contains(result, "Container started") {
		t.Error("status bar should contain message")
	}
}

func TestRenderStatusBar_Error(t *testing.T) {
	m := newTestModel()
	m.statusLevel = StatusError
	m.statusMessage = "Failed to start"
	m.err = fmt.Errorf("connection refused")

	result := m.renderStatusBar(80)

	if !strings.Contains(result, "✗") {
		t.Error("error status should contain X mark")
	}
	if !strings.Contains(result, "Failed to start") {
		t.Error("status bar should contain message")
	}
}

func TestRenderStatusBar_Loading(t *testing.T) {
	m := newTestModel()
	m.statusLevel = StatusLoading
	m.statusMessage = "Starting container..."

	result := m.renderStatusBar(80)

	// Spinner renders differently each frame, just check message
	if !strings.Contains(result, "Starting container...") {
		t.Error("status bar should contain loading message")
	}
}
```

Add `import "fmt"` if not present.

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestRenderStatusBar
```

Expected: FAIL - renderStatusBar undefined

**Step 3: Add renderStatusBar method**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/view.go`:

```go
// renderStatusBar renders the status bar with operation feedback and help.
func (m Model) renderStatusBar(width int) string {
	var statusIcon string
	var messageStyle lipgloss.Style

	switch m.statusLevel {
	case StatusLoading:
		statusIcon = m.statusSpinner.View()
		messageStyle = m.styles.InfoStatusStyle()
	case StatusSuccess:
		statusIcon = m.styles.SuccessStyle().Render("✓")
		messageStyle = m.styles.SuccessStyle()
	case StatusError:
		statusIcon = m.styles.ErrorStyle().Render("✗")
		messageStyle = m.styles.ErrorStyle()
	default: // StatusInfo
		statusIcon = ""
		messageStyle = m.styles.InfoStatusStyle()
	}

	// Build status message
	var statusText string
	if statusIcon != "" {
		statusText = statusIcon + " " + messageStyle.Render(m.statusMessage)
	} else if m.statusMessage != "" {
		statusText = messageStyle.Render(m.statusMessage)
	}

	// Add error hint if in error state
	if m.statusLevel == StatusError && m.err != nil {
		statusText += m.styles.HelpStyle().Render(" (esc to clear)")
	}

	// Build help text
	help := m.renderContextualHelp()

	// Calculate spacing
	statusWidth := lipgloss.Width(statusText)
	helpWidth := lipgloss.Width(help)
	spacerWidth := width - statusWidth - helpWidth - 2 // 2 for padding

	if spacerWidth < 1 {
		spacerWidth = 1
	}

	spacer := strings.Repeat(" ", spacerWidth)

	return lipgloss.JoinHorizontal(lipgloss.Bottom,
		statusText,
		spacer,
		help,
	)
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestRenderStatusBar
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/view.go internal/tui/view_test.go && git commit -m "feat(tui): add renderStatusBar method with spinner and icons"
```
<!-- END_TASK_4 -->

<!-- START_TASK_5 -->
### Task 5: Integrate status bar into View()

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/view.go`

**Step 1: Update View() to use renderStatusBar**

In the `View()` method, replace the status bar line:

Old:
```go
	// Build status bar with contextual help
	statusBar := lipgloss.NewStyle().Width(layout.StatusBar.Width).Render(m.renderContextualHelp())
```

New:
```go
	// Build status bar with operation feedback and contextual help
	statusBar := lipgloss.NewStyle().Width(layout.StatusBar.Width).Render(m.renderStatusBar(layout.StatusBar.Width))
```

**Step 2: Run tests**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v
```

Expected: All tests pass

**Step 3: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/view.go && git commit -m "refactor(tui): integrate status bar into View()"
```
<!-- END_TASK_5 -->
<!-- END_SUBCOMPONENT_B -->

<!-- START_SUBCOMPONENT_C (tasks 6-8) -->
## Subcomponent C: Status Updates

<!-- START_TASK_6 -->
### Task 6: Add status helper methods

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model.go`

**Step 1: Add status helper methods**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model.go`:

```go
// setStatus updates the status bar message and level.
func (m *Model) setStatus(level StatusLevel, message string) {
	m.statusLevel = level
	m.statusMessage = message
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
}

// clearStatus resets the status bar to default.
func (m *Model) clearStatus() {
	m.statusLevel = StatusInfo
	m.statusMessage = ""
	m.err = nil
}
```

**Step 2: Verify build**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go build ./...
```

Expected: Build succeeds

**Step 3: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/model.go && git commit -m "feat(tui): add status helper methods"
```
<!-- END_TASK_6 -->

<!-- START_TASK_7 -->
### Task 7: Wire container actions to status updates

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update.go`

**Step 1: Write the test**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update_test.go`:

```go
func TestContainerAction_ShowsLoading(t *testing.T) {
	m := newTestModel()

	// Add a container
	ctr := &container.Container{
		ID:    "abc123def456",
		Name:  "test-container",
		State: container.StateStopped,
	}
	m.containerList.SetItems(toListItems([]*container.Container{ctr}))

	// Press 's' to start
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}
	updated, cmd := m.Update(msg)
	m = updated.(Model)

	if m.statusLevel != StatusLoading {
		t.Errorf("statusLevel = %v, want %v", m.statusLevel, StatusLoading)
	}
	if !strings.Contains(m.statusMessage, "Starting") {
		t.Errorf("statusMessage = %q, should contain 'Starting'", m.statusMessage)
	}
	if cmd == nil {
		t.Error("should return command for spinner")
	}
}

func TestContainerActionMsg_Success(t *testing.T) {
	m := newTestModel()
	m.statusLevel = StatusLoading
	m.statusMessage = "Starting..."

	// Simulate success message
	msg := containerActionMsg{action: "start", id: "abc123", err: nil}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.statusLevel != StatusSuccess {
		t.Errorf("statusLevel = %v, want %v", m.statusLevel, StatusSuccess)
	}
}

func TestContainerActionMsg_Error(t *testing.T) {
	m := newTestModel()
	m.statusLevel = StatusLoading

	// Simulate error message
	msg := containerActionMsg{action: "start", id: "abc123", err: fmt.Errorf("connection refused")}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.statusLevel != StatusError {
		t.Errorf("statusLevel = %v, want %v", m.statusLevel, StatusError)
	}
	if m.err == nil {
		t.Error("err should be set")
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestContainerAction
```

Expected: FAIL - actions don't update status yet

**Step 3: Update container action handlers in Update()**

Find the "s" (start), "x" (stop), "d" (destroy) key handlers and modify them to set loading status.

For example, modify the "s" case:

Old:
```go
		case "s":
			if item, ok := m.containerList.SelectedItem().(containerItem); ok {
				return m, m.startContainer(item.container.ID)
			}
```

New:
```go
		case "s":
			if item, ok := m.containerList.SelectedItem().(containerItem); ok {
				cmd := m.setLoading("Starting " + item.container.Name + "...")
				return m, tea.Batch(cmd, m.startContainer(item.container.ID))
			}
```

Do the same for "x" (stop) and "d" (destroy).

**Step 4: Update containerActionMsg handler**

Find the `containerActionMsg` case and modify:

Old:
```go
	case containerActionMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		return m, m.refreshContainers()
```

New:
```go
	case containerActionMsg:
		if msg.err != nil {
			m.setError(fmt.Sprintf("Failed to %s container", msg.action), msg.err)
			return m, nil
		}
		actionNames := map[string]string{
			"start":   "started",
			"stop":    "stopped",
			"destroy": "destroyed",
		}
		m.setSuccess(fmt.Sprintf("Container %s", actionNames[msg.action]))
		return m, m.refreshContainers()
```

**Step 5: Handle spinner tick**

Add spinner tick handling in Update(). Import the spinner package and add this case:

```go
	case spinner.TickMsg:
		if m.statusLevel == StatusLoading {
			var cmd tea.Cmd
			m.statusSpinner, cmd = m.statusSpinner.Update(msg)
			return m, cmd
		}
		return m, nil
```

Add import: `"github.com/charmbracelet/bubbles/spinner"`

**Step 6: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestContainerAction
```

Expected: PASS

**Step 7: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/update.go internal/tui/update_test.go && git commit -m "feat(tui): wire container actions to status bar updates"
```
<!-- END_TASK_7 -->

<!-- START_TASK_8 -->
### Task 8: Add Esc to clear error state

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update.go`

**Step 1: Write the test**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update_test.go`:

```go
func TestEscape_ClearsError(t *testing.T) {
	m := newTestModel()
	m.statusLevel = StatusError
	m.statusMessage = "Something failed"
	m.err = fmt.Errorf("test error")

	// Press Escape
	msg := tea.KeyMsg{Type: tea.KeyEscape}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.statusLevel == StatusError {
		t.Error("statusLevel should not be Error after Escape")
	}
	if m.err != nil {
		t.Error("err should be nil after Escape")
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestEscape_ClearsError
```

Expected: FAIL - Escape doesn't clear error yet

**Step 3: Add Escape handler for error clearing**

In Update(), before the existing key handling, add:

```go
		// Clear error with Escape
		if msg.Type == tea.KeyEscape && m.statusLevel == StatusError {
			m.clearStatus()
			return m, nil
		}
```

This should be early in the key handling, before form/session view delegation.

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestEscape_ClearsError
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/update.go internal/tui/update_test.go && git commit -m "feat(tui): add Escape to clear error status"
```
<!-- END_TASK_8 -->
<!-- END_SUBCOMPONENT_C -->

<!-- START_TASK_9 -->
### Task 9: Run all tests and verify phase complete

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
<!-- END_TASK_9 -->

---

## Phase Completion Checklist

- [ ] Status bar shows at bottom of screen
- [ ] Operations show spinner while in progress
- [ ] Success shows green checkmark
- [ ] Error shows red X
- [ ] Error displays message and "(esc to clear)" hint
- [ ] Esc clears error state
- [ ] Contextual help displays on right side
- [ ] All existing tests still pass

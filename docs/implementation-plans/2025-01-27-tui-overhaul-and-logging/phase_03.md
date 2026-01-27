# TUI Overhaul and Logging Infrastructure - Phase 3: Tab Navigation and Drill-Down

**Goal:** Implement tab switching and container-to-session drill-down flow.

**Architecture:** Tab switching via number keys (1/2) or h/l keys. Enter on container selects it and switches to Tab 2 (Sessions). Session list uses existing infrastructure but renders in the tab layout instead of modal overlay. Backspace in Tab 2 returns to Tab 1.

**Tech Stack:** Go 1.24+, Bubbletea for TUI framework, Bubbles list component

**Scope:** 7 phases from original design (this is phase 3 of 7)

**Codebase verified:** 2025-01-27

---

## Phase Overview

This phase integrates tab navigation with the existing session view infrastructure. The codebase already has:
- `sessionViewOpen`, `selectedContainer`, `selectedSessionIdx` in Model
- `handleSessionViewKey()` for session navigation
- `renderSessionView()` for session rendering (currently modal)

We need to:
1. Add tab switching keys (1/2, h/l)
2. Modify Enter on container to select + switch tab (not open modal)
3. Migrate session rendering from modal to tab content
4. Add Backspace to return to Tab 1

**Dependencies:** Phase 2 (layout system with TabMode)

**Current patterns from investigation:**
- Key handling at `internal/tui/update.go:38-147`
- Session state at `internal/tui/model.go:34-42`
- Session view rendering at `internal/tui/view.go:183-267`

---

<!-- START_SUBCOMPONENT_A (tasks 1-2) -->
## Subcomponent A: Tab Switching

<!-- START_TASK_1 -->
### Task 1: Add tab switching key handlers

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update.go`

**Step 1: Write the test**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update_test.go` (create if doesn't exist):

```go
package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/config"
	"devagent/internal/container"
)

func newTestModel() Model {
	cfg := &config.Config{Theme: "mocha", Runtime: "docker"}
	templates := []config.Template{{Name: "go-project", Description: "Go development"}}
	m := NewModelWithTemplates(cfg, templates)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(Model)
}

// toListItems converts containers to list.Item slice for test setup.
func toListItems(containers []*container.Container) []list.Item {
	items := make([]list.Item, len(containers))
	for i, c := range containers {
		items[i] = c
	}
	return items
}

func TestTabSwitching_NumberKeys(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		startTab   TabMode
		wantTab    TabMode
	}{
		{"press 1 switches to Containers", "1", TabSessions, TabContainers},
		{"press 2 switches to Sessions", "2", TabContainers, TabSessions},
		{"press 1 stays on Containers", "1", TabContainers, TabContainers},
		{"press 2 stays on Sessions", "2", TabSessions, TabSessions},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel()
			m.currentTab = tt.startTab

			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			updated, _ := m.Update(msg)
			result := updated.(Model)

			if result.currentTab != tt.wantTab {
				t.Errorf("currentTab = %v, want %v", result.currentTab, tt.wantTab)
			}
		})
	}
}

func TestTabSwitching_HLKeys(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		startTab   TabMode
		wantTab    TabMode
	}{
		{"press h switches left to Containers", "h", TabSessions, TabContainers},
		{"press l switches right to Sessions", "l", TabContainers, TabSessions},
		{"press h stays on Containers (left boundary)", "h", TabContainers, TabContainers},
		{"press l stays on Sessions (right boundary)", "l", TabSessions, TabSessions},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel()
			m.currentTab = tt.startTab

			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			updated, _ := m.Update(msg)
			result := updated.(Model)

			if result.currentTab != tt.wantTab {
				t.Errorf("currentTab = %v, want %v", result.currentTab, tt.wantTab)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestTabSwitching
```

Expected: FAIL - tab switching not implemented yet

**Step 3: Add tab switching to Update()**

In `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update.go`, add tab switching cases in the main key switch block (after form/session view delegation, before existing cases like "q"):

```go
		// Tab switching with number keys
		case "1":
			m.currentTab = TabContainers
			return m, nil
		case "2":
			m.currentTab = TabSessions
			return m, nil

		// Tab switching with h/l (vim-style)
		case "h":
			if m.currentTab > TabContainers {
				m.currentTab--
			}
			return m, nil
		case "l":
			if m.currentTab < TabSessions {
				m.currentTab++
			}
			return m, nil
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestTabSwitching
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/update.go internal/tui/update_test.go && git commit -m "feat(tui): add tab switching with number keys (1/2) and vim keys (h/l)"
```
<!-- END_TASK_1 -->

<!-- START_TASK_2 -->
### Task 2: Modify Enter to select container and switch tabs

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update.go`
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model.go`

**Step 1: Write the test**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update_test.go`:

```go
func TestEnterOnContainer_SwitchesToSessionsTab(t *testing.T) {
	m := newTestModel()

	// Add a container to the list
	ctr := &container.Container{
		ID:    "abc123def456",
		Name:  "test-container",
		State: container.StateRunning,
	}
	m.containerList.SetItems(toListItems([]*container.Container{ctr}))
	m.currentTab = TabContainers

	// Press Enter
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	// Should switch to Sessions tab
	if result.currentTab != TabSessions {
		t.Errorf("currentTab = %v, want %v", result.currentTab, TabSessions)
	}

	// Should have selected the container
	if result.selectedContainer == nil {
		t.Fatal("selectedContainer should not be nil")
	}
	if result.selectedContainer.ID != ctr.ID {
		t.Errorf("selectedContainer.ID = %q, want %q", result.selectedContainer.ID, ctr.ID)
	}
}

func TestEnterOnContainer_RefreshesSessions(t *testing.T) {
	m := newTestModel()

	// Add a container
	ctr := &container.Container{
		ID:    "abc123def456",
		Name:  "test-container",
		State: container.StateRunning,
	}
	m.containerList.SetItems(toListItems([]*container.Container{ctr}))
	m.currentTab = TabContainers

	// Press Enter
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := m.Update(msg)

	// Should return a command (to refresh sessions)
	if cmd == nil {
		t.Error("Update should return a command to refresh sessions")
	}
}
```

You'll need to add this import to the test file:
```go
import "devagent/internal/container"
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestEnterOnContainer
```

Expected: FAIL - Enter currently opens modal sessionView

**Step 3: Add selectContainer method to model.go**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model.go`:

```go
// selectContainer selects a container and switches to the Sessions tab.
// Returns a command to refresh the container's sessions.
func (m *Model) selectContainer() tea.Cmd {
	if item, ok := m.containerList.SelectedItem().(containerItem); ok {
		m.selectedContainer = item.container
		m.selectedSessionIdx = 0
		m.currentTab = TabSessions
		return m.refreshSessions()
	}
	return nil
}

// SelectedSession returns the currently selected session, or nil if none.
func (m Model) SelectedSession() *container.Session {
	if m.selectedContainer == nil || len(m.selectedContainer.Sessions) == 0 {
		return nil
	}
	if m.selectedSessionIdx < 0 || m.selectedSessionIdx >= len(m.selectedContainer.Sessions) {
		return nil
	}
	return &m.selectedContainer.Sessions[m.selectedSessionIdx]
}
```

**Step 4: Modify Enter key handling in update.go**

Find the "enter" case in the main key switch in Update() and modify it:

Old code (approximately):
```go
		case "enter":
			m.openSessionView()
			return m, nil
```

New code:
```go
		case "enter":
			// In Containers tab: select container and switch to Sessions
			if m.currentTab == TabContainers {
				cmd := m.selectContainer()
				return m, cmd
			}
			return m, nil
```

**Step 5: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestEnterOnContainer
```

Expected: PASS

**Step 6: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/update.go internal/tui/model.go internal/tui/update_test.go && git commit -m "feat(tui): Enter on container selects and switches to Sessions tab"
```
<!-- END_TASK_2 -->
<!-- END_SUBCOMPONENT_A -->

<!-- START_SUBCOMPONENT_B (tasks 3-4) -->
## Subcomponent B: Session List in Tab

<!-- START_TASK_3 -->
### Task 3: Create session list for tab content

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/view.go`
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model.go`

**Step 1: Write the test**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/view_test.go`:

```go
func TestRenderSessionsTabContent_NoContainer(t *testing.T) {
	m := newTestModel()
	m.currentTab = TabSessions
	m.selectedContainer = nil

	layout := ComputeLayout(80, 24, false)
	content := m.renderSessionsTabContent(layout)

	if !strings.Contains(content, "Select a container") {
		t.Error("should show 'Select a container' when no container selected")
	}
}

func TestRenderSessionsTabContent_WithContainer(t *testing.T) {
	m := newTestModel()
	m.currentTab = TabSessions
	m.selectedContainer = &container.Container{
		ID:   "abc123",
		Name: "test-container",
		Sessions: []container.Session{
			{Name: "dev", ContainerID: "abc123", Windows: 2},
			{Name: "test", ContainerID: "abc123", Windows: 1},
		},
	}
	m.selectedSessionIdx = 0

	layout := ComputeLayout(80, 24, false)
	content := m.renderSessionsTabContent(layout)

	if !strings.Contains(content, "test-container") {
		t.Error("should show container name")
	}
	if !strings.Contains(content, "dev") {
		t.Error("should show first session")
	}
	if !strings.Contains(content, "test") {
		t.Error("should show second session")
	}
}

func TestRenderSessionsTabContent_EmptySessions(t *testing.T) {
	m := newTestModel()
	m.currentTab = TabSessions
	m.selectedContainer = &container.Container{
		ID:       "abc123",
		Name:     "test-container",
		Sessions: []container.Session{},
	}

	layout := ComputeLayout(80, 24, false)
	content := m.renderSessionsTabContent(layout)

	if !strings.Contains(content, "No sessions") {
		t.Error("should show 'No sessions' when container has no sessions")
	}
}
```

Add the container import if needed:
```go
import "devagent/internal/container"
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestRenderSessionsTabContent
```

Expected: FAIL - current implementation just shows placeholder

**Step 3: Update renderSessionsTabContent in view.go**

Replace the `renderSessionsTabContent` method in `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/view.go`:

```go
// renderSessionsTabContent renders the sessions tab content.
// Shows "Select container" if no container selected, otherwise session list.
func (m Model) renderSessionsTabContent(layout Layout) string {
	if m.selectedContainer == nil {
		placeholder := m.styles.InfoStyle().Render("Select a container from Tab 1 to view sessions")
		return lipgloss.Place(
			layout.Content.Width,
			layout.Content.Height,
			lipgloss.Center,
			lipgloss.Center,
			placeholder,
		)
	}

	// Build header with container name
	containerName := m.styles.TitleStyle().Render(m.selectedContainer.Name)
	sessionCount := fmt.Sprintf("%d session(s)", len(m.selectedContainer.Sessions))
	header := lipgloss.JoinVertical(lipgloss.Left,
		containerName,
		m.styles.InfoStyle().Render(sessionCount),
	)

	// Build session list or empty message
	var sessionList string
	if len(m.selectedContainer.Sessions) == 0 {
		sessionList = m.styles.InfoStyle().Render("No sessions. Press 't' to create one.")
	} else {
		var lines []string
		for i, session := range m.selectedContainer.Sessions {
			indicator := "  "
			nameStyle := m.styles.InfoStyle()

			if i == m.selectedSessionIdx {
				indicator = m.styles.AccentStyle().Render("▸ ")
				nameStyle = m.styles.AccentStyle()
			}

			status := ""
			if session.Attached {
				status = m.styles.AccentStyle().Render(" (attached)")
			}

			windowInfo := fmt.Sprintf(" [%d windows]", session.Windows)
			line := indicator + nameStyle.Render(session.Name) + status + m.styles.HelpStyle().Render(windowInfo)
			lines = append(lines, line)
		}
		sessionList = lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	// Show attach command if session selected
	var attachCmd string
	if cmd := m.AttachCommand(); cmd != "" {
		attachCmd = m.styles.HelpStyle().Render("Attach: " + cmd)
	}

	// Build help text
	help := m.styles.HelpStyle().Render("↑/↓: navigate • t: create • k: kill • backspace: back to containers")

	// Compose content
	parts := []string{header, "", sessionList}
	if attachCmd != "" {
		parts = append(parts, "", attachCmd)
	}
	parts = append(parts, "", help)

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)

	return lipgloss.NewStyle().
		Width(layout.Content.Width).
		Height(layout.Content.Height).
		Padding(1).
		Render(content)
}
```

Add the `fmt` import if not already present.

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestRenderSessionsTabContent
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/view.go internal/tui/view_test.go && git commit -m "feat(tui): add session list rendering for Sessions tab"
```
<!-- END_TASK_3 -->

<!-- START_TASK_4 -->
### Task 4: Add session navigation in Sessions tab

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update.go`

**Step 1: Write the test**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update_test.go`:

```go
func TestSessionsTab_UpDownNavigation(t *testing.T) {
	m := newTestModel()
	m.currentTab = TabSessions
	m.selectedContainer = &container.Container{
		ID:   "abc123",
		Name: "test-container",
		Sessions: []container.Session{
			{Name: "dev", ContainerID: "abc123"},
			{Name: "test", ContainerID: "abc123"},
			{Name: "prod", ContainerID: "abc123"},
		},
	}
	m.selectedSessionIdx = 0

	// Press down twice
	msg := tea.KeyMsg{Type: tea.KeyDown}
	updated, _ := m.Update(msg)
	m = updated.(Model)
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if m.selectedSessionIdx != 2 {
		t.Errorf("selectedSessionIdx = %d, want 2", m.selectedSessionIdx)
	}

	// Press up once
	msg = tea.KeyMsg{Type: tea.KeyUp}
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if m.selectedSessionIdx != 1 {
		t.Errorf("selectedSessionIdx = %d, want 1", m.selectedSessionIdx)
	}
}

func TestSessionsTab_JKNavigation(t *testing.T) {
	m := newTestModel()
	m.currentTab = TabSessions
	m.selectedContainer = &container.Container{
		ID:   "abc123",
		Name: "test-container",
		Sessions: []container.Session{
			{Name: "dev", ContainerID: "abc123"},
			{Name: "test", ContainerID: "abc123"},
		},
	}
	m.selectedSessionIdx = 0

	// Press j (down)
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.selectedSessionIdx != 1 {
		t.Errorf("selectedSessionIdx = %d, want 1 after 'j'", m.selectedSessionIdx)
	}

	// Press k (up)
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if m.selectedSessionIdx != 0 {
		t.Errorf("selectedSessionIdx = %d, want 0 after 'k'", m.selectedSessionIdx)
	}
}

func TestSessionsTab_Backspace_ReturnsToContainers(t *testing.T) {
	m := newTestModel()
	m.currentTab = TabSessions
	m.selectedContainer = &container.Container{
		ID:   "abc123",
		Name: "test-container",
	}

	// Press backspace
	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.currentTab != TabContainers {
		t.Errorf("currentTab = %v, want %v after backspace", m.currentTab, TabContainers)
	}
	if m.selectedContainer != nil {
		t.Error("selectedContainer should be nil after backspace")
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestSessionsTab
```

Expected: FAIL - navigation in Sessions tab not implemented

**Step 3: Add Sessions tab key handling in Update()**

In `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update.go`, add a handler for Sessions tab keys. Add this before the existing switch on `msg.String()`:

```go
		// Sessions tab navigation
		if m.currentTab == TabSessions && m.selectedContainer != nil {
			switch msg.Type {
			case tea.KeyUp:
				if m.selectedSessionIdx > 0 {
					m.selectedSessionIdx--
				}
				return m, nil
			case tea.KeyDown:
				if m.selectedSessionIdx < len(m.selectedContainer.Sessions)-1 {
					m.selectedSessionIdx++
				}
				return m, nil
			case tea.KeyBackspace:
				m.selectedContainer = nil
				m.selectedSessionIdx = 0
				m.currentTab = TabContainers
				return m, nil
			}

			// Vim-style navigation in Sessions tab
			switch msg.String() {
			case "j":
				if m.selectedSessionIdx < len(m.selectedContainer.Sessions)-1 {
					m.selectedSessionIdx++
				}
				return m, nil
			case "k":
				if m.selectedSessionIdx > 0 {
					m.selectedSessionIdx--
				}
				return m, nil
			case "t":
				m.sessionFormOpen = true
				m.sessionFormName = ""
				return m, nil
			}
		}
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestSessionsTab
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/update.go internal/tui/update_test.go && git commit -m "feat(tui): add session navigation and backspace in Sessions tab"
```
<!-- END_TASK_4 -->
<!-- END_SUBCOMPONENT_B -->

<!-- START_SUBCOMPONENT_C (tasks 5-6) -->
## Subcomponent C: Session Detail View

<!-- START_TASK_5 -->
### Task 5: Add session detail view on Enter

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update.go`
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model.go`

**Step 1: Write the test**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update_test.go`:

```go
func TestSessionsTab_Enter_OpensSessionDetail(t *testing.T) {
	m := newTestModel()
	m.currentTab = TabSessions
	m.selectedContainer = &container.Container{
		ID:   "abc123",
		Name: "test-container",
		Sessions: []container.Session{
			{Name: "dev", ContainerID: "abc123"},
		},
	}
	m.selectedSessionIdx = 0

	// Press Enter
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if !m.sessionDetailOpen {
		t.Error("sessionDetailOpen should be true after Enter on session")
	}
}

func TestSessionDetail_Escape_Returns(t *testing.T) {
	m := newTestModel()
	m.currentTab = TabSessions
	m.selectedContainer = &container.Container{
		ID:       "abc123",
		Name:     "test-container",
		Sessions: []container.Session{{Name: "dev", ContainerID: "abc123"}},
	}
	m.sessionDetailOpen = true

	// Press Escape
	msg := tea.KeyMsg{Type: tea.KeyEscape}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.sessionDetailOpen {
		t.Error("sessionDetailOpen should be false after Escape")
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestSessionsTab_Enter -run TestSessionDetail
```

Expected: FAIL - sessionDetailOpen field doesn't exist

**Step 3: Add sessionDetailOpen to model.go**

Add to the Model struct in `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model.go`:

```go
	// Session detail view
	sessionDetailOpen bool
```

**Step 4: Update Update() to handle Enter and Escape for session detail**

In the Sessions tab navigation section of Update(), add Enter handling:

```go
			case tea.KeyEnter:
				if len(m.selectedContainer.Sessions) > 0 {
					m.sessionDetailOpen = true
				}
				return m, nil
			case tea.KeyEscape:
				if m.sessionDetailOpen {
					m.sessionDetailOpen = false
					return m, nil
				}
				// Escape without detail view goes back to Containers
				m.selectedContainer = nil
				m.selectedSessionIdx = 0
				m.currentTab = TabContainers
				return m, nil
```

**Step 5: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run "TestSessionsTab_Enter|TestSessionDetail"
```

Expected: PASS

**Step 6: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/update.go internal/tui/model.go internal/tui/update_test.go && git commit -m "feat(tui): add session detail view on Enter, Escape to return"
```
<!-- END_TASK_5 -->

<!-- START_TASK_6 -->
### Task 6: Render session detail in view

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/view.go`

**Step 1: Update renderSessionsTabContent to handle detail view**

Modify `renderSessionsTabContent` in `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/view.go` to check for `sessionDetailOpen`:

At the start of `renderSessionsTabContent`, add:

```go
	// If session detail is open, render that instead
	if m.sessionDetailOpen && m.selectedContainer != nil && len(m.selectedContainer.Sessions) > 0 {
		return m.renderSessionDetail(layout)
	}
```

Then add the `renderSessionDetail` method:

```go
// renderSessionDetail renders the detail view for a selected session.
func (m Model) renderSessionDetail(layout Layout) string {
	session := m.selectedContainer.Sessions[m.selectedSessionIdx]

	// Title: Session name
	title := m.styles.TitleStyle().Render(session.Name)

	// Session info
	info := []string{
		m.styles.InfoStyle().Render(fmt.Sprintf("Container: %s", m.selectedContainer.Name)),
		m.styles.InfoStyle().Render(fmt.Sprintf("Windows: %d", session.Windows)),
	}

	if session.Attached {
		info = append(info, m.styles.AccentStyle().Render("Status: Attached"))
	} else {
		info = append(info, m.styles.InfoStyle().Render("Status: Detached"))
	}

	// Attach command
	cmd := m.AttachCommand()
	if cmd != "" {
		info = append(info, "")
		info = append(info, m.styles.HelpStyle().Render("To attach, run:"))
		info = append(info, m.styles.AccentStyle().Render(cmd))
	}

	// Help
	help := m.styles.HelpStyle().Render("esc: back to session list")

	// Compose
	parts := []string{title, ""}
	parts = append(parts, info...)
	parts = append(parts, "", help)

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)

	return lipgloss.NewStyle().
		Width(layout.Content.Width).
		Height(layout.Content.Height).
		Padding(1).
		Render(content)
}
```

**Step 2: Run all tests**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v
```

Expected: All tests pass

**Step 3: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/view.go && git commit -m "feat(tui): add session detail view rendering"
```
<!-- END_TASK_6 -->
<!-- END_SUBCOMPONENT_C -->

<!-- START_TASK_7 -->
### Task 7: Update help text based on current tab

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/view.go`

**Step 1: Create contextual help function**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/view.go`:

```go
// renderContextualHelp returns help text based on current state.
func (m Model) renderContextualHelp() string {
	var help string
	switch m.currentTab {
	case TabContainers:
		help = "q: quit • r: refresh • c: create • s: start • x: stop • d: destroy • enter: sessions • 1/2: tabs"
	case TabSessions:
		if m.selectedContainer == nil {
			help = "1/2: tabs • backspace: containers"
		} else {
			help = "↑/↓: navigate • t: create • k: kill • enter: detail • backspace: containers • 1/2: tabs"
		}
	}
	return m.styles.HelpStyle().Render(help)
}
```

**Step 2: Update View() to use contextual help**

In the `View()` method, replace the hardcoded help line with:

```go
	// Build status bar with contextual help
	statusBar := lipgloss.NewStyle().Width(layout.StatusBar.Width).Render(m.renderContextualHelp())
```

**Step 3: Run tests**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v
```

Expected: All tests pass

**Step 4: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/view.go && git commit -m "feat(tui): add contextual help text based on current tab"
```
<!-- END_TASK_7 -->

<!-- START_TASK_8 -->
### Task 8: Run all tests and verify phase complete

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
<!-- END_TASK_8 -->

---

## Phase Completion Checklist

- [ ] Number keys (1/2) switch between tabs
- [ ] h/l keys switch between tabs (vim-style)
- [ ] Enter on container selects it and switches to Tab 2
- [ ] Tab 2 shows sessions for selected container
- [ ] Up/Down and j/k navigate session list
- [ ] Enter on session opens detail view
- [ ] Esc returns from detail to list
- [ ] Backspace in Tab 2 returns to Tab 1
- [ ] Help text updates based on context
- [ ] All existing tests still pass

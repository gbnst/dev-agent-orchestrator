# TUI Overhaul and Logging Infrastructure - Phase 2: Layout System

**Goal:** Replace centered layout with full-width tab-based layout using Layout struct for region computation.

**Architecture:** Layout struct computes regions based on terminal dimensions. Height budget: 6 fixed lines (header 2, tabs 1, status 1, margins 2) plus dynamic content area. Tab bar follows cc_session_mon patterns with active/inactive styling and gap fill.

**Tech Stack:** Go 1.24+, Lipgloss for styling and layout, Bubbletea for TUI framework

**Scope:** 7 phases from original design (this is phase 2 of 7)

**Codebase verified:** 2025-01-27

---

## Phase Overview

This phase replaces the centered container list with a full-width tab-based layout. We:
1. Create Layout struct for region computation
2. Add tab state (TabMode, currentTab) to Model
3. Refactor View() to use layout regions instead of centered rendering
4. Add tab styles extending existing Catppuccin theme

**Dependencies:** Phase 1 (logging available for debugging)

**Current state from investigation:**
- Model at `internal/tui/model.go:14-44` has width/height fields
- View() at `internal/tui/view.go:60-66` centers content with `lipgloss.Place()`
- Styles at `internal/tui/styles.go` use Catppuccin flavors
- Update() at `internal/tui/update.go:40-50` handles `tea.WindowSizeMsg`

---

<!-- START_SUBCOMPONENT_A (tasks 1-2) -->
## Subcomponent A: Layout System

<!-- START_TASK_1 -->
### Task 1: Create Layout struct and region computation

**Files:**
- Create: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/layout.go`
- Create: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/layout_test.go`

**Step 1: Write the test**

Create `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/layout_test.go`:

```go
package tui

import "testing"

func TestComputeLayout(t *testing.T) {
	tests := []struct {
		name          string
		width         int
		height        int
		logPanelOpen  bool
		wantHeader    Region
		wantContent   int // content height
		wantLogHeight int // 0 if closed
	}{
		{
			name:         "standard terminal no logs",
			width:        80,
			height:       24,
			logPanelOpen: false,
			wantHeader:   Region{X: 0, Y: 0, Width: 80, Height: 2},
			wantContent:  18, // 24 - 6 fixed lines
		},
		{
			name:         "large terminal no logs",
			width:        120,
			height:       40,
			logPanelOpen: false,
			wantHeader:   Region{X: 0, Y: 0, Width: 120, Height: 2},
			wantContent:  34, // 40 - 6 fixed lines
		},
		{
			name:          "standard terminal with logs",
			width:         80,
			height:        24,
			logPanelOpen:  true,
			wantContent:   7,  // 40% of (24-6) = 7.2 -> 7
			wantLogHeight: 11, // 60% of (24-6) = 10.8 -> 11
		},
		{
			name:         "minimum height",
			width:        80,
			height:       10,
			logPanelOpen: false,
			wantContent:  4, // 10 - 6 fixed lines
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := ComputeLayout(tt.width, tt.height, tt.logPanelOpen)

			if layout.Header.Width != tt.width {
				t.Errorf("Header.Width = %d, want %d", layout.Header.Width, tt.width)
			}
			if layout.Header.Height != 2 {
				t.Errorf("Header.Height = %d, want 2", layout.Header.Height)
			}
			if layout.Tabs.Height != 1 {
				t.Errorf("Tabs.Height = %d, want 1", layout.Tabs.Height)
			}
			if layout.Content.Height != tt.wantContent {
				t.Errorf("Content.Height = %d, want %d", layout.Content.Height, tt.wantContent)
			}
			if tt.logPanelOpen {
				if layout.Logs.Height != tt.wantLogHeight {
					t.Errorf("Logs.Height = %d, want %d", layout.Logs.Height, tt.wantLogHeight)
				}
			}
			if layout.StatusBar.Height != 1 {
				t.Errorf("StatusBar.Height = %d, want 1", layout.StatusBar.Height)
			}
		})
	}
}

func TestLayout_TotalHeight(t *testing.T) {
	tests := []struct {
		name         string
		width        int
		height       int
		logPanelOpen bool
	}{
		{"no logs", 80, 24, false},
		{"with logs", 80, 24, true},
		{"large", 120, 40, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := ComputeLayout(tt.width, tt.height, tt.logPanelOpen)

			// Total of all regions should equal terminal height
			total := layout.Header.Height + layout.Tabs.Height + layout.Content.Height
			if tt.logPanelOpen {
				total += layout.Logs.Height + 1 // +1 for separator
			}
			total += layout.StatusBar.Height

			// Allow 1-2 line variance for margins
			if total < tt.height-2 || total > tt.height {
				t.Errorf("total height = %d, want ~%d", total, tt.height)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestComputeLayout
```

Expected: FAIL - ComputeLayout undefined

**Step 3: Write the implementation**

Create `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/layout.go`:

```go
package tui

// Region defines a rectangular area within the terminal.
type Region struct {
	X      int // Left position (0-indexed)
	Y      int // Top position (0-indexed)
	Width  int // Width in cells
	Height int // Height in lines
}

// Layout holds computed regions for all UI components.
type Layout struct {
	Header    Region // App title and subtitle (2 lines)
	Tabs      Region // Tab bar (1 line)
	Content   Region // Main content area (dynamic)
	Logs      Region // Log panel when open (dynamic, 60% of content area)
	StatusBar Region // Status bar (1 line)
	Separator Region // Separator between content and logs (1 line when logs open)
}

// Fixed heights for chrome elements
const (
	headerHeight    = 2 // Title + subtitle
	tabsHeight      = 1 // Tab bar
	statusBarHeight = 1 // Status bar
	marginHeight    = 2 // Top + bottom margins
	separatorHeight = 1 // Separator when log panel open
)

// ComputeLayout calculates regions based on terminal dimensions.
// When logPanelOpen is true, content area splits 40/60 (content/logs).
func ComputeLayout(width, height int, logPanelOpen bool) Layout {
	// Calculate available height for dynamic content
	fixedHeight := headerHeight + tabsHeight + statusBarHeight + marginHeight
	availableHeight := height - fixedHeight

	// Ensure minimum usable height
	if availableHeight < 4 {
		availableHeight = 4
	}

	var contentHeight, logsHeight int
	if logPanelOpen {
		// 40% content, 60% logs (subtract 1 for separator)
		workableHeight := availableHeight - separatorHeight
		contentHeight = int(float64(workableHeight) * 0.4)
		logsHeight = workableHeight - contentHeight
	} else {
		contentHeight = availableHeight
		logsHeight = 0
	}

	// Build layout top-to-bottom
	y := 0

	header := Region{X: 0, Y: y, Width: width, Height: headerHeight}
	y += headerHeight

	tabs := Region{X: 0, Y: y, Width: width, Height: tabsHeight}
	y += tabsHeight

	content := Region{X: 0, Y: y, Width: width, Height: contentHeight}
	y += contentHeight

	var separator, logs Region
	if logPanelOpen {
		separator = Region{X: 0, Y: y, Width: width, Height: separatorHeight}
		y += separatorHeight

		logs = Region{X: 0, Y: y, Width: width, Height: logsHeight}
		y += logsHeight
	}

	statusBar := Region{X: 0, Y: y, Width: width, Height: statusBarHeight}

	return Layout{
		Header:    header,
		Tabs:      tabs,
		Content:   content,
		Logs:      logs,
		StatusBar: statusBar,
		Separator: separator,
	}
}

// ContentListHeight returns the height available for the container/session list
// after accounting for list chrome (selection indicator, padding).
func (l Layout) ContentListHeight() int {
	// Subtract 2 for list padding/borders
	h := l.Content.Height - 2
	if h < 1 {
		h = 1
	}
	return h
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestComputeLayout
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/layout.go internal/tui/layout_test.go && git commit -m "feat(tui): add Layout struct for region computation"
```
<!-- END_TASK_1 -->

<!-- START_TASK_2 -->
### Task 2: Add tab state to Model

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model.go`

**Step 1: Write the test**

Add to existing test file or create `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model_test.go`:

```go
package tui

import "testing"

func TestTabMode_String(t *testing.T) {
	tests := []struct {
		tab  TabMode
		want string
	}{
		{TabContainers, "Containers"},
		{TabSessions, "Sessions"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.tab.String(); got != tt.want {
				t.Errorf("TabMode.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestModel_HasTabs(t *testing.T) {
	// After this phase, Model should have currentTab field
	// This is a compile-time check more than runtime
	m := Model{
		currentTab:   TabContainers,
		logPanelOpen: false,
	}

	if m.currentTab != TabContainers {
		t.Errorf("currentTab = %v, want %v", m.currentTab, TabContainers)
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestTabMode
```

Expected: FAIL - TabMode undefined

**Step 3: Add TabMode and state to model.go**

Modify `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model.go`.

Add after the imports, before the Model struct:

```go
// TabMode represents which tab is currently active.
type TabMode int

const (
	TabContainers TabMode = iota
	TabSessions
)

// String returns the display name for the tab.
func (t TabMode) String() string {
	switch t {
	case TabContainers:
		return "Containers"
	case TabSessions:
		return "Sessions"
	default:
		return "Unknown"
	}
}
```

Add these fields to the Model struct (after existing fields like `err error`):

```go
	// Tab navigation
	currentTab   TabMode
	logPanelOpen bool
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestTabMode
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/model.go internal/tui/model_test.go && git commit -m "feat(tui): add TabMode enum and tab state to Model"
```
<!-- END_TASK_2 -->
<!-- END_SUBCOMPONENT_A -->

<!-- START_SUBCOMPONENT_B (tasks 3-4) -->
## Subcomponent B: Tab Styles

<!-- START_TASK_3 -->
### Task 3: Add tab styles to styles.go

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/styles.go`

**Step 1: Write the test**

Create `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/styles_test.go`:

```go
package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestStyles_TabStyles(t *testing.T) {
	styles := NewStyles("mocha")

	// Verify tab styles exist and return non-empty styles
	activeStyle := styles.ActiveTabStyle()
	if activeStyle.GetBold() != true {
		t.Error("ActiveTabStyle should be bold")
	}

	inactiveStyle := styles.InactiveTabStyle()
	// Inactive should not be bold
	if inactiveStyle.GetBold() == true {
		t.Error("InactiveTabStyle should not be bold")
	}

	// Tab gap should return a string
	gap := styles.TabGapFill()
	if gap == "" {
		t.Error("TabGapFill should return a non-empty string")
	}
}

func TestStyles_AllFlavors(t *testing.T) {
	flavors := []string{"latte", "frappe", "macchiato", "mocha"}

	for _, flavor := range flavors {
		t.Run(flavor, func(t *testing.T) {
			styles := NewStyles(flavor)

			// Verify all tab styles work for each flavor
			_ = styles.ActiveTabStyle()
			_ = styles.InactiveTabStyle()
			_ = styles.TabGapFill()

			// Existing styles should still work
			_ = styles.TitleStyle()
			_ = styles.ErrorStyle()
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestStyles_Tab
```

Expected: FAIL - ActiveTabStyle undefined

**Step 3: Add tab styles**

Add these methods to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/styles.go` (after existing style methods):

```go
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
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestStyles_Tab
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/styles.go internal/tui/styles_test.go && git commit -m "feat(tui): add tab styles extending Catppuccin theme"
```
<!-- END_TASK_3 -->

<!-- START_TASK_4 -->
### Task 4: Add renderTabs method

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/view.go`

**Step 1: Write the test**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/view_test.go` (create if doesn't exist):

```go
package tui

import (
	"strings"
	"testing"
)

func TestRenderTabs(t *testing.T) {
	styles := NewStyles("mocha")

	tests := []struct {
		name       string
		currentTab TabMode
		width      int
		wantActive string
	}{
		{
			name:       "containers tab active",
			currentTab: TabContainers,
			width:      80,
			wantActive: "1 Containers",
		},
		{
			name:       "sessions tab active",
			currentTab: TabSessions,
			width:      80,
			wantActive: "2 Sessions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderTabs(tt.currentTab, tt.width, styles)

			if !strings.Contains(result, tt.wantActive) {
				t.Errorf("renderTabs() should contain %q, got %q", tt.wantActive, result)
			}
			if !strings.Contains(result, "1 Containers") {
				t.Errorf("renderTabs() should always contain '1 Containers'")
			}
			if !strings.Contains(result, "2 Sessions") {
				t.Errorf("renderTabs() should always contain '2 Sessions'")
			}
		})
	}
}

func TestRenderTabs_FillsWidth(t *testing.T) {
	styles := NewStyles("mocha")

	result := renderTabs(TabContainers, 80, styles)
	// Should contain gap fill characters
	if !strings.Contains(result, "─") {
		t.Error("renderTabs() should contain gap fill characters")
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestRenderTabs
```

Expected: FAIL - renderTabs undefined

**Step 3: Add renderTabs function**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/view.go` (before the View method):

```go
import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderTabs renders the tab bar with active/inactive styling.
func renderTabs(currentTab TabMode, width int, styles *Styles) string {
	tabs := []struct {
		key  string
		name string
		mode TabMode
	}{
		{"1", "Containers", TabContainers},
		{"2", "Sessions", TabSessions},
	}

	var parts []string
	for _, tab := range tabs {
		label := tab.key + " " + tab.name
		var style lipgloss.Style
		if tab.mode == currentTab {
			style = styles.ActiveTabStyle()
		} else {
			style = styles.InactiveTabStyle()
		}
		parts = append(parts, style.Render(label))
	}

	tabContent := lipgloss.JoinHorizontal(lipgloss.Bottom, parts...)

	// Calculate remaining width and fill with gap character
	tabWidth := lipgloss.Width(tabContent)
	remaining := width - tabWidth
	if remaining > 0 {
		gap := styles.TabGapStyle().Render(strings.Repeat(styles.TabGapFill(), remaining))
		tabContent = lipgloss.JoinHorizontal(lipgloss.Bottom, tabContent, gap)
	}

	return tabContent
}
```

Note: You'll need to ensure the `strings` import is added if not already present.

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestRenderTabs
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/view.go internal/tui/view_test.go && git commit -m "feat(tui): add renderTabs function for tab bar rendering"
```
<!-- END_TASK_4 -->
<!-- END_SUBCOMPONENT_B -->

<!-- START_SUBCOMPONENT_C (tasks 5-6) -->
## Subcomponent C: View Integration

<!-- START_TASK_5 -->
### Task 5: Refactor View() to use layout

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/view.go`

**Step 1: Understand current View() structure**

Current View() at lines 9-70 does:
1. Checks formOpen/sessionViewOpen for modal rendering
2. Builds parts: title, subtitle, theme info, container list/empty, error, help
3. Joins vertically with lipgloss.JoinVertical
4. Centers with lipgloss.Place(m.width, m.height, ...)

**Step 2: Refactor View() to use Layout**

Replace the View() method in `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/view.go`:

```go
// View renders the TUI.
func (m Model) View() string {
	// Modal forms render on top of everything
	if m.formOpen {
		return m.renderForm()
	}

	// Session detail is also a modal overlay
	if m.sessionViewOpen {
		return m.renderSessionView()
	}

	// Compute layout regions
	layout := ComputeLayout(m.width, m.height, m.logPanelOpen)

	// Build header (title + subtitle)
	title := m.styles.TitleStyle().Render("devagent")
	subtitle := m.styles.SubtitleStyle().Render("Development Agent Orchestrator")
	themeInfo := m.styles.InfoStyle().Render("theme: " + m.themeName)
	header := lipgloss.JoinVertical(lipgloss.Left, title, subtitle+" "+themeInfo)
	header = lipgloss.NewStyle().Width(layout.Header.Width).Render(header)

	// Build tab bar
	tabs := renderTabs(m.currentTab, layout.Tabs.Width, m.styles)

	// Build content based on current tab
	var content string
	switch m.currentTab {
	case TabContainers:
		content = m.renderContainerContent(layout)
	case TabSessions:
		content = m.renderSessionsTabContent(layout)
	}

	// Build status bar (placeholder for Phase 4)
	help := m.styles.HelpStyle().Render("q: quit • r: refresh • c: create • s: start • x: stop • d: destroy • 1/2: tabs")
	statusBar := lipgloss.NewStyle().Width(layout.StatusBar.Width).Render(help)

	// Error display (if any)
	var errorDisplay string
	if m.err != nil {
		errorDisplay = m.styles.ErrorStyle().Render("Error: " + m.err.Error())
	}

	// Compose full layout
	parts := []string{header, tabs, content}
	if errorDisplay != "" {
		parts = append(parts, errorDisplay)
	}
	parts = append(parts, statusBar)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// renderContainerContent renders the container list for the Containers tab.
func (m Model) renderContainerContent(layout Layout) string {
	var content string
	if len(m.containerList.Items()) == 0 {
		emptyMsg := m.styles.InfoStyle().Render("No containers. Press 'c' to create one.")
		content = lipgloss.Place(
			layout.Content.Width,
			layout.Content.Height,
			lipgloss.Center,
			lipgloss.Center,
			emptyMsg,
		)
	} else {
		content = m.containerList.View()
	}

	return lipgloss.NewStyle().
		Width(layout.Content.Width).
		Height(layout.Content.Height).
		Render(content)
}

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

	// TODO: Phase 3 will add session list rendering here
	sessionInfo := m.styles.InfoStyle().Render("Sessions for: " + m.selectedContainer.Name)
	return lipgloss.Place(
		layout.Content.Width,
		layout.Content.Height,
		lipgloss.Center,
		lipgloss.Center,
		sessionInfo,
	)
}
```

**Step 3: Run existing tests**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v
```

Expected: All tests pass

**Step 4: Run full build**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go build ./...
```

Expected: Build succeeds

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/view.go && git commit -m "feat(tui): refactor View() to use layout-based full-width rendering"
```
<!-- END_TASK_5 -->

<!-- START_TASK_6 -->
### Task 6: Update window resize handler

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update.go`

**Step 1: Update WindowSizeMsg handler**

Current handler at lines 40-50 calculates `listHeight := m.height - 8`. Update it to use Layout:

Find and replace the `tea.WindowSizeMsg` case in Update() (around lines 40-50):

```go
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Use Layout for consistent height calculation
		layout := ComputeLayout(m.width, m.height, m.logPanelOpen)
		listHeight := layout.ContentListHeight()

		m.containerList.SetSize(m.width-4, listHeight)
		return m, nil
```

**Step 2: Run tests**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v
```

Expected: All tests pass

**Step 3: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/update.go && git commit -m "refactor(tui): use Layout for window resize calculations"
```
<!-- END_TASK_6 -->
<!-- END_SUBCOMPONENT_C -->

<!-- START_TASK_7 -->
### Task 7: Run all tests and verify phase complete

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
<!-- END_TASK_7 -->

---

## Phase Completion Checklist

- [ ] TUI renders full-width with persistent tab bar
- [ ] Tab 1 shows container list
- [ ] Tab 2 shows "Select container" placeholder
- [ ] Layout adjusts correctly on terminal resize
- [ ] Visual appearance uses Catppuccin colors for tabs
- [ ] All existing tests still pass

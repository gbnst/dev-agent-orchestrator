# TUI Overhaul and Logging Infrastructure - Phase 5: Inline Operation Feedback

**Goal:** Show spinners on list items during container/session operations.

**Architecture:** pendingOperations map tracks containerID → operation type. The containerDelegate checks this map during Render() and replaces the state indicator bullet with a spinner frame for pending containers. Multiple concurrent operations show independent spinners.

**Tech Stack:** Go 1.24+, Bubbletea, Bubbles spinner component

**Scope:** 7 phases from original design (this is phase 5 of 7)

**Codebase verified:** 2025-01-27

---

## Phase Overview

This phase extends the status bar spinner (Phase 4) to show inline feedback on individual list items. We:
1. Add pendingOperations map to Model
2. Pass spinner frame to delegate for rendering
3. Modify containerDelegate.Render() to show spinner for pending items
4. Wire container actions to set/clear pending state

**Dependencies:** Phase 4 (status bar provides spinner component)

**Current patterns from investigation:**
- containerDelegate.Render() at `internal/tui/delegates.go:63-116`
- State indicator at lines 107-110: `stateIndicator = "●"` with state color
- Output format at line 115: `{indicator}{stateIndicator} {title}\n    {desc}`
- No existing pendingOperations tracking

---

<!-- START_SUBCOMPONENT_A (tasks 1-2) -->
## Subcomponent A: Pending Operations Tracking

<!-- START_TASK_1 -->
### Task 1: Add pendingOperations map to Model

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model.go`

**Step 1: Write the test**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model_test.go`:

```go
func TestModel_PendingOperations(t *testing.T) {
	m := newTestModel()

	// Initially empty
	if len(m.pendingOperations) != 0 {
		t.Error("pendingOperations should be empty initially")
	}

	// Can add pending operation
	m.setPending("abc123", "start")
	if op, ok := m.pendingOperations["abc123"]; !ok || op != "start" {
		t.Errorf("pendingOperations[abc123] = %q, want 'start'", op)
	}

	// Can check if pending
	if !m.isPending("abc123") {
		t.Error("abc123 should be pending")
	}
	if m.isPending("xyz789") {
		t.Error("xyz789 should not be pending")
	}

	// Can clear pending operation
	m.clearPending("abc123")
	if m.isPending("abc123") {
		t.Error("abc123 should not be pending after clear")
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestModel_PendingOperations
```

Expected: FAIL - pendingOperations undefined

**Step 3: Add pendingOperations to Model**

Add field to Model struct in `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model.go`:

```go
	// Pending operations (containerID -> operation type)
	pendingOperations map[string]string
```

Add helper methods:

```go
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
```

Initialize the map in NewModelWithTemplates:

```go
	m.pendingOperations = make(map[string]string)
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestModel_PendingOperations
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/model.go internal/tui/model_test.go && git commit -m "feat(tui): add pendingOperations tracking to Model"
```
<!-- END_TASK_1 -->

<!-- START_TASK_2 -->
### Task 2: Wire container actions to pending tracking

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update.go`

**Step 1: Write the test**

Add to `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update_test.go`:

```go
func TestContainerAction_SetsPending(t *testing.T) {
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
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if !m.isPending("abc123def456") {
		t.Error("container should be pending after start action")
	}
	if m.getPendingOperation("abc123def456") != "start" {
		t.Errorf("pending operation = %q, want 'start'", m.getPendingOperation("abc123def456"))
	}
}

func TestContainerActionMsg_ClearsPending(t *testing.T) {
	m := newTestModel()
	m.setPending("abc123", "start")

	// Simulate success message
	msg := containerActionMsg{action: "start", id: "abc123", err: nil}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.isPending("abc123") {
		t.Error("container should not be pending after action completes")
	}
}

func TestContainerActionMsg_ClearsPendingOnError(t *testing.T) {
	m := newTestModel()
	m.setPending("abc123", "start")

	// Simulate error message
	msg := containerActionMsg{action: "start", id: "abc123", err: fmt.Errorf("failed")}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if m.isPending("abc123") {
		t.Error("container should not be pending after error")
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run "TestContainerAction_SetsPending|TestContainerActionMsg_Clears"
```

Expected: FAIL - pending not set/cleared yet

**Step 3: Update container action key handlers**

Find the "s", "x", "d" cases and add setPending calls. Example for "s":

```go
		case "s":
			if item, ok := m.containerList.SelectedItem().(containerItem); ok {
				m.setPending(item.container.ID, "start")
				cmd := m.setLoading("Starting " + item.container.Name + "...")
				return m, tea.Batch(cmd, m.startContainer(item.container.ID))
			}
```

Do the same for "x" (operation: "stop") and "d" (operation: "destroy").

**Step 4: Update containerActionMsg handler to clear pending**

Modify the `containerActionMsg` case:

```go
	case containerActionMsg:
		// Clear pending state regardless of success/error
		m.clearPending(msg.id)

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

**Step 5: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run "TestContainerAction_SetsPending|TestContainerActionMsg_Clears"
```

Expected: PASS

**Step 6: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/update.go internal/tui/update_test.go && git commit -m "feat(tui): wire container actions to pending tracking"
```
<!-- END_TASK_2 -->
<!-- END_SUBCOMPONENT_A -->

<!-- START_SUBCOMPONENT_B (tasks 3-4) -->
## Subcomponent B: Delegate Spinner Rendering

<!-- START_TASK_3 -->
### Task 3: Add spinner frame to delegate

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/delegates.go`

**Step 1: Update containerDelegate to include spinner frame and pending check**

The delegate needs access to:
1. Current spinner frame (from Model's statusSpinner)
2. Pending operations map

Modify the containerDelegate struct:

```go
type containerDelegate struct {
	styles        *Styles
	spinnerFrame  string
	pendingOps    map[string]string
}

func newContainerDelegate(styles *Styles) containerDelegate {
	return containerDelegate{
		styles:     styles,
		pendingOps: make(map[string]string),
	}
}
```

Add a method to update delegate state:

```go
// WithSpinnerState returns a delegate with updated spinner state.
func (d containerDelegate) WithSpinnerState(spinnerFrame string, pendingOps map[string]string) containerDelegate {
	d.spinnerFrame = spinnerFrame
	d.pendingOps = pendingOps
	return d
}
```

**Step 2: Update Render() to show spinner for pending items**

Modify the state indicator section (around lines 107-110):

```go
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
```

**Step 3: Verify build**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go build ./...
```

Expected: Build succeeds

**Step 4: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/delegates.go && git commit -m "feat(tui): add spinner frame support to containerDelegate"
```
<!-- END_TASK_3 -->

<!-- START_TASK_4 -->
### Task 4: Update list delegate on each render

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/view.go`

**Step 1: Update delegate before rendering list**

In the `renderContainerContent` method (or wherever the container list is rendered), update the delegate with current spinner state before calling `m.containerList.View()`.

The challenge is that Bubbles list caches the delegate. We need to update the delegate on the list each time we render.

Find where the container list is rendered and update the delegate:

```go
// renderContainerContent renders the container list for the Containers tab.
func (m Model) renderContainerContent(layout Layout) string {
	// Update delegate with current spinner state
	delegate := m.containerList.Delegate().(containerDelegate)
	updatedDelegate := delegate.WithSpinnerState(m.statusSpinner.View(), m.pendingOperations)
	m.containerList.SetDelegate(updatedDelegate)

	var content string
	if len(m.containerList.Items()) == 0 {
		// ... empty state handling
	} else {
		content = m.containerList.View()
	}

	return lipgloss.NewStyle().
		Width(layout.Content.Width).
		Height(layout.Content.Height).
		Render(content)
}
```

Note: Since Model is passed by value to View(), we need to be careful. The list delegate update should work because the list itself is a value that gets modified. However, to be safe, we might need to update the delegate in the Update() method instead, on each spinner tick.

**Alternative approach - update delegate in Update():**

In the `spinner.TickMsg` case:

```go
	case spinner.TickMsg:
		if m.statusLevel == StatusLoading {
			var cmd tea.Cmd
			m.statusSpinner, cmd = m.statusSpinner.Update(msg)

			// Update list delegate with new spinner frame
			if delegate, ok := m.containerList.Delegate().(containerDelegate); ok {
				m.containerList.SetDelegate(delegate.WithSpinnerState(m.statusSpinner.View(), m.pendingOperations))
			}

			return m, cmd
		}
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
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/view.go internal/tui/update.go && git commit -m "feat(tui): update list delegate with spinner state on each tick"
```
<!-- END_TASK_4 -->
<!-- END_SUBCOMPONENT_B -->

<!-- START_TASK_5 -->
### Task 5: Test spinner rendering in delegate

**Files:**
- Create: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/delegates_test.go`

**Step 1: Write tests for delegate spinner rendering**

Create `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/delegates_test.go`:

```go
package tui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"

	"devagent/internal/container"
)

func TestContainerDelegate_ShowsSpinnerForPending(t *testing.T) {
	styles := NewStyles("mocha")
	delegate := newContainerDelegate(styles)

	// Set up pending operation
	pendingOps := map[string]string{"abc123": "start"}
	delegate = delegate.WithSpinnerState("⠋", pendingOps)

	// Create a container item
	ctr := &container.Container{
		ID:    "abc123",
		Name:  "test-container",
		State: container.StateStopped,
	}
	item := containerItem{container: ctr}

	// Create a minimal list model for testing
	items := []list.Item{item}
	l := list.New(items, delegate, 80, 10)

	// Render the item
	var buf bytes.Buffer
	delegate.Render(&buf, l, 0, item)

	output := buf.String()

	// Should contain spinner frame instead of bullet
	if !strings.Contains(output, "⠋") {
		t.Errorf("output should contain spinner frame, got: %q", output)
	}
	if strings.Contains(output, "●") {
		t.Error("output should not contain bullet when pending")
	}
}

func TestContainerDelegate_ShowsBulletWhenNotPending(t *testing.T) {
	styles := NewStyles("mocha")
	delegate := newContainerDelegate(styles)

	// No pending operations
	delegate = delegate.WithSpinnerState("⠋", map[string]string{})

	// Create a container item
	ctr := &container.Container{
		ID:    "abc123",
		Name:  "test-container",
		State: container.StateRunning,
	}
	item := containerItem{container: ctr}

	items := []list.Item{item}
	l := list.New(items, delegate, 80, 10)

	var buf bytes.Buffer
	delegate.Render(&buf, l, 0, item)

	output := buf.String()

	// Should contain bullet, not spinner
	if !strings.Contains(output, "●") {
		t.Errorf("output should contain bullet, got: %q", output)
	}
}
```

**Step 2: Run tests**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test ./internal/tui/... -v -run TestContainerDelegate
```

Expected: PASS

**Step 3: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/delegates_test.go && git commit -m "test(tui): add tests for delegate spinner rendering"
```
<!-- END_TASK_5 -->

<!-- START_TASK_6 -->
### Task 6: Run all tests and verify phase complete

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
<!-- END_TASK_6 -->

---

## Phase Completion Checklist

- [ ] pendingOperations map tracks active operations by container ID
- [ ] Starting a container shows spinner on that list item
- [ ] Stopping a container shows spinner on that list item
- [ ] Destroying a container shows spinner on that list item
- [ ] Spinner clears on success
- [ ] Spinner clears on error
- [ ] Multiple concurrent operations show independent spinners
- [ ] All existing tests still pass

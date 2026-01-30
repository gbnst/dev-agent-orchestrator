package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/config"
	"devagent/internal/container"
	"devagent/internal/logging"
)

func TestTreeItemType_Container(t *testing.T) {
	item := TreeItem{Type: TreeItemContainer, ContainerID: "abc123"}
	if item.Type != TreeItemContainer {
		t.Error("expected TreeItemContainer type")
	}
	if !item.IsContainer() {
		t.Error("IsContainer should return true for container items")
	}
	if item.IsSession() {
		t.Error("IsSession should return false for container items")
	}
}

func TestTreeItemType_Session(t *testing.T) {
	item := TreeItem{Type: TreeItemSession, ContainerID: "abc123", SessionName: "dev"}
	if item.Type != TreeItemSession {
		t.Error("expected TreeItemSession type")
	}
	if !item.IsSession() {
		t.Error("IsSession should return true for session items")
	}
	if item.IsContainer() {
		t.Error("IsContainer should return false for session items")
	}
}

func TestTreeItem_Expanded(t *testing.T) {
	item := TreeItem{Type: TreeItemContainer, ContainerID: "abc123", Expanded: true}
	if !item.Expanded {
		t.Error("Expanded should be true when set")
	}
}

// Helper to create a test model for tree tests
func newTreeTestModel(t *testing.T) Model {
	cfg := &config.Config{
		Theme:   "mocha",
		Runtime: "docker",
	}
	templates := []config.Template{
		{Name: "go-project"},
	}
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test-tree.log"
	lm, _ := logging.NewManager(logging.Config{
		FilePath:       logPath,
		MaxSizeMB:      1,
		MaxBackups:     1,
		MaxAgeDays:     1,
		ChannelBufSize: 100,
		Level:          "debug",
	})
	return NewModelWithTemplates(cfg, templates, lm)
}

func TestRebuildTreeItems_CollapsedContainers(t *testing.T) {
	m := newTreeTestModel(t)

	// Add containers to the list
	items := []list.Item{
		containerItem{container: &container.Container{ID: "c1", Name: "container-1"}},
		containerItem{container: &container.Container{ID: "c2", Name: "container-2"}},
	}
	m.containerList.SetItems(items)

	// All collapsed by default (expandedContainers is empty/nil)
	m.rebuildTreeItems()

	// 1 All + 2 containers = 3 items
	if len(m.treeItems) != 3 {
		t.Errorf("expected 3 items (All + 2 collapsed containers), got %d", len(m.treeItems))
	}
	if m.treeItems[0].Type != TreeItemAll {
		t.Error("first item should be All")
	}
	if m.treeItems[1].Type != TreeItemContainer {
		t.Error("second item should be container")
	}
	if m.treeItems[2].Type != TreeItemContainer {
		t.Error("third item should be container")
	}
}

func TestRebuildTreeItems_ExpandedContainer(t *testing.T) {
	m := newTreeTestModel(t)

	c1 := &container.Container{
		ID:   "c1",
		Name: "container-1",
		Sessions: []container.Session{
			{Name: "dev", ContainerID: "c1"},
			{Name: "test", ContainerID: "c1"},
		},
	}
	items := []list.Item{containerItem{container: c1}}
	m.containerList.SetItems(items)

	// Mark container as expanded
	m.expandedContainers = map[string]bool{"c1": true}

	m.rebuildTreeItems()

	// 1 All + 1 container + 2 sessions = 4 items
	if len(m.treeItems) != 4 {
		t.Errorf("expected 4 items, got %d", len(m.treeItems))
	}
	if m.treeItems[0].Type != TreeItemAll {
		t.Error("first item should be All")
	}
	if m.treeItems[1].Type != TreeItemContainer {
		t.Error("second item should be container")
	}
	if m.treeItems[1].ContainerID != "c1" {
		t.Errorf("second item should have ContainerID 'c1', got %s", m.treeItems[1].ContainerID)
	}
	if m.treeItems[2].Type != TreeItemSession {
		t.Error("third item should be session")
	}
	if m.treeItems[2].SessionName != "dev" {
		t.Errorf("third item should be session 'dev', got %s", m.treeItems[2].SessionName)
	}
	if m.treeItems[3].Type != TreeItemSession {
		t.Error("fourth item should be session")
	}
	if m.treeItems[3].SessionName != "test" {
		t.Errorf("fourth item should be session 'test', got %s", m.treeItems[3].SessionName)
	}
}

func TestRebuildTreeItems_MixedExpansion(t *testing.T) {
	m := newTreeTestModel(t)

	c1 := &container.Container{
		ID:   "c1",
		Name: "container-1",
		Sessions: []container.Session{
			{Name: "dev", ContainerID: "c1"},
		},
	}
	c2 := &container.Container{
		ID:   "c2",
		Name: "container-2",
		Sessions: []container.Session{
			{Name: "prod", ContainerID: "c2"},
		},
	}
	items := []list.Item{
		containerItem{container: c1},
		containerItem{container: c2},
	}
	m.containerList.SetItems(items)

	// Only first container expanded
	m.expandedContainers = map[string]bool{"c1": true}

	m.rebuildTreeItems()

	// All + c1 (expanded) + dev session + c2 (collapsed) = 4 items
	if len(m.treeItems) != 4 {
		t.Errorf("expected 4 items, got %d", len(m.treeItems))
	}
	if m.treeItems[0].Type != TreeItemAll {
		t.Error("first item should be All")
	}
	if m.treeItems[1].ContainerID != "c1" || m.treeItems[1].Type != TreeItemContainer {
		t.Error("second item should be container c1")
	}
	if m.treeItems[2].SessionName != "dev" || m.treeItems[2].Type != TreeItemSession {
		t.Error("third item should be session dev")
	}
	if m.treeItems[3].ContainerID != "c2" || m.treeItems[3].Type != TreeItemContainer {
		t.Error("fourth item should be container c2")
	}
}

func TestRebuildTreeItems_EmptyContainers(t *testing.T) {
	m := newTreeTestModel(t)

	// No containers
	m.containerList.SetItems([]list.Item{})

	m.rebuildTreeItems()

	// All is always present, even with no containers
	if len(m.treeItems) != 1 {
		t.Errorf("expected 1 item (All) for empty container list, got %d", len(m.treeItems))
	}
	if m.treeItems[0].Type != TreeItemAll {
		t.Error("only item should be All")
	}
}

func TestRebuildTreeItems_ExpandedState(t *testing.T) {
	m := newTreeTestModel(t)

	c1 := &container.Container{
		ID:       "c1",
		Name:     "container-1",
		Sessions: []container.Session{},
	}
	items := []list.Item{containerItem{container: c1}}
	m.containerList.SetItems(items)

	// Mark as expanded
	m.expandedContainers = map[string]bool{"c1": true}

	m.rebuildTreeItems()

	// Container item should have Expanded = true (index 1, after All)
	if !m.treeItems[1].Expanded {
		t.Error("container tree item should have Expanded=true when in expandedContainers")
	}
}

// Helper to create a model with containers for tree navigation tests
func newTreeTestModelWithContainers(t *testing.T, count int) Model {
	m := newTreeTestModel(t)

	var items []list.Item
	for i := 0; i < count; i++ {
		c := &container.Container{
			ID:   fmt.Sprintf("c%d", i+1),
			Name: fmt.Sprintf("container-%d", i+1),
			Sessions: []container.Session{
				{Name: "dev", ContainerID: fmt.Sprintf("c%d", i+1)},
				{Name: "test", ContainerID: fmt.Sprintf("c%d", i+1)},
			},
		}
		items = append(items, containerItem{container: c})
	}
	m.containerList.SetItems(items)
	m.expandedContainers = make(map[string]bool)
	m.rebuildTreeItems()
	return m
}

// Tree Navigation Tests

func TestTreeNavigation_DownKey(t *testing.T) {
	m := newTreeTestModelWithContainers(t, 3)
	m.selectedIdx = 0

	msg := tea.KeyMsg{Type: tea.KeyDown}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.selectedIdx != 1 {
		t.Errorf("expected selectedIdx=1, got %d", result.selectedIdx)
	}
}

func TestTreeNavigation_DownKey_StopsAtEnd(t *testing.T) {
	m := newTreeTestModelWithContainers(t, 2)
	// All + 2 containers = 3 items, last index is 2
	m.selectedIdx = 2

	msg := tea.KeyMsg{Type: tea.KeyDown}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	// Should stay at last item
	if result.selectedIdx != 2 {
		t.Errorf("expected selectedIdx=2 (stay at end), got %d", result.selectedIdx)
	}
}

func TestTreeNavigation_UpKey(t *testing.T) {
	m := newTreeTestModelWithContainers(t, 3)
	m.selectedIdx = 2

	msg := tea.KeyMsg{Type: tea.KeyUp}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.selectedIdx != 1 {
		t.Errorf("expected selectedIdx=1, got %d", result.selectedIdx)
	}
}

func TestTreeNavigation_UpKey_StopsAtStart(t *testing.T) {
	m := newTreeTestModelWithContainers(t, 2)
	m.selectedIdx = 0

	msg := tea.KeyMsg{Type: tea.KeyUp}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	// Should stay at first item
	if result.selectedIdx != 0 {
		t.Errorf("expected selectedIdx=0 (stay at start), got %d", result.selectedIdx)
	}
}

func TestTreeNavigation_EnterExpandsContainer(t *testing.T) {
	m := newTreeTestModelWithContainers(t, 2)
	m.selectedIdx = 1 // First container (after All, collapsed)
	initialItems := len(m.treeItems)

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	containerID := result.treeItems[1].ContainerID
	if !result.expandedContainers[containerID] {
		t.Error("container should be expanded after Enter")
	}
	if len(result.treeItems) <= initialItems {
		t.Errorf("tree should have more items after expanding: had %d, now %d", initialItems, len(result.treeItems))
	}
}

func TestTreeNavigation_EnterCollapsesExpandedContainer(t *testing.T) {
	m := newTreeTestModelWithContainers(t, 1)
	m.expandedContainers["c1"] = true
	m.rebuildTreeItems()
	m.selectedIdx = 1 // Select the expanded container (after All)

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.expandedContainers["c1"] {
		t.Error("container should be collapsed after Enter on expanded container")
	}
}

func TestTreeNavigation_RightOpensDetailPanel(t *testing.T) {
	m := newTreeTestModelWithContainers(t, 1)
	m.detailPanelOpen = false
	m.selectedIdx = 0

	msg := tea.KeyMsg{Type: tea.KeyRight}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if !result.detailPanelOpen {
		t.Error("detail panel should open on right arrow")
	}
}

func TestTreeNavigation_LeftClosesDetailPanel(t *testing.T) {
	m := newTreeTestModelWithContainers(t, 1)
	m.detailPanelOpen = true
	m.selectedIdx = 0

	msg := tea.KeyMsg{Type: tea.KeyLeft}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.detailPanelOpen {
		t.Error("detail panel should close on left arrow")
	}
}

func TestTreeNavigation_EscapeClosesDetailPanel(t *testing.T) {
	m := newTreeTestModelWithContainers(t, 1)
	m.detailPanelOpen = true
	m.selectedIdx = 0

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.detailPanelOpen {
		t.Error("detail panel should close on Escape")
	}
}

func TestTreeNavigation_SelectionSyncsSelectedContainer(t *testing.T) {
	m := newTreeTestModelWithContainers(t, 2)
	m.selectedIdx = 1 // First container (after All)

	// Move down to second container
	msg := tea.KeyMsg{Type: tea.KeyDown}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	// selectedContainer should be set to the container at selectedIdx
	if result.selectedContainer == nil {
		t.Fatal("selectedContainer should be set after navigation")
	}
	if result.selectedContainer.ID != "c2" {
		t.Errorf("selectedContainer.ID = %q, want 'c2'", result.selectedContainer.ID)
	}
}

func TestTreeNavigation_SelectionOnSessionSyncsSession(t *testing.T) {
	m := newTreeTestModelWithContainers(t, 1)
	m.expandedContainers["c1"] = true
	m.rebuildTreeItems()
	m.selectedIdx = 2 // First session under c1 (All=0, c1=1, dev=2)

	// Trigger sync by navigating down to second session (test=3)
	msg := tea.KeyMsg{Type: tea.KeyDown}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	// Should have container selected
	if result.selectedContainer == nil {
		t.Fatal("selectedContainer should be set")
	}
	// selectedSessionIdx should reflect the second session
	if result.selectedSessionIdx != 1 {
		t.Errorf("selectedSessionIdx = %d, want 1", result.selectedSessionIdx)
	}
}

// Tree Rendering Tests

func TestRenderTree_ShowsContainers(t *testing.T) {
	m := newTreeTestModel(t)
	c := &container.Container{
		ID:     "c1",
		Name:   "my-container",
		State: container.StateRunning,
	}
	m.containerList.SetItems([]list.Item{containerItem{container: c}})
	m.rebuildTreeItems()

	layout := ComputeLayout(80, 24, false, false)
	result := m.renderTree(layout)

	if !strings.Contains(result, "my-container") {
		t.Errorf("tree should show container name, got: %s", result)
	}
	if !strings.Contains(result, "▸") {
		t.Errorf("collapsed container should show ▸ indicator, got: %s", result)
	}
}

func TestRenderTree_ShowsExpandedContainerWithSessions(t *testing.T) {
	m := newTreeTestModel(t)
	c := &container.Container{
		ID:   "c1",
		Name: "my-container",
		Sessions: []container.Session{
			{Name: "dev", ContainerID: "c1"},
			{Name: "test", ContainerID: "c1"},
		},
	}
	m.containerList.SetItems([]list.Item{containerItem{container: c}})
	m.expandedContainers = map[string]bool{"c1": true}
	m.rebuildTreeItems()

	layout := ComputeLayout(80, 24, false, false)
	result := m.renderTree(layout)

	if !strings.Contains(result, "▾") {
		t.Errorf("expanded container should show ▾ indicator, got: %s", result)
	}
	if !strings.Contains(result, "dev") {
		t.Errorf("expanded container should show sessions, got: %s", result)
	}
	if !strings.Contains(result, "test") {
		t.Errorf("expanded container should show all sessions, got: %s", result)
	}
}

func TestRenderTree_HighlightsSelectedItem(t *testing.T) {
	m := newTreeTestModel(t)
	c1 := &container.Container{ID: "c1", Name: "container-1"}
	c2 := &container.Container{ID: "c2", Name: "container-2"}
	m.containerList.SetItems([]list.Item{
		containerItem{container: c1},
		containerItem{container: c2},
	})
	m.rebuildTreeItems()
	m.selectedIdx = 2 // Second container selected (after All + first container)

	layout := ComputeLayout(80, 24, false, false)
	result := m.renderTree(layout)

	// Selected item should have cursor indicator (>)
	// The output should show container-2 with some form of highlight
	if !strings.Contains(result, "container-2") {
		t.Errorf("should show container-2, got: %s", result)
	}
	// Check for cursor indicator near container-2
	if !strings.Contains(result, ">") {
		t.Errorf("selected item should have cursor indicator, got: %s", result)
	}
}

func TestRenderTree_ShowsContainerState(t *testing.T) {
	m := newTreeTestModel(t)
	c := &container.Container{
		ID:    "c1",
		Name:  "my-container",
		State: container.StateRunning,
	}
	m.containerList.SetItems([]list.Item{containerItem{container: c}})
	m.rebuildTreeItems()

	layout := ComputeLayout(80, 24, false, false)
	result := m.renderTree(layout)

	// Should show running state (or indicator of it)
	if !strings.Contains(result, "running") && !strings.Contains(result, "●") {
		t.Errorf("tree should show container state indicator, got: %s", result)
	}
}

func TestRenderTree_EmptyList(t *testing.T) {
	m := newTreeTestModel(t)
	m.containerList.SetItems([]list.Item{})
	m.rebuildTreeItems()

	layout := ComputeLayout(80, 24, false, false)
	result := m.renderTree(layout)

	// Should show some indication of empty state
	if result == "" {
		t.Error("tree should render something even when empty")
	}
}

// Detail Panel Rendering Tests

func TestRenderDetailPanel_Container(t *testing.T) {
	m := newTreeTestModel(t)
	c := &container.Container{
		ID:          "c1",
		Name:        "my-container",
		State:       container.StateRunning,
		ProjectPath: "/path/to/project",
		Template:    "go-project",
		Sessions: []container.Session{
			{Name: "dev", ContainerID: "c1"},
			{Name: "test", ContainerID: "c1"},
		},
	}
	m.containerList.SetItems([]list.Item{containerItem{container: c}})
	m.rebuildTreeItems()
	m.selectedIdx = 1 // Container (after All)
	m.detailPanelOpen = true
	m.syncSelectionFromTree()

	layout := ComputeLayout(100, 40, false, true)
	result := m.renderDetailPanel(layout)

	if !strings.Contains(result, "my-container") {
		t.Errorf("should show container name, got: %s", result)
	}
	if !strings.Contains(result, "running") {
		t.Errorf("should show container state, got: %s", result)
	}
	if !strings.Contains(result, "/path/to/project") {
		t.Errorf("should show project path, got: %s", result)
	}
	if !strings.Contains(result, "go-project") {
		t.Errorf("should show template, got: %s", result)
	}
}

func TestRenderDetailPanel_Session(t *testing.T) {
	m := newTreeTestModel(t)
	c := &container.Container{
		ID:   "c1",
		Name: "my-container",
		Sessions: []container.Session{
			{Name: "dev", ContainerID: "c1", Attached: true, Windows: 3},
		},
	}
	m.containerList.SetItems([]list.Item{containerItem{container: c}})
	m.expandedContainers = map[string]bool{"c1": true}
	m.rebuildTreeItems()
	m.selectedIdx = 2 // Session item (All=0, c1=1, dev=2)
	m.detailPanelOpen = true
	m.syncSelectionFromTree()

	layout := ComputeLayout(100, 40, false, true)
	result := m.renderDetailPanel(layout)

	if !strings.Contains(result, "dev") {
		t.Errorf("should show session name, got: %s", result)
	}
	if !strings.Contains(result, "Attached") || !strings.Contains(result, "Yes") {
		t.Errorf("should show attached status, got: %s", result)
	}
	if !strings.Contains(result, "3") {
		t.Errorf("should show window count, got: %s", result)
	}
}

func TestRenderDetailPanel_ShowsContainerForSession(t *testing.T) {
	m := newTreeTestModel(t)
	c := &container.Container{
		ID:   "c1",
		Name: "my-container",
		Sessions: []container.Session{
			{Name: "dev", ContainerID: "c1"},
		},
	}
	m.containerList.SetItems([]list.Item{containerItem{container: c}})
	m.expandedContainers = map[string]bool{"c1": true}
	m.rebuildTreeItems()
	m.selectedIdx = 2 // Session item (All=0, c1=1, dev=2)
	m.detailPanelOpen = true
	m.syncSelectionFromTree()

	layout := ComputeLayout(100, 40, false, true)
	result := m.renderDetailPanel(layout)

	// Should show which container the session belongs to
	if !strings.Contains(result, "my-container") {
		t.Errorf("should show parent container name, got: %s", result)
	}
}

func TestRenderDetailPanel_Empty(t *testing.T) {
	m := newTreeTestModel(t)
	m.containerList.SetItems([]list.Item{})
	m.rebuildTreeItems()
	m.detailPanelOpen = true

	layout := ComputeLayout(100, 40, false, true)
	result := m.renderDetailPanel(layout)

	// Should show something even with nothing selected
	if result == "" {
		t.Error("detail panel should render something even when empty")
	}
}

// New tests for TreeItemAll

func TestTreeItemAll_IsAll(t *testing.T) {
	item := TreeItem{Type: TreeItemAll}
	if !item.IsAll() {
		t.Error("IsAll should return true for All items")
	}
	if item.IsContainer() {
		t.Error("IsContainer should return false for All items")
	}
	if item.IsSession() {
		t.Error("IsSession should return false for All items")
	}
}

func TestRebuildTreeItems_AlwaysHasAll(t *testing.T) {
	m := newTreeTestModel(t)

	// With no containers
	m.containerList.SetItems([]list.Item{})
	m.rebuildTreeItems()
	if len(m.treeItems) < 1 || m.treeItems[0].Type != TreeItemAll {
		t.Error("tree should always start with All item")
	}

	// With containers
	m.containerList.SetItems([]list.Item{
		containerItem{container: &container.Container{ID: "c1", Name: "c1"}},
	})
	m.rebuildTreeItems()
	if m.treeItems[0].Type != TreeItemAll {
		t.Error("tree should always start with All item even with containers")
	}
}

func TestSyncSelection_AllContainersNilsSelectedContainer(t *testing.T) {
	m := newTreeTestModel(t)
	c := &container.Container{ID: "c1", Name: "test", State: container.StateRunning}
	m.containerList.SetItems([]list.Item{containerItem{container: c}})
	m.rebuildTreeItems()

	// Select a container first
	m.selectedIdx = 1
	m.syncSelectionFromTree()
	if m.selectedContainer == nil {
		t.Fatal("selectedContainer should be set when container is selected")
	}

	// Select All
	m.selectedIdx = 0
	m.syncSelectionFromTree()
	if m.selectedContainer != nil {
		t.Error("selectedContainer should be nil when All is selected")
	}
	if m.logFilter != "" {
		t.Errorf("logFilter should be empty when All is selected, got %q", m.logFilter)
	}
}

func TestContainerAction_NoOpWhenAllSelected(t *testing.T) {
	m := newTreeTestModel(t)
	c := &container.Container{ID: "c1", Name: "test", State: container.StateStopped}
	m.containerList.SetItems([]list.Item{containerItem{container: c}})
	m.rebuildTreeItems()
	m.selectedIdx = 0 // All selected
	m.syncSelectionFromTree()

	// Press 's' - should be no-op
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}
	updated, _ := m.Update(msg)
	result := updated.(Model)

	if result.statusLevel == StatusLoading {
		t.Error("s key should be no-op when All is selected")
	}

	// Press 'x' - should be no-op
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")}
	updated, _ = m.Update(msg)
	result = updated.(Model)

	if result.statusLevel == StatusLoading {
		t.Error("x key should be no-op when All is selected")
	}

	// Press 'd' - should be no-op
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")}
	updated, _ = m.Update(msg)
	result = updated.(Model)

	if result.statusLevel == StatusLoading {
		t.Error("d key should be no-op when All is selected")
	}

	// Press 't' - should be no-op
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")}
	updated, _ = m.Update(msg)
	result = updated.(Model)

	if result.sessionFormOpen {
		t.Error("t key should be no-op when All is selected")
	}
}

func TestRenderAllContainersDetailContent(t *testing.T) {
	m := newTreeTestModel(t)
	containers := []*container.Container{
		{ID: "c1", Name: "running-1", State: container.StateRunning, Sessions: []container.Session{
			{Name: "dev", ContainerID: "c1"},
		}},
		{ID: "c2", Name: "stopped-1", State: container.StateStopped},
		{ID: "c3", Name: "running-2", State: container.StateRunning},
	}
	var items []list.Item
	for _, c := range containers {
		items = append(items, containerItem{container: c})
	}
	m.containerList.SetItems(items)
	m.rebuildTreeItems()
	m.selectedIdx = 0 // All
	m.detailPanelOpen = true

	layout := ComputeLayout(100, 40, false, true)
	result := m.renderDetailPanel(layout)

	if !strings.Contains(result, "3 containers") {
		t.Errorf("should show total container count, got: %s", result)
	}
	if !strings.Contains(result, "Running:  2") {
		t.Errorf("should show running count, got: %s", result)
	}
	if !strings.Contains(result, "Stopped:  1") {
		t.Errorf("should show stopped count, got: %s", result)
	}
	if !strings.Contains(result, "Sessions: 1") {
		t.Errorf("should show total session count, got: %s", result)
	}
}

func TestRenderTree_ShowsAllContainersRow(t *testing.T) {
	m := newTreeTestModel(t)
	c := &container.Container{ID: "c1", Name: "test-container", State: container.StateRunning}
	m.containerList.SetItems([]list.Item{containerItem{container: c}})
	m.rebuildTreeItems()
	m.selectedIdx = 0

	layout := ComputeLayout(80, 24, false, false)
	result := m.renderTree(layout)

	if !strings.Contains(result, "All Containers") {
		t.Errorf("tree should show All Containers row, got: %s", result)
	}
	if !strings.Contains(result, "(1)") {
		t.Errorf("All Containers should show count, got: %s", result)
	}
}

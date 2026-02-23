package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/config"
	"devagent/internal/container"
	"devagent/internal/discovery"
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
	if m.treeItems[0].Type != TreeItemAllProjects {
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
	if m.treeItems[0].Type != TreeItemAllProjects {
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
	if m.treeItems[0].Type != TreeItemAllProjects {
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
	if m.treeItems[0].Type != TreeItemAllProjects {
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
		ID:    "c1",
		Name:  "my-container",
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

// New tests for TreeItemAllProjects

func TestTreeItemAllProjects_IsAll(t *testing.T) {
	item := TreeItem{Type: TreeItemAllProjects}
	if !item.IsAllProjects() {
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
	if len(m.treeItems) < 1 || m.treeItems[0].Type != TreeItemAllProjects {
		t.Error("tree should always start with All item")
	}

	// With containers
	m.containerList.SetItems([]list.Item{
		containerItem{container: &container.Container{ID: "c1", Name: "c1"}},
	})
	m.rebuildTreeItems()
	if m.treeItems[0].Type != TreeItemAllProjects {
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

	if !strings.Contains(result, "Containers: 3") {
		t.Errorf("should show total container count, got: %s", result)
	}
	if !strings.Contains(result, "Running:    2") {
		t.Errorf("should show running count, got: %s", result)
	}
	if !strings.Contains(result, "Stopped:    1") {
		t.Errorf("should show stopped count, got: %s", result)
	}
	if !strings.Contains(result, "Sessions:   1") {
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

// Tests for project-grouped tree structure (Phase 3)

func TestRebuildTreeItems_NoProjectsFallbackToFlatContainers(t *testing.T) {
	m := newTreeTestModel(t)

	// Set up containers without any discovered projects
	c1 := &container.Container{ID: "c1", Name: "container-1", ProjectPath: "/path1"}
	c2 := &container.Container{ID: "c2", Name: "container-2", ProjectPath: "/path2"}
	items := []list.Item{
		containerItem{container: c1},
		containerItem{container: c2},
	}
	m.containerList.SetItems(items)
	m.discoveredProjects = nil // No projects

	m.rebuildTreeItems()

	// Should have: All + 2 containers
	if len(m.treeItems) != 3 {
		t.Errorf("expected 3 items (All + 2 containers), got %d", len(m.treeItems))
	}
	if m.treeItems[0].Type != TreeItemAllProjects {
		t.Error("first item should be All")
	}
	if m.treeItems[1].Type != TreeItemContainer || m.treeItems[1].ContainerID != "c1" {
		t.Error("second item should be container c1")
	}
	if m.treeItems[2].Type != TreeItemContainer || m.treeItems[2].ContainerID != "c2" {
		t.Error("third item should be container c2")
	}
}

func TestRebuildTreeItems_WithProjectsCreatesProjectGroups(t *testing.T) {
	m := newTreeTestModel(t)

	// Set up discovered projects
	project1 := discovery.DiscoveredProject{
		Name: "project-1",
		Path: "/projects/proj1",
		Worktrees: []discovery.Worktree{
			{Name: "feature-x", Path: "/projects/proj1/feature-x", Branch: "feature-x"},
		},
	}
	project2 := discovery.DiscoveredProject{
		Name:      "project-2",
		Path:      "/projects/proj2",
		Worktrees: []discovery.Worktree{},
	}
	m.discoveredProjects = []discovery.DiscoveredProject{project1, project2}
	m.expandedProjects = make(map[string]bool)

	// Set up containers matching projects
	c1 := &container.Container{ID: "c1", Name: "container-1", ProjectPath: "/projects/proj1"}
	c2 := &container.Container{ID: "c2", Name: "container-2", ProjectPath: "/projects/proj1/feature-x"}
	items := []list.Item{
		containerItem{container: c1},
		containerItem{container: c2},
	}
	m.containerList.SetItems(items)

	// Expand first project to see its contents
	m.expandedProjects["/projects/proj1"] = true

	m.rebuildTreeItems()

	// Should have: All + project1 + main worktree + c1 + feature-x worktree + c2 + project2
	// = 1 (All) + 1 (project1) + 1 (main) + 1 (c1) + 1 (feature-x) + 1 (c2) + 1 (project2) = 7
	if len(m.treeItems) != 7 {
		t.Errorf("expected 7 items, got %d. Items: %v", len(m.treeItems), m.treeItems)
	}

	// Verify structure
	if m.treeItems[0].Type != TreeItemAllProjects {
		t.Error("first item should be All")
	}
	if m.treeItems[1].Type != TreeItemProject || m.treeItems[1].ProjectName != "project-1" {
		t.Error("second item should be project-1")
	}
	if m.treeItems[2].Type != TreeItemWorktree || m.treeItems[2].WorktreeName != "main" {
		t.Error("third item should be worktree 'main'")
	}
	if m.treeItems[3].Type != TreeItemContainer || m.treeItems[3].ContainerID != "c1" {
		t.Error("fourth item should be container c1")
	}
	if m.treeItems[4].Type != TreeItemWorktree || m.treeItems[4].WorktreeName != "feature-x" {
		t.Error("fifth item should be worktree 'feature-x'")
	}
	if m.treeItems[5].Type != TreeItemContainer || m.treeItems[5].ContainerID != "c2" {
		t.Error("sixth item should be container c2")
	}
	if m.treeItems[6].Type != TreeItemProject || m.treeItems[6].ProjectName != "project-2" {
		t.Error("seventh item should be project-2")
	}
}

func TestFindContainersForPath_MatchesContainersByProjectPath(t *testing.T) {
	m := newTreeTestModel(t)

	// Set up containers with different project paths
	c1 := &container.Container{ID: "c1", Name: "container-1", ProjectPath: "/projects/proj1"}
	c2 := &container.Container{ID: "c2", Name: "container-2", ProjectPath: "/projects/proj1"}
	c3 := &container.Container{ID: "c3", Name: "container-3", ProjectPath: "/projects/proj2"}
	items := []list.Item{
		containerItem{container: c1},
		containerItem{container: c2},
		containerItem{container: c3},
	}
	m.containerList.SetItems(items)

	// Find containers for project 1
	result := m.findContainersForPath("/projects/proj1")

	if len(result) != 2 {
		t.Errorf("expected 2 containers for /projects/proj1, got %d", len(result))
	}
	if result[0].ID != "c1" || result[1].ID != "c2" {
		t.Errorf("expected c1 and c2, got %v", []string{result[0].ID, result[1].ID})
	}

	// Find containers for project 2
	result = m.findContainersForPath("/projects/proj2")

	if len(result) != 1 {
		t.Errorf("expected 1 container for /projects/proj2, got %d", len(result))
	}
	if result[0].ID != "c3" {
		t.Errorf("expected c3, got %s", result[0].ID)
	}

	// Find containers for non-existent path
	result = m.findContainersForPath("/projects/nonexistent")

	if len(result) != 0 {
		t.Errorf("expected 0 containers for non-existent path, got %d", len(result))
	}
}

func TestRebuildTreeItems_OtherGroupCollectsUnmatchedContainers(t *testing.T) {
	m := newTreeTestModel(t)

	// Set up discovered projects
	project1 := discovery.DiscoveredProject{
		Name: "project-1",
		Path: "/projects/proj1",
	}
	m.discoveredProjects = []discovery.DiscoveredProject{project1}
	m.expandedProjects = make(map[string]bool)

	// Set up containers: one matching project, one unmatched
	c1 := &container.Container{ID: "c1", Name: "container-1", ProjectPath: "/projects/proj1"}
	c2 := &container.Container{ID: "c2", Name: "container-2", ProjectPath: "/other/path"}
	items := []list.Item{
		containerItem{container: c1},
		containerItem{container: c2},
	}
	m.containerList.SetItems(items)

	// Expand project to see matched container
	m.expandedProjects["/projects/proj1"] = true
	// Expand "Other" group to see unmatched container
	m.expandedProjects["__other__"] = true

	m.rebuildTreeItems()

	// Should have: All + project1 + main worktree + c1 + Other + c2
	// = 1 (All) + 1 (project) + 1 (main) + 1 (c1) + 1 (Other) + 1 (c2) = 6
	if len(m.treeItems) != 6 {
		t.Errorf("expected 6 items, got %d", len(m.treeItems))
	}

	// Find the "Other" item
	otherIdx := -1
	for i, item := range m.treeItems {
		if item.Type == TreeItemProject && item.ProjectName == "Other" {
			otherIdx = i
			break
		}
	}
	if otherIdx == -1 {
		t.Error("should have 'Other' group in tree")
	} else if m.treeItems[otherIdx+1].ContainerID != "c2" {
		t.Errorf("'Other' group should contain unmatched container c2, got %s", m.treeItems[otherIdx+1].ContainerID)
	}
}

func TestRebuildTreeItems_OtherGroupOmittedWhenAllContainersMatched(t *testing.T) {
	m := newTreeTestModel(t)

	// Set up discovered project
	project1 := discovery.DiscoveredProject{
		Name: "project-1",
		Path: "/projects/proj1",
	}
	m.discoveredProjects = []discovery.DiscoveredProject{project1}
	m.expandedProjects = make(map[string]bool)

	// Set up containers all matching project
	c1 := &container.Container{ID: "c1", Name: "container-1", ProjectPath: "/projects/proj1"}
	c2 := &container.Container{ID: "c2", Name: "container-2", ProjectPath: "/projects/proj1"}
	items := []list.Item{
		containerItem{container: c1},
		containerItem{container: c2},
	}
	m.containerList.SetItems(items)

	m.expandedProjects["/projects/proj1"] = true

	m.rebuildTreeItems()

	// Should not have "Other" group since all containers are matched
	for _, item := range m.treeItems {
		if item.Type == TreeItemProject && item.ProjectName == "Other" {
			t.Error("should not have 'Other' group when all containers are matched")
		}
	}
}

func TestRebuildTreeItems_ProjectExpansionToggle(t *testing.T) {
	m := newTreeTestModel(t)

	project1 := discovery.DiscoveredProject{
		Name: "project-1",
		Path: "/projects/proj1",
	}
	m.discoveredProjects = []discovery.DiscoveredProject{project1}
	m.expandedProjects = make(map[string]bool)

	c1 := &container.Container{ID: "c1", Name: "container-1", ProjectPath: "/projects/proj1"}
	items := []list.Item{containerItem{container: c1}}
	m.containerList.SetItems(items)

	// Initially collapsed
	m.expandedProjects["/projects/proj1"] = false
	m.rebuildTreeItems()

	// When project is collapsed, it doesn't claim its containers (they go to "Other")
	// Should have: All + project1 + Other + c1
	if len(m.treeItems) < 2 {
		t.Errorf("collapsed project should have at least 2 items (All + project), got %d", len(m.treeItems))
	}

	// Verify project item is marked as collapsed
	projectItem := m.treeItems[1]
	if projectItem.Expanded {
		t.Error("project item should have Expanded=false when collapsed")
	}

	// Expand project
	m.expandedProjects["/projects/proj1"] = true
	m.rebuildTreeItems()

	// Should have: All + project1 + main + c1
	if len(m.treeItems) < 3 {
		t.Errorf("expanded project should have more items, got %d", len(m.treeItems))
	}

	// Verify expansion flag is set
	projectItem = m.treeItems[1]
	if !projectItem.Expanded {
		t.Error("project item should have Expanded=true when expanded")
	}
}

func TestRebuildTreeItems_ContainerExpansionUnderWorktree(t *testing.T) {
	m := newTreeTestModel(t)

	project1 := discovery.DiscoveredProject{
		Name: "project-1",
		Path: "/projects/proj1",
		Worktrees: []discovery.Worktree{
			{Name: "feature-x", Path: "/projects/proj1/feature-x", Branch: "feature-x"},
		},
	}
	m.discoveredProjects = []discovery.DiscoveredProject{project1}
	m.expandedProjects = make(map[string]bool)
	m.expandedProjects["/projects/proj1"] = true

	// Container under worktree with session
	c1 := &container.Container{
		ID:          "c1",
		Name:        "container-1",
		ProjectPath: "/projects/proj1/feature-x",
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

	// Find the container item
	containerIdx := -1
	for i, item := range m.treeItems {
		if item.Type == TreeItemContainer && item.ContainerID == "c1" {
			containerIdx = i
			break
		}
	}
	if containerIdx == -1 {
		t.Fatal("container c1 not found in tree")
	}

	// Should have sessions after the container
	if containerIdx+1 >= len(m.treeItems) {
		t.Error("container should have sessions after it")
	}
	if m.treeItems[containerIdx+1].Type != TreeItemSession {
		t.Error("item after container should be a session")
	}
	if m.treeItems[containerIdx+1].SessionName != "dev" {
		t.Errorf("expected session 'dev', got %s", m.treeItems[containerIdx+1].SessionName)
	}
	if m.treeItems[containerIdx+2].SessionName != "test" {
		t.Errorf("expected session 'test', got %s", m.treeItems[containerIdx+2].SessionName)
	}
}

func TestToggleTreeExpand_ProjectExpansion(t *testing.T) {
	m := newTreeTestModel(t)

	project1 := discovery.DiscoveredProject{
		Name: "project-1",
		Path: "/projects/proj1",
	}
	m.discoveredProjects = []discovery.DiscoveredProject{project1}
	m.expandedProjects = make(map[string]bool)

	c1 := &container.Container{ID: "c1", Name: "container-1", ProjectPath: "/projects/proj1"}
	items := []list.Item{containerItem{container: c1}}
	m.containerList.SetItems(items)

	m.rebuildTreeItems()
	m.selectedIdx = 1 // Select the project item (after All)

	// Toggle expand
	m.toggleTreeExpand()

	// Project should now be expanded
	if !m.expandedProjects["/projects/proj1"] {
		t.Error("project should be expanded after toggle")
	}

	// Tree should be rebuilt
	projectItem := m.treeItems[1]
	if !projectItem.Expanded {
		t.Error("project item should have Expanded=true after toggle")
	}
}

func TestToggleTreeExpand_OtherGroupExpansion(t *testing.T) {
	m := newTreeTestModel(t)

	project1 := discovery.DiscoveredProject{
		Name: "project-1",
		Path: "/projects/proj1",
	}
	m.discoveredProjects = []discovery.DiscoveredProject{project1}
	m.expandedProjects = make(map[string]bool)

	// One matched, one unmatched
	c1 := &container.Container{ID: "c1", Name: "container-1", ProjectPath: "/projects/proj1"}
	c2 := &container.Container{ID: "c2", Name: "container-2", ProjectPath: "/other"}
	items := []list.Item{
		containerItem{container: c1},
		containerItem{container: c2},
	}
	m.containerList.SetItems(items)

	m.rebuildTreeItems()

	// Find the "Other" item
	otherIdx := -1
	for i, item := range m.treeItems {
		if item.Type == TreeItemProject && item.ProjectName == "Other" {
			otherIdx = i
			break
		}
	}
	if otherIdx == -1 {
		t.Fatal("'Other' group not found")
	}

	m.selectedIdx = otherIdx

	// Toggle expand
	m.toggleTreeExpand()

	// "Other" should now be expanded
	if !m.expandedProjects["__other__"] {
		t.Error("'Other' group should be expanded after toggle")
	}
}

func TestRebuildTreeItems_CollapsedProjectHidesChildren(t *testing.T) {
	m := newTreeTestModel(t)

	project1 := discovery.DiscoveredProject{
		Name: "project-1",
		Path: "/projects/proj1",
		Worktrees: []discovery.Worktree{
			{Name: "feature", Path: "/projects/proj1/feature", Branch: "feature"},
		},
	}
	m.discoveredProjects = []discovery.DiscoveredProject{project1}
	m.expandedProjects = make(map[string]bool)

	c1 := &container.Container{ID: "c1", Name: "container-1", ProjectPath: "/projects/proj1"}
	c2 := &container.Container{ID: "c2", Name: "container-2", ProjectPath: "/projects/proj1/feature"}
	items := []list.Item{
		containerItem{container: c1},
		containerItem{container: c2},
	}
	m.containerList.SetItems(items)

	// Project is collapsed
	m.expandedProjects["/projects/proj1"] = false

	m.rebuildTreeItems()

	// Collapsed projects don't claim their containers (they go to "Other" group)
	// Should have: All + project1 + Other + c1 + c2
	// (No worktrees or nested containers under project because it's collapsed)
	if len(m.treeItems) < 2 {
		t.Errorf("collapsed project should have at least All + project, got %d", len(m.treeItems))
	}

	// Verify no worktree items visible under the project
	hasWorktreeUnderProject := false
	projectIdx := -1
	for i, item := range m.treeItems {
		if item.Type == TreeItemProject && item.ProjectName == "project-1" {
			projectIdx = i
			break
		}
	}
	if projectIdx != -1 && projectIdx+1 < len(m.treeItems) {
		if m.treeItems[projectIdx+1].Type == TreeItemWorktree {
			hasWorktreeUnderProject = true
		}
	}
	if hasWorktreeUnderProject {
		t.Error("collapsed project should not have worktree items")
	}
}

func TestFindContainersForProject_IncludesMainAndWorktrees(t *testing.T) {
	m := newTreeTestModel(t)

	project1 := discovery.DiscoveredProject{
		Name: "project-1",
		Path: "/projects/proj1",
		Worktrees: []discovery.Worktree{
			{Name: "feature", Path: "/projects/proj1/feature", Branch: "feature"},
			{Name: "bugfix", Path: "/projects/proj1/bugfix", Branch: "bugfix"},
		},
	}

	// Containers in main and worktrees
	c1 := &container.Container{ID: "c1", Name: "container-1", ProjectPath: "/projects/proj1"}
	c2 := &container.Container{ID: "c2", Name: "container-2", ProjectPath: "/projects/proj1/feature"}
	c3 := &container.Container{ID: "c3", Name: "container-3", ProjectPath: "/projects/proj1/bugfix"}
	items := []list.Item{
		containerItem{container: c1},
		containerItem{container: c2},
		containerItem{container: c3},
	}
	m.containerList.SetItems(items)

	result := m.findContainersForProject(project1)

	if len(result) != 3 {
		t.Errorf("expected 3 containers for project, got %d", len(result))
	}

	// Verify all containers are included
	ids := make(map[string]bool)
	for _, c := range result {
		ids[c.ID] = true
	}
	if !ids["c1"] || !ids["c2"] || !ids["c3"] {
		t.Errorf("expected c1, c2, c3, got %v", ids)
	}
}

// Test for syncSelectionFromTree clearing selectedContainer for Project nodes
func TestSyncSelection_ProjectNodeClearsSelectedContainer(t *testing.T) {
	m := newTreeTestModel(t)

	// Set up a project and container
	project1 := discovery.DiscoveredProject{
		Name: "project-1",
		Path: "/projects/proj1",
	}
	m.discoveredProjects = []discovery.DiscoveredProject{project1}
	m.expandedProjects = make(map[string]bool)
	m.expandedProjects["/projects/proj1"] = true

	c1 := &container.Container{ID: "c1", Name: "container-1", ProjectPath: "/projects/proj1"}
	items := []list.Item{containerItem{container: c1}}
	m.containerList.SetItems(items)

	m.rebuildTreeItems()

	// First, select a container
	// Tree structure: All (0) + Project (1) + main worktree (2) + container (3)
	m.selectedIdx = 3 // Select container
	m.syncSelectionFromTree()

	if m.selectedContainer == nil {
		t.Fatal("selectedContainer should be set when container is selected")
	}
	if m.selectedContainer.ID != "c1" {
		t.Errorf("selectedContainer.ID = %q, want 'c1'", m.selectedContainer.ID)
	}

	// Now navigate to the project node
	m.selectedIdx = 1 // Select project node
	m.syncSelectionFromTree()

	// selectedContainer should be nil after navigating to project node
	if m.selectedContainer != nil {
		t.Errorf("selectedContainer should be nil when project node is selected, got %v", m.selectedContainer.ID)
	}
	if m.selectedSessionIdx != 0 {
		t.Errorf("selectedSessionIdx should be 0 when project node is selected, got %d", m.selectedSessionIdx)
	}
}

// Test for syncSelectionFromTree clearing selectedContainer for Worktree nodes
func TestSyncSelection_WorktreeNodeClearsSelectedContainer(t *testing.T) {
	m := newTreeTestModel(t)

	// Set up a project with worktree
	project1 := discovery.DiscoveredProject{
		Name: "project-1",
		Path: "/projects/proj1",
		Worktrees: []discovery.Worktree{
			{Name: "feature", Path: "/projects/proj1/feature", Branch: "feature"},
		},
	}
	m.discoveredProjects = []discovery.DiscoveredProject{project1}
	m.expandedProjects = make(map[string]bool)
	m.expandedProjects["/projects/proj1"] = true

	// Container under worktree
	c1 := &container.Container{ID: "c1", Name: "container-1", ProjectPath: "/projects/proj1/feature"}
	items := []list.Item{containerItem{container: c1}}
	m.containerList.SetItems(items)

	m.rebuildTreeItems()

	// First, select the container
	// Find the container in the tree
	containerIdx := -1
	for i, item := range m.treeItems {
		if item.Type == TreeItemContainer && item.ContainerID == "c1" {
			containerIdx = i
			break
		}
	}
	if containerIdx == -1 {
		t.Fatal("container c1 not found in tree")
	}

	m.selectedIdx = containerIdx
	m.syncSelectionFromTree()

	if m.selectedContainer == nil {
		t.Fatal("selectedContainer should be set when container is selected")
	}
	if m.selectedContainer.ID != "c1" {
		t.Errorf("selectedContainer.ID = %q, want 'c1'", m.selectedContainer.ID)
	}

	// Now navigate to the worktree node
	worktreeIdx := containerIdx - 1 // Worktree should be just before container
	if worktreeIdx < 0 || m.treeItems[worktreeIdx].Type != TreeItemWorktree {
		t.Fatal("worktree item not found at expected location")
	}

	m.selectedIdx = worktreeIdx
	m.syncSelectionFromTree()

	// selectedContainer should be nil after navigating to worktree node
	if m.selectedContainer != nil {
		t.Errorf("selectedContainer should be nil when worktree node is selected, got %v", m.selectedContainer.ID)
	}
	if m.selectedSessionIdx != 0 {
		t.Errorf("selectedSessionIdx should be 0 when worktree node is selected, got %d", m.selectedSessionIdx)
	}
}

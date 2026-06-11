package tui

import (
	"strings"
	"testing"
)

// TestContextualHelp_VSCodeHint verifies the status-bar help advertises the "v"
// VS Code shortcut exactly on the node types where it works (container and its
// sessions) and not on project/worktree nodes where it is a no-op.
func TestContextualHelp_VSCodeHint(t *testing.T) {
	tests := []struct {
		name     string
		item     TreeItem
		wantHint bool
	}{
		{"container shows v", TreeItem{Type: TreeItemContainer, ContainerID: "abc"}, true},
		{"session shows v", TreeItem{Type: TreeItemSession, ContainerID: "abc", SessionName: "dev"}, true},
		{"project omits v", TreeItem{Type: TreeItemProject, ProjectPath: "/p"}, false},
		{"worktree omits v", TreeItem{Type: TreeItemWorktree, ProjectPath: "/p", WorktreeName: "main"}, false},
		{"all-projects omits v", TreeItem{Type: TreeItemAllProjects}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTreeTestModel(t)
			m.panelFocus = FocusTree
			m.treeItems = []TreeItem{tt.item}
			m.selectedIdx = 0

			help := m.renderContextualHelp()
			if got := strings.Contains(help, "v: VS Code"); got != tt.wantHint {
				t.Errorf("help = %q; contains \"v: VS Code\" = %v, want %v", help, got, tt.wantHint)
			}
		})
	}
}

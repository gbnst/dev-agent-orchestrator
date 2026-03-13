package worktree

import (
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"feature-x", false},
		{"feature/new-model", false},
		{"fix_bug_123", false},
		{"v2.0", false},
		{"my-branch", false},
		{"", true},                       // empty
		{strings.Repeat("a", 101), true}, // too long
		{"-starts-with-hyphen", true},    // starts with non-alphanumeric
		{"has spaces", true},             // spaces
		{"has..dots", true},              // path traversal
		{"../escape", true},              // path traversal
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestWorktreeDir(t *testing.T) {
	dir := WorktreeDir("/home/user/project", "feature-x")
	expected := "/home/user/project/.worktrees/feature-x"
	if dir != expected {
		t.Errorf("WorktreeDir = %q, want %q", dir, expected)
	}
}

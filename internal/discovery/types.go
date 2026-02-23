// pattern: Functional Core

package discovery

// Worktree represents a git worktree for a project.
type Worktree struct {
	Name   string // Branch name or worktree directory name
	Path   string // Absolute path to the worktree directory
	Branch string // Git branch name
}

// DiscoveredProject represents a project found during directory scanning.
type DiscoveredProject struct {
	Name        string     // Directory name (used as display name)
	Path        string     // Absolute path to the project root (main worktree)
	Worktrees   []Worktree // Existing git worktrees (empty if none)
	HasMakefile bool       // Whether the project has a Makefile (for worktree-prep)
}

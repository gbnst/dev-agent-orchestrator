// pattern: Imperative Shell

package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// validNameRe matches valid worktree names: alphanumeric, hyphens, underscores, slashes.
var validNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]*$`)

// ValidateName checks if a worktree name is valid.
// Names must start with an alphanumeric character and contain only
// alphanumeric, hyphens, underscores, dots, and slashes.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}
	if len(name) > 100 {
		return fmt.Errorf("worktree name too long (max 100 characters)")
	}
	if !validNameRe.MatchString(name) {
		return fmt.Errorf("invalid worktree name %q: must start with alphanumeric, may contain a-z A-Z 0-9 . _ / -", name)
	}
	// Disallow ".." path traversal
	if strings.Contains(name, "..") {
		return fmt.Errorf("worktree name cannot contain '..'")
	}
	return nil
}

// WorktreeDir returns the path where a worktree would be created.
// Worktrees are stored in <project>/.worktrees/<name>/
func WorktreeDir(projectPath, name string) string {
	return filepath.Join(projectPath, ".worktrees", name)
}

// Create creates a new git worktree with a feature branch.
// Steps:
// 1. Validate name
// 2. git worktree add .worktrees/<name> -b <name>
// 3. Patch worktree's compose YAML with volume mounts
// 4. Run make worktree-prep if Makefile exists
//
// Returns the path to the created worktree directory.
func Create(projectPath, name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}

	wtDir := WorktreeDir(projectPath, name)

	// Check if worktree already exists
	if _, err := os.Stat(wtDir); err == nil {
		return "", fmt.Errorf("worktree %q already exists at %s", name, wtDir)
	}

	// Ensure .worktrees directory exists
	if err := os.MkdirAll(filepath.Join(projectPath, ".worktrees"), 0755); err != nil {
		return "", fmt.Errorf("creating .worktrees directory: %w", err)
	}

	// Create git worktree with a new branch
	cmd := exec.Command("git", "worktree", "add", wtDir, "-b", name)
	cmd.Dir = projectPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(output)), err)
	}

	// Patch compose YAML
	if err := PatchComposeForWorktree(projectPath, wtDir, name); err != nil {
		_ = removeWorktree(projectPath, wtDir)
		return "", fmt.Errorf("patching compose: %w", err)
	}

	// Run make worktree-prep if Makefile exists
	makefilePath := filepath.Join(wtDir, "Makefile")
	if _, err := os.Stat(makefilePath); err == nil {
		cmd := exec.Command("make", "worktree-prep")
		cmd.Dir = wtDir
		if _, err := cmd.CombinedOutput(); err != nil {
			// Non-fatal: worktree-prep is optional setup
		}
	}

	return wtDir, nil
}

// Destroy removes a worktree and its branch.
// Steps:
// 1. docker compose down (caller's responsibility — we just do git cleanup)
// 2. git worktree remove (without --force, refuses if dirty)
// 3. git branch -d (without -D, refuses if unmerged)
func Destroy(projectPath, name string) error {
	wtDir := WorktreeDir(projectPath, name)

	// Remove the git worktree (non-force: refuses if dirty)
	if err := removeWorktree(projectPath, wtDir); err != nil {
		return err
	}

	// Delete the branch (non-force: refuses if unmerged)
	cmd := exec.Command("git", "branch", "-d", name)
	cmd.Dir = projectPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git branch -d: %s: %w", strings.TrimSpace(string(output)), err)
	}

	return nil
}

// removeWorktree calls git worktree remove for cleanup.
func removeWorktree(projectPath, wtDir string) error {
	cmd := exec.Command("git", "worktree", "remove", wtDir)
	cmd.Dir = projectPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

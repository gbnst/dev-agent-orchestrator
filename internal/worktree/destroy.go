// pattern: Imperative Shell

package worktree

import (
	"context"
	"fmt"

	"devagent/internal/container"
)

// ContainerOps abstracts container operations for testability.
type ContainerOps interface {
	List() []*container.Container
	StopWithCompose(ctx context.Context, containerID string) error
	DestroyWithCompose(ctx context.Context, containerID string) error
}

// WorktreeOps abstracts worktree operations for testability.
type WorktreeOps interface {
	WorktreeDir(projectPath, name string) string
	Destroy(projectPath, name string) error
}

// DestroyWorktreeWithContainer performs a compound operation:
// 1. Find container for the worktree (by project path + worktree name)
// 2. If container exists and is running: stop it
// 3. If container exists: destroy it (compose down)
// 4. Remove git worktree
//
// This ensures both TUI and Web use identical semantics for worktree deletion.
// If wtOps is nil, uses the real worktree package functions.
func DestroyWorktreeWithContainer(
	ctx context.Context,
	containerOps ContainerOps,
	projectPath string,
	name string,
	wtOps WorktreeOps,
) error {
	// Use provided worktreeOps or fall back to real functions
	var wtDir string
	if wtOps != nil {
		wtDir = wtOps.WorktreeDir(projectPath, name)
	} else {
		wtDir = WorktreeDir(projectPath, name)
	}

	// Find container for this worktree path
	containers := containerOps.List()
	for _, c := range containers {
		if c.ProjectPath == wtDir {
			// Stop if running
			if c.IsRunning() {
				if err := containerOps.StopWithCompose(ctx, c.ID); err != nil {
					return fmt.Errorf("failed to stop container: %w", err)
				}
			}
			// Destroy container
			if err := containerOps.DestroyWithCompose(ctx, c.ID); err != nil {
				return fmt.Errorf("failed to destroy container: %w", err)
			}
			break
		}
	}

	// Remove git worktree and branch
	if wtOps != nil {
		return wtOps.Destroy(projectPath, name)
	}
	return Destroy(projectPath, name)
}

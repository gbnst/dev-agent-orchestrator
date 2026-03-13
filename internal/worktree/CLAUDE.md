# Worktree Domain

Last verified: 2026-03-13

## Purpose
Manages git worktree lifecycle for parallel feature development. Creates worktrees with feature branches, writes git overlay files for container compatibility, and runs project-specific setup hooks. Provides compound operations to align worktree deletion semantics between TUI and Web.

## Contracts
- **Exposes**: `Create()`, `Destroy()`, `ValidateName()`, `WorktreeDir()`, `WriteGitOverlays()`, `GitOverlayContent()`, `DestroyWorktreeWithContainer()`, `ContainerOps` (interface), `WorktreeOps` (interface)
- **Guarantees**: Host .git files never modified (overlay mount strategy). Name validation prevents path traversal. Destroy uses non-force git variants (refuses dirty worktrees and unmerged branches). Compose project name sanitized for isolation. DestroyWorktreeWithContainer performs atomic compound operation: stop container (if running) -> destroy container -> git worktree remove, ensuring consistent semantics across TUI and Web.
- **Expects**: Git binary available. Project has .devcontainer/docker-compose.yml. DestroyWorktreeWithContainer requires ContainerOps implementation (e.g., container.Manager) and optional WorktreeOps for testability.

## Dependencies
- **Uses**: os/exec (git, make), encoding/json, container.Container (for DestroyWorktreeWithContainer)
- **Used by**: TUI (via worktree actions, worktree destroy command), Web (via worktree delete endpoint), container.Manager (indirectly via DestroyWorktreeWithContainer)
- **Boundary**: Git and filesystem operations; container lifecycle operations abstracted behind ContainerOps interface

## Key Decisions
- Git overlay mount strategy: container mounts override worktree's .git file and gitdir back-reference without touching host files
- Non-force destroy: git built-in safety prevents data loss (uncommitted changes, unmerged branches)
- make worktree-prep: optional hook for project-specific setup (non-fatal if it fails)
- Compose project name: sanitized to lowercase-alphanumeric-hyphens for Docker Compose compatibility. Worktree containers are created with compose project name derived from `SanitizeComposeName(projectBaseName + "-" + worktreeName)` at container creation time (not at worktree creation time).
- DestroyWorktreeWithContainer: compound operation to align TUI and Web deletion semantics. Finds container by compose project name (not path matching). Accepts ContainerOps interface (container.Manager satisfies it) for flexible testing. Optional WorktreeOps parameter allows test mocking; if nil, uses real worktree functions.

## Key Files
- `worktree.go` - Create/Destroy orchestration, name validation (Imperative Shell)
- `destroy.go` - Compound DestroyWorktreeWithContainer operation with container lifecycle integration (Imperative Shell)
- `git.go` - Git overlay file writing and content generation (Imperative Shell)

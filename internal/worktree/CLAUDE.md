# Worktree Domain

Last verified: 2026-02-23

## Purpose
Manages git worktree lifecycle for parallel feature development. Creates worktrees with feature branches, writes git overlay files for container compatibility, writes compose override files for volume mounts, and runs project-specific setup hooks.

## Contracts
- **Exposes**: `Create()`, `Destroy()`, `ValidateName()`, `WorktreeDir()`, `WriteGitOverlays()`, `GitOverlayContent()`, `WriteComposeOverride()`
- **Guarantees**: Host .git files never modified (overlay mount strategy). Name validation prevents path traversal. Destroy uses non-force git variants (refuses dirty worktrees and unmerged branches). Original docker-compose.yml never modified (override file strategy). Compose project name sanitized for isolation.
- **Expects**: Git binary available. Project has .devcontainer/docker-compose.yml. Project's devcontainer.json must list `docker-compose.worktree.yml` in its `dockerComposeFile` array (templates do this by default). Caller handles container lifecycle (compose up/down) before/after worktree operations.

## Dependencies
- **Uses**: os/exec (git, make), encoding/json
- **Used by**: TUI (via worktree actions), container.Manager (future integration)
- **Boundary**: Git and filesystem operations only; no container runtime interaction

## Key Decisions
- Git overlay mount strategy: container mounts override worktree's .git file and gitdir back-reference without touching host files
- Non-force destroy: git built-in safety prevents data loss (uncommitted changes, unmerged branches)
- make worktree-prep: optional hook for project-specific setup (non-fatal if it fails)
- Compose project name: sanitized to lowercase-alphanumeric-hyphens for Docker Compose compatibility
- Compose override file strategy: writes docker-compose.worktree.yml instead of patching docker-compose.yml in-place. Projects must pre-configure devcontainer.json with dockerComposeFile array (templates include this).

## Key Files
- `worktree.go` - Create/Destroy orchestration, name validation (Imperative Shell)
- `git.go` - Git overlay file writing and content generation (Imperative Shell)
- `compose.go` - Compose override file writing (Imperative Shell)

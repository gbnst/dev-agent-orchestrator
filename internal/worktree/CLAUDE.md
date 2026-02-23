# Worktree Domain

Last verified: 2026-02-23

## Purpose
Manages git worktree lifecycle for parallel feature development. Creates worktrees with feature branches, writes git overlay files for container compatibility, patches compose YAML with volume mounts, and runs project-specific setup hooks.

## Contracts
- **Exposes**: `Create()`, `Destroy()`, `ValidateName()`, `WorktreeDir()`, `WriteGitOverlays()`, `GitOverlayContent()`, `PatchComposeForWorktree()`
- **Guarantees**: Host .git files never modified (overlay mount strategy). Name validation prevents path traversal. Destroy uses non-force git variants (refuses dirty worktrees and unmerged branches). Compose YAML patched with unique project name for isolation.
- **Expects**: Git binary available. Project has .devcontainer/docker-compose.yml. Caller handles container lifecycle (compose up/down) before/after worktree operations.

## Dependencies
- **Uses**: os/exec (git, make), gopkg.in/yaml.v3
- **Used by**: TUI (via worktree actions), container.Manager (future integration)
- **Boundary**: Git and filesystem operations only; no container runtime interaction

## Key Decisions
- Git overlay mount strategy: container mounts override worktree's .git file and gitdir back-reference without touching host files
- Non-force destroy: git built-in safety prevents data loss (uncommitted changes, unmerged branches)
- make worktree-prep: optional hook for project-specific setup (non-fatal if it fails)
- Compose project name: sanitized to lowercase-alphanumeric-hyphens for Docker Compose compatibility

## Key Files
- `worktree.go` - Create/Destroy orchestration, name validation (Imperative Shell)
- `git.go` - Git overlay file writing and content generation (Imperative Shell)
- `compose.go` - Compose YAML patching with yaml.v3 node manipulation (Imperative Shell)

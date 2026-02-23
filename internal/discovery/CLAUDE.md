# Discovery Domain

Last verified: 2026-02-23

## Purpose
Scans configured directories to discover devagent-managed projects on disk. Detects existing git worktrees for each project.

## Contracts
- **Exposes**: `Scanner`, `NewScanner()`, `DiscoveredProject`, `Worktree`
- **Guarantees**: Walks scan paths one level deep. Projects identified by `.devcontainer/docker-compose.yml` with `devagent.managed: "true"` label. Symlinks resolved and deduplicated. Missing directories silently skipped. Git worktrees detected via `git worktree list --porcelain`.
- **Expects**: Valid directory paths. Git binary available for worktree detection (graceful degradation if missing).

## Dependencies
- **Uses**: gopkg.in/yaml.v3, os/exec (git)
- **Used by**: main.go, TUI (via Model.discoveredProjects)
- **Boundary**: Read-only scanning; no project modification

## Key Files
- `types.go` - DiscoveredProject, Worktree types (Functional Core)
- `scanner.go` - Scanner with ScanAll, compose label checking, worktree listing (Imperative Shell)

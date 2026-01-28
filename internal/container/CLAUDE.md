# Container Domain

Last verified: 2026-01-28

## Purpose
Orchestrates devcontainer lifecycle: creation via @devcontainers/cli, start/stop/destroy via Docker or Podman, and tmux session management within containers.

## Contracts
- **Exposes**: `Manager`, `Container`, `Session`, `CreateOptions`, `ContainerState`, `RuntimeInterface`, `DevcontainerJSON`
- **Guarantees**: Auto-detects Docker/Podman from config. Operations are idempotent (stop already-stopped is safe). Labels track devagent metadata.
- **Expects**: Container runtime available. Valid config for Create operations. Refresh() called before List().

## Dependencies
- **Uses**: config.Config, logging.Manager (optional), @devcontainers/cli (external)
- **Used by**: TUI (Model), main.go
- **Boundary**: Container operations only; no UI concerns

## Key Decisions
- RuntimeInterface abstraction: Enables mock testing without real containers
- Devcontainer CLI for creation: Handles complex setup (features, mounts, env)
- Labels for metadata: devagent.managed, devagent.project_path, devagent.template

## Invariants
- containers map updated only via Refresh() or after Create/Destroy
- State transitions: created -> running <-> stopped -> (destroyed)
- Manager methods are nil-safe for logger (NopLogger default)

## Key Files
- `manager.go` - Manager struct, lifecycle operations, session management
- `runtime.go` - RuntimeInterface impl for Docker/Podman CLI
- `devcontainer.go` - DevcontainerGenerator, DevcontainerCLI wrappers
- `types.go` - Container, Session, CreateOptions, state constants

## Gotchas
- Container IDs may be truncated; Create() does prefix matching on refresh
- Session is duplicated from tmux package to avoid import cycles
- RuntimePath() returns full binary path to bypass shell aliases

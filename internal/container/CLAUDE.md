# Container Domain

Last verified: 2026-02-01

## Purpose
Orchestrates devcontainer lifecycle: creation via @devcontainers/cli, start/stop/destroy via Docker or Podman, and tmux session management within containers. Manages per-container Claude Code configuration including auth token injection and persistent settings.

## Contracts
- **Exposes**: `Manager`, `Container`, `Session`, `CreateOptions`, `ContainerState`, `RuntimeInterface`, `DevcontainerJSON`
- **Guarantees**: Auto-detects Docker/Podman from config. Operations are idempotent (stop already-stopped is safe). Labels track devagent metadata. Claude config directories persist across container recreations.
- **Expects**: Container runtime available. Valid config for Create operations. Refresh() called before List().

## Dependencies
- **Uses**: config.Config, logging.Manager (optional), @devcontainers/cli (external)
- **Used by**: TUI (Model), main.go
- **Boundary**: Container operations only; no UI concerns

## Key Decisions
- RuntimeInterface abstraction: Enables mock testing without real containers
- Devcontainer CLI for creation: Handles complex setup (features, mounts, env)
- Labels for metadata: devagent.managed, devagent.project_path, devagent.template, devagent.remote_user
- RemoteUser defaults to "vscode" per devcontainer spec; all exec operations use ExecAs with this user
- Per-project Claude config: Each container gets a unique .claude directory (hashed by project path) mounted from ~/.local/share/devagent/claude-configs/
- Auth token injection: Reads ~/.claude/create-auth-token and injects as CLAUDE_CODE_OAUTH_TOKEN via containerEnv
- Template claude files: Copied from template's home/vscode/.claude/ to container's claude dir on creation (won't overwrite existing)

## Invariants
- containers map updated only via Refresh() or after Create/Destroy
- State transitions: created -> running <-> stopped -> (destroyed)
- Manager methods are nil-safe for logger (NopLogger default)
- Claude config directories are never deleted (persist user customizations)

## Key Files
- `manager.go` - Manager struct, lifecycle operations, session management
- `runtime.go` - RuntimeInterface impl for Docker/Podman CLI, ExecAs for user-specific commands
- `devcontainer.go` - DevcontainerGenerator, DevcontainerCLI, Claude config management
- `types.go` - Container, Session, CreateOptions, state constants

## Gotchas
- Container IDs may be truncated; Create() does prefix matching on refresh
- Session is duplicated from tmux package to avoid import cycles
- RuntimePath() returns full binary path to bypass shell aliases
- Session.AttachCommand(runtime, user) requires both runtime and user parameters
- Claude auth token file must exist at ~/.claude/create-auth-token for auth injection to work

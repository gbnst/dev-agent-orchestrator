# Container Domain

Last verified: 2026-02-02

## Purpose
Orchestrates devcontainer lifecycle: creation via @devcontainers/cli, start/stop/destroy via Docker or Podman, and tmux session management within containers. Manages per-container Claude Code configuration including auth token injection and persistent settings. Provides network isolation via mitmproxy sidecars with domain allowlisting.

## Contracts
- **Exposes**: `Manager`, `Container`, `Session`, `Sidecar`, `CreateOptions`, `ProxyConfig`, `RunContainerOptions`, `ContainerState`, `RuntimeInterface`, `DevcontainerJSON`, `IsolationInfo`, `ProgressStep`, `ProgressCallback`
- **Guarantees**: Auto-detects Docker/Podman from config. Operations are idempotent (stop already-stopped is safe). Labels track devagent metadata. Claude config directories persist across container recreations. Sidecars are created before devcontainer and destroyed after. Proxy CA certs are auto-installed via postCreateCommand. Container creation reports progress via OnProgress callback. Isolation info can be queried from running containers.
- **Expects**: Container runtime available. Valid config for Create operations. Refresh() called before List(). mitmproxy image available for network isolation.

## Dependencies
- **Uses**: config.Config, config.IsolationConfig, config.DefaultIsolation, logging.Manager (optional), @devcontainers/cli (external), mitmproxy/mitmproxy (external image)
- **Used by**: TUI (Model), main.go
- **Boundary**: Container operations only; no UI concerns

## Key Decisions
- RuntimeInterface abstraction: Enables mock testing without real containers; includes network ops (CreateNetwork, RemoveNetwork, RunContainer)
- Devcontainer CLI for creation: Handles complex setup (features, mounts, env)
- Labels for metadata: devagent.managed, devagent.project_path, devagent.template, devagent.remote_user, devagent.sidecar_of, devagent.sidecar_type
- RemoteUser defaults to "vscode" per devcontainer spec; all exec operations use ExecAs with this user
- Per-project Claude config: Each container gets a unique .claude directory (hashed by project path) mounted from ~/.local/share/devagent/claude-configs/
- Auth token injection: Reads ~/.claude/create-auth-token and injects as CLAUDE_CODE_OAUTH_TOKEN via containerEnv
- Template claude files: Copied from template's home/vscode/.claude/ to container's claude dir on creation (won't overwrite existing)
- Sidecar architecture: Proxy sidecars use project path hash as ParentRef (not container ID) because sidecar is created before devcontainer exists
- Network isolation via mitmproxy: Filter script (allowlist) and --ignore-hosts (passthrough) control traffic; CA cert mounted and installed in devcontainer
- Proxy environment variables: http_proxy, https_proxy, and cert paths (REQUESTS_CA_BUNDLE, NODE_EXTRA_CA_CERTS, SSL_CERT_FILE) auto-injected when isolation enabled

## Invariants
- containers map updated only via Refresh() or after Create/Destroy
- sidecars map updated via Refresh() or after sidecar create/destroy
- State transitions: created -> running <-> stopped -> (destroyed)
- Manager methods are nil-safe for logger (NopLogger default)
- Claude config directories are never deleted (persist user customizations)
- Sidecar lifecycle: started before main container, stopped after main container
- Network and proxy configs cleaned up only on Destroy (not Stop)

## Key Files
- `manager.go` - Manager struct, lifecycle operations, session management, sidecar lifecycle (createProxySidecar, destroySidecar, etc.), GetContainerIsolationInfo()
- `runtime.go` - RuntimeInterface impl for Docker/Podman CLI, ExecAs for user-specific commands, CreateNetwork, RemoveNetwork, RunContainer, InspectContainer(), GetIsolationInfo()
- `devcontainer.go` - DevcontainerGenerator, DevcontainerCLI, Claude config management, proxy env injection, CA cert mount
- `proxy.go` - Mitmproxy filter script generation (GenerateFilterScript), passthrough patterns (GenerateIgnoreHostsPattern), proxy config/cert directory management, ReadAllowlistFromFilterScript()
- `types.go` - Container, Session, Sidecar, CreateOptions, ProxyConfig, RunContainerOptions, IsolationInfo, ProgressStep, ProgressCallback, state constants, label constants

## Gotchas
- Container IDs may be truncated; Create() does prefix matching on refresh
- Session is duplicated from tmux package to avoid import cycles
- RuntimePath() returns full binary path to bypass shell aliases
- Session.AttachCommand(runtime, user) requires both runtime and user parameters
- Claude auth token file must exist at ~/.claude/create-auth-token for auth injection to work
- Sidecar ParentRef is project path hash (12 chars), not container ID
- Proxy health check waits for port 8080 to be listening (30s timeout)
- Network names follow pattern: devagent-{hash}-net; proxy names: devagent-{hash}-proxy

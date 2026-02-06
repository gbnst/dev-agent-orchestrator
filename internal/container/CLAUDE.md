# Container Domain

Last verified: 2026-02-06

## Purpose
Orchestrates devcontainer lifecycle: creation via @devcontainers/cli, start/stop/destroy via Docker Compose, and tmux session management within containers. Manages per-container Claude Code configuration including auth token injection and persistent settings. Provides network isolation via mitmproxy sidecars with domain allowlisting and optional GitHub PR merge blocking.

## Contracts
- **Exposes**: `Manager`, `Container`, `Session`, `Sidecar`, `CreateOptions`, `ContainerState`, `RuntimeInterface`, `DevcontainerJSON`, `IsolationInfo`, `ProgressStep`, `ProgressCallback`, `ComposeGenerator`, `ComposeResult`, `ComposeOptions`, `TemplateData`, `GenerateResult`, `DevcontainerGenerator`, `DevcontainerCLI`, `ProcessDevcontainerTemplate`, `HashTruncLen`, `MountInfo`
- **Guarantees**: Auto-detects Docker/Podman from config. Operations are idempotent (stop already-stopped is safe). Labels track devagent metadata. Claude config directories persist across container recreations. Sidecars are created before devcontainer and destroyed after. Proxy CA certs are auto-installed via postCreateCommand. Container creation reports progress via OnProgress callback. Isolation info can be queried from running containers. Compose mode generates docker-compose.yml with app + proxy services in isolated network.
- **Expects**: Container runtime available. Valid config for Create operations. Refresh() called before List(). mitmproxy image available for network isolation. For compose mode: docker-compose or podman-compose available.

## Dependencies
- **Uses**: config.Config, config.Template, logging.Manager (optional), @devcontainers/cli (external), mitmproxy/mitmproxy (external image)
- **Used by**: TUI (Model), main.go
- **Boundary**: Container operations only; no UI concerns

## Key Decisions
- RuntimeInterface abstraction: Enables mock testing without real containers; includes network ops (CreateNetwork, RemoveNetwork, RunContainer) and compose lifecycle ops (ComposeUp, ComposeStart, ComposeStop, ComposeDown)
- Devcontainer CLI for creation: Handles complex setup (features, mounts, env); supports both image-based and dockerComposeFile modes
- Compose orchestration: When CreateOptions.UseCompose is true, generates docker-compose.yml with app service (devcontainer) + proxy service (mitmproxy) in isolated network; uses devcontainer CLI's dockerComposeFile property
- Compose file generation: ComposeGenerator processes docker-compose.yml.tmpl and filter.py.tmpl via processFilterTemplate(); Dockerfile.proxy is loaded as static file; filter.py.tmpl is embedded in template directories; DevcontainerGenerator.WriteAll() writes all files to .devcontainer/
- Labels for metadata: devagent.managed, devagent.project_path, devagent.template, devagent.remote_user, devagent.sidecar_of, devagent.sidecar_type
- RemoteUser defaults to "vscode" per devcontainer spec; all exec operations use ExecAs with this user
- Per-project Claude config: Each container gets a unique .claude directory (hashed by project path) mounted from ~/.local/share/devagent/claude-configs/
- Auth token auto-provisioning: Checks for `{XDG_CONFIG_HOME}/claude/.devagent-claude-token` (or `~/.claude/.devagent-claude-token`), runs `claude setup-token` if missing (non-blocking on error)
- Token injection via bind mount: Token file mounted read-only to `/run/secrets/claude-token`, shell profiles export CLAUDE_CODE_OAUTH_TOKEN from mounted file (not via containerEnv)
- Template claude files: Copied from template's home/vscode/.claude/ to container's claude dir on creation (won't overwrite existing)
- Sidecar architecture: Proxy sidecars use project path hash as ParentRef (not container ID) because sidecar is created before devcontainer exists
- Network isolation via mitmproxy: Proxy command is static (no `--ignore-hosts` flag); filter.py.tmpl (from template) controls traffic with hardcoded allowlist and passthrough domains via the filter script's `load()` hook using `ctx.options.ignore_hosts`; CA cert mounted and installed in devcontainer via CertInstallCommand in postCreateCommand
- GitHub PR merge blocking: When BLOCK_GITHUB_PR_MERGE enabled in filter.py.tmpl, filter script blocks PUT to /repos/.*/pulls/\d+/merge and POST /graphql with mergePullRequest
- Proxy environment variables: http_proxy, https_proxy, and cert paths (REQUESTS_CA_BUNDLE, NODE_EXTRA_CA_CERTS, SSL_CERT_FILE) auto-injected when isolation enabled

## Invariants
- containers and sidecars maps protected by sync.RWMutex; all reads use RLock, all writes use Lock
- containers map updated only via Refresh() or after Create/Destroy
- sidecars map updated via Refresh() or after sidecar create/destroy
- State transitions: created -> running <-> stopped -> (destroyed)
- Manager methods are nil-safe for logger (NopLogger default)
- Claude config directories are never deleted (persist user customizations)
- Sidecar lifecycle: started before main container, stopped after main container
- Network and proxy configs cleaned up only on Destroy (not Stop)

## Key Files
- `manager.go` - Manager struct, compose-based lifecycle operations (CreateWithCompose, StartWithCompose, StopWithCompose, DestroyWithCompose), session management, sidecar lifecycle, GetContainerIsolationInfo()
- `runtime.go` - RuntimeInterface impl for Docker/Podman CLI, ExecAs for user-specific commands, CreateNetwork, RemoveNetwork, RunContainer, InspectContainer(), GetIsolationInfo(), GetMounts(), ComposeUp/Start/Stop/Down
- `compose.go` - ComposeGenerator with processFilterTemplate() and processComposeTemplate(), TemplateData (ProjectHash, ProjectPath, ProjectName, WorkspaceFolder, ClaudeConfigDir, TemplateName, ContainerName, CertInstallCommand, ProxyImage, ProxyPort, RemoteUser), ComposeResult, ComposeOptions; processes docker-compose.yml.tmpl and filter.py.tmpl for compose-based orchestration
- `devcontainer.go` - DevcontainerGenerator, GenerateResult, DevcontainerCLI, ProcessDevcontainerTemplate; Claude config management, proxy env injection, CA cert mount; WriteComposeFiles(), WriteAll()
- `proxy.go` - Mitmproxy utility functions: proxy config/cert directory management (GetProxyConfigDir, GetProxyCertDir, GetProxyCACertPath, ProxyCertExists), allowlist parsing from filter script (ReadAllowlistFromFilterScript, parseAllowlistFromScript), CleanupProxyConfigs
- `types.go` - Container, Session, Sidecar, CreateOptions (UseCompose flag), IsolationInfo, MountInfo (with JSON tags for external tool output), DevcontainerJSON (DockerComposeFile, Service, RemoteUser fields), BuildConfig, ProgressStep, ProgressCallback, HashTruncLen, state constants, label constants

## Gotchas
- Container IDs may be truncated; Create() does prefix matching on refresh
- Session is duplicated from tmux package to avoid import cycles
- RuntimePath() returns full binary path to bypass shell aliases
- Session.AttachCommand(runtime, user) requires both runtime and user parameters
- Claude auth token is auto-provisioned via `claude setup-token` if not present; token stored in `~/.claude/.devagent-claude-token` (XDG-aware)
- Sidecar ParentRef is project path hash (12 chars), not container ID
- Proxy health check waits for container to be running (30s timeout)
- Network names follow pattern: devagent-{hash}-net; proxy names: devagent-{hash}-proxy
- Compose mode: workspace mount IS in docker-compose.yml (devcontainer CLI doesn't auto-mount in compose mode)
- Compose mode requires templates to define isolation config (no hardcoded defaults)
- Podman + dockerComposeFile: Known devcontainer CLI bug #863; see docs/PODMAN.md for workarounds
- filter.py.tmpl is processed as a Go template via processFilterTemplate() but currently has no Go template placeholders (all config is hardcoded in the template)

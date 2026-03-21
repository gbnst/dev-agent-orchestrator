# Container Domain

Last verified: 2026-03-13

## Purpose
Orchestrates devcontainer lifecycle: creation via Docker Compose with template rendering, start/stop/destroy via Docker Compose, and tmux session management within containers. Provides network isolation via mitmproxy sidecars with domain allowlisting and optional GitHub PR merge blocking. Integrates proxy log tailing for real-time HTTP request visibility in TUI.

## Contracts
- **Exposes**: `Manager`, `ManagerOptions`, `NewManager(opts)`, `Manager.SetOnChange()`, `Manager.GetByNameOrID()`, `Manager.GetByComposeProject()`, `Manager.CaptureSession()`, `Manager.CaptureSessionLines()`, `Manager.CursorPosition()`, `Manager.SendToSession()`, `Container`, `ComposeProject`, `Ports`, `Sidecar`, `CreateOptions`, `ContainerState`, `RuntimeInterface`, `DevcontainerJSON`, `IsolationInfo`, `ProgressStep`, `ProgressCallback`, `ComposeGenerator`, `ComposeResult`, `ComposeOptions`, `TemplateData`, `GenerateResult`, `SanitizeComposeName`, `ParsePortEnvVars`, `AllocateFreePorts`, `FindTemplateForProject`, `ComposeGenerator.WriteToProject`, `HashTruncLen`, `MountInfo`
- **Note**: `NewManager(ManagerOptions{...})` is the single constructor; requires Runtime or Config (auto-creates Runtime from Config if nil). Other fields (Templates, LogManager, etc.) have sensible defaults. `NewComposeGenerator(cfg, templates, logger)` requires a `*config.Config` and `*logging.ScopedLogger` parameter (use `&config.Config{}` and `logging.NopLogger()` in tests)
- **Guarantees**: Auto-detects Docker/Podman from config. Operations are idempotent (stop already-stopped is safe). Labels track devagent metadata. Sidecars are created before devcontainer and destroyed after. Proxy CA certs are auto-installed via entrypoint.sh (runs before VS Code connects). Proxy service healthcheck gates app startup on cert existence. Container creation reports progress via OnProgress callback. Isolation info can be queried from running containers. Compose mode generates docker-compose.yml with app + proxy services in isolated network. Proxy log reader started on container creation, stopped on stop or destroy. GitHub token injected into containers when available (non-blocking on missing token). Template files are copied via generic directory walk (`copyTemplateDir`) — adding new template files requires zero Go code changes. State-change callback (`SetOnChange`) fires after Refresh, Start, Stop, Destroy, CreateSession, and KillSession to enable push notifications (e.g., SSE). ComposeGenerator.Generate() validates template data (ContainerName, ProjectName) against YAML-special characters before returning.
- **Expects**: Container runtime available. Valid config for Create operations. Refresh() called before List(). mitmproxy image available for network isolation. For compose mode: docker-compose or podman-compose available. LogManager must implement GetChannelSink() for proxy log integration.

## Dependencies
- **Uses**: config.Config, config.Template, logging.Manager (optional), logging.ScopedLogger, logging.ProxyLogReader, mitmproxy/mitmproxy (external image), gh CLI (external, installed in Dockerfiles)
- **Used by**: TUI (Model), web.Server, main.go
- **Boundary**: Container operations only; no UI concerns

## Key Decisions
- RuntimeInterface abstraction: Enables mock testing without real containers; includes query ops (ListContainers, InspectContainer, GetIsolationInfo, Exec, ExecAs) and compose lifecycle ops (ComposeUp, ComposeStart, ComposeStop, ComposeDown). Manager always uses Compose-based operations for lifecycle
- Compose-based creation: All containers created via docker-compose from project root, not worktree paths. Template rendering generates docker-compose.yml at project root's .devcontainer directory. Compose project name derived from project base name or worktree-specific naming (SanitizeComposeName for Docker Compose compatibility).
- Compose file generation: ComposeGenerator.Generate() returns TemplateData; ComposeGenerator.WriteToProject() walks template's `.devcontainer/` subtree via `copyTemplateDir()`, processing `.tmpl` files and copying all others
- Port management: AllocateFreePorts finds free host ports; ParsePortEnvVars extracts port bindings from environment vars. Ports map stored in Container for API responses.
- Compose project naming: SanitizeComposeName converts arbitrary names to lowercase alphanumeric-hyphen format for Docker Compose compatibility
- Labels for metadata: devagent.managed, devagent.project_path, devagent.template, devagent.remote_user, devagent.sidecar_type, devagent.compose_project; sidecar-to-devcontainer correlation uses com.docker.compose.project label (set automatically by Docker Compose)
- Container metadata: ComposeProject (compose project name), Ports (map of service:host_port)
- RemoteUser defaults to "vscode" per devcontainer spec; all exec operations use ExecAs with this user
- Auth token paths configurable: `Config.ClaudeTokenPath` and `Config.GitHubTokenPath` set in config.yaml; `Config.ResolveTokenPath()` expands `~/` prefix; if path is empty/omitted, that token is skipped entirely (no auto-detection)
- Auth token auto-provisioning: `ensureClaudeToken(path)` reads existing token or runs `claude setup-token` if missing (non-blocking on error); `ensureGitHubToken(path)` reads token or returns empty (no auto-provisioning)
- Token injection via bind mount: Token file mounted read-only to `/run/secrets/claude-token`, shell profiles export CLAUDE_CODE_OAUTH_TOKEN from mounted file (not via containerEnv)
- GitHub CLI authentication: Token file mounted read-only to `/run/secrets/github-token`; shell profiles export GH_TOKEN (with `-s` file-size check to avoid exporting empty string); falls back to /dev/null mount if token file missing (non-blocking, warns via logger); gh CLI installed in all template Dockerfiles
- SetOnChange callback: `Manager.SetOnChange(fn func())` registers a single callback invoked after any state mutation (Refresh, Start, Stop, Destroy, CreateSession, KillSession); must be set before concurrent access; used by web.Server to drive SSE event broker
- Sidecar architecture: Proxy sidecars use compose project name as ParentRef (from com.docker.compose.project label); both app and proxy containers share this label automatically via Docker Compose
- Network isolation via mitmproxy: Proxy uses mitmproxy/mitmproxy:latest image; filter.py (from template) controls traffic with hardcoded allowlist and passthrough domains via the filter script's `load()` hook using `ctx.options.ignore_hosts`; CA cert installed in devcontainer via entrypoint.sh (runs before VS Code connects, installs to system trust store)
- GitHub PR merge blocking: When BLOCK_GITHUB_PR_MERGE enabled in filter.py, filter script blocks PUT to /repos/.*/pulls/\d+/merge and POST /graphql with mergePullRequest
- Proxy environment variables: http_proxy, https_proxy, and cert paths (REQUESTS_CA_BUNDLE, NODE_EXTRA_CA_CERTS, SSL_CERT_FILE) auto-injected when isolation enabled

## Invariants
- containers and sidecars maps protected by sync.RWMutex; all reads use RLock, all writes use Lock
- containers map updated only via Refresh() or after Create/Destroy
- sidecars map updated via Refresh() or after sidecar create/destroy
- proxyLogCancels map protected by same mutex as containers
- State transitions: created -> running <-> stopped -> (destroyed)
- Manager methods are nil-safe for logger (NopLogger default)
- Sidecar lifecycle: started before main container, stopped after main container
- Network and proxy configs cleaned up only on Destroy (not Stop)
- Proxy log reader lifecycle: started after CreateWithCompose, cancelled in StopWithCompose and DestroyWithCompose

## Key Files
- `manager.go` - Manager struct, compose-based lifecycle operations (CreateWithCompose, StartWithCompose, StopWithCompose, DestroyWithCompose), session management, sidecar lifecycle, GetContainerIsolationInfo(), GetByComposeProject()
- `runtime.go` - RuntimeInterface impl for Docker/Podman CLI: ListContainers, Exec, ExecAs, InspectContainer, GetIsolationInfo, ComposeUp/Start/Stop/Down, GetMounts
- `compose.go` - ComposeGenerator with buildTemplateData(), validateTemplateData(), processTemplate(), WriteToProject(); TemplateData (ProjectPath, ProjectName, WorkspaceFolder, ClaudeTokenPath, GitHubTokenPath, TemplateName, ContainerName, ProxyImage, RemoteUser, ProxyLogPath); ComposeResult (TemplateData only); ComposeOptions; SanitizeComposeName, ParsePortEnvVars, AllocateFreePorts
- `devcontainer.go` - Utility functions: token management (ensureClaudeToken, ensureGitHubToken), ReadWorkspaceFolder, copyTemplateDir, getDataDir
- `proxy.go` - Mitmproxy utility functions: proxy cert directory management (GetProxyCertDir, GetProxyCACertPath, ProxyCertExists), allowlist parsing from filter script (ReadAllowlistFromFilterScript, parseAllowlistFromScript), CleanupProxyConfigs
- `types.go` - Container (with ComposeProject, Ports fields), Session, Sidecar (ID, Name, Type, ParentRef, State), CreateOptions, IsolationInfo, MountInfo (with JSON tags for external tool output), DevcontainerJSON (DockerComposeFile, Service, RemoteUser fields), BuildConfig, ProgressStep, ProgressCallback, HashTruncLen, FindTemplateForProject, state constants, label constants
- `ports.go` - Port discovery and allocation: AllocateFreePorts, ParsePortEnvVars, netFindFreePort (internal)

## Gotchas
- Container IDs are full 64-character hashes (--no-trunc); Create() does prefix matching on refresh as fallback
- Container.Sessions uses `tmux.Session` directly (no duplication); Manager delegates session operations to `tmux.Client`
- RuntimePath() returns full binary path to bypass shell aliases
- Session.AttachCommand(runtime, user) requires both runtime and user parameters
- Token paths are configured via `config.yaml` (`claude_token_path`, `github_token_path`); omitting a path skips that token entirely
- Claude auth token is auto-provisioned via `claude setup-token` if configured path doesn't exist
- GitHub token is NOT auto-provisioned; user must manually create the file at the configured path; missing token is non-fatal (gh CLI will be unauthenticated)
- Sidecar ParentRef is compose project name (from com.docker.compose.project label), not container ID or hash
- Proxy health check waits for container to be running (30s timeout)
- Container names are auto-generated by Docker Compose (e.g., myproject-app-1, myproject-proxy-1); no hardcoded container_name in templates
- Compose mode: workspace mount IS in docker-compose.yml (devcontainer CLI doesn't auto-mount in compose mode)
- Compose mode requires templates to define isolation config (no hardcoded defaults)
- Podman + dockerComposeFile: Known devcontainer CLI bug #863; see docs/PODMAN.md for workarounds
- filter.py is provided by the template at .devcontainer/containers/proxy/opt/devagent-proxy/filter.py and mounted at /opt/devagent-proxy/filter.py for the mitmproxy sidecar
- Proxy log reader requires LogManager with GetChannelSink(); uses type assertion at runtime
- Proxy logs directory created via .gitkeep at .devcontainer/containers/proxy/opt/devagent-proxy/logs/
- Template directory layout: all template files live under `.devcontainer/`; `containers/app/` mirrors app container filesystem; `containers/proxy/` mirrors proxy container filesystem; `.tmpl` files are processed, others copied as-is
- entrypoint.sh handles mitmproxy CA cert installation (runs as root before VS Code connects); uses `sh` invocation in compose entrypoint since copyTemplateDir writes files with 0644
- post-create.sh handles project-specific setup only (go mod download, uv sync, etc.); called via `bash` from devcontainer.json postCreateCommand

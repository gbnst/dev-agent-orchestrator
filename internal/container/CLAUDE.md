# Container Domain

Last verified: 2026-02-11

## Purpose
Orchestrates devcontainer lifecycle: creation via @devcontainers/cli, start/stop/destroy via Docker Compose, and tmux session management within containers. Provides network isolation via mitmproxy sidecars with domain allowlisting and optional GitHub PR merge blocking. Integrates proxy log tailing for real-time HTTP request visibility in TUI.

## Contracts
- **Exposes**: `Manager`, `Container`, `Session`, `Sidecar`, `CreateOptions`, `ContainerState`, `RuntimeInterface`, `DevcontainerJSON`, `IsolationInfo`, `ProgressStep`, `ProgressCallback`, `ComposeGenerator`, `ComposeResult`, `ComposeOptions`, `TemplateData`, `GenerateResult`, `DevcontainerGenerator`, `DevcontainerCLI`, `HashTruncLen`, `MountInfo`
- **Note**: `NewComposeGenerator(templates, logger)` requires a `*logging.ScopedLogger` parameter (use `logging.NopLogger()` in tests)
- **Guarantees**: Auto-detects Docker/Podman from config. Operations are idempotent (stop already-stopped is safe). Labels track devagent metadata. Sidecars are created before devcontainer and destroyed after. Proxy CA certs are auto-installed via post-create.sh. Container creation reports progress via OnProgress callback. Isolation info can be queried from running containers. Compose mode generates docker-compose.yml with app + proxy services in isolated network. Proxy log reader started on container creation, stopped on destroy. GitHub token injected into containers when available (non-blocking on missing token). Template files are copied via generic directory walk (`copyTemplateDir`) — adding new template files requires zero Go code changes.
- **Expects**: Container runtime available. Valid config for Create operations. Refresh() called before List(). mitmproxy image available for network isolation. For compose mode: docker-compose or podman-compose available. LogManager must implement GetChannelSink() for proxy log integration.

## Dependencies
- **Uses**: config.Config, config.Template, logging.Manager (optional), logging.ScopedLogger, logging.ProxyLogReader, @devcontainers/cli (external), mitmproxy/mitmproxy (external image), gh CLI (external, installed in Dockerfiles)
- **Used by**: TUI (Model), main.go
- **Boundary**: Container operations only; no UI concerns

## Key Decisions
- RuntimeInterface abstraction: Enables mock testing without real containers; includes network ops (CreateNetwork, RemoveNetwork, RunContainer) and compose lifecycle ops (ComposeUp, ComposeStart, ComposeStop, ComposeDown)
- Devcontainer CLI for creation: Handles complex setup (features, mounts, env); supports both image-based and dockerComposeFile modes
- Compose orchestration: When CreateOptions.UseCompose is true, generates docker-compose.yml with app service (devcontainer) + proxy service (mitmproxy) in isolated network; uses devcontainer CLI's dockerComposeFile property
- Compose file generation: ComposeGenerator.Generate() returns TemplateData; DevcontainerGenerator.WriteToProject() walks template's `.devcontainer/` subtree via `copyTemplateDir()`, processing `.tmpl` files and copying all others; WriteAll() delegates to WriteToProject with TemplateData from ComposeResult
- Labels for metadata: devagent.managed, devagent.project_path, devagent.template, devagent.remote_user, devagent.sidecar_type; sidecar-to-devcontainer correlation uses com.docker.compose.project label (set automatically by Docker Compose)
- RemoteUser defaults to "vscode" per devcontainer spec; all exec operations use ExecAs with this user
- Auth token auto-provisioning: Checks for `{XDG_CONFIG_HOME}/claude/.devagent-claude-token` (or `~/.claude/.devagent-claude-token`), runs `claude setup-token` if missing (non-blocking on error)
- Token injection via bind mount: Token file mounted read-only to `/run/secrets/claude-token`, shell profiles export CLAUDE_CODE_OAUTH_TOKEN from mounted file (not via containerEnv)
- GitHub CLI authentication: `ensureGitHubToken()` reads `{XDG_CONFIG_HOME}/github/token` (or `~/.config/github/token`); token file mounted read-only to `/run/secrets/github-token`; shell profiles export GH_TOKEN (with `-s` file-size check to avoid exporting empty string); falls back to /dev/null mount if token file missing (non-blocking, warns via logger); gh CLI installed in all template Dockerfiles
- Sidecar architecture: Proxy sidecars use compose project name as ParentRef (from com.docker.compose.project label); both app and proxy containers share this label automatically via Docker Compose
- Network isolation via mitmproxy: Proxy uses mitmproxy/mitmproxy:latest image; filter.py (from template) controls traffic with hardcoded allowlist and passthrough domains via the filter script's `load()` hook using `ctx.options.ignore_hosts`; CA cert installed in devcontainer via post-create.sh (waits for cert, copies to trust store)
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
- Proxy log reader lifecycle: started after CreateWithCompose, cancelled in DestroyWithCompose

## Key Files
- `manager.go` - Manager struct, compose-based lifecycle operations (CreateWithCompose, StartWithCompose, StopWithCompose, DestroyWithCompose), session management, sidecar lifecycle, GetContainerIsolationInfo()
- `runtime.go` - RuntimeInterface impl for Docker/Podman CLI, ExecAs for user-specific commands, CreateNetwork, RemoveNetwork, RunContainer, InspectContainer(), GetIsolationInfo(), GetMounts(), ComposeUp/Start/Stop/Down
- `compose.go` - ComposeGenerator with buildTemplateData(), processTemplate(); TemplateData (ProjectPath, ProjectName, WorkspaceFolder, ClaudeTokenPath, GitHubTokenPath, TemplateName, ContainerName, ProxyImage, ProxyPort, RemoteUser, ProxyLogPath); ComposeResult (TemplateData only); ComposeOptions
- `devcontainer.go` - DevcontainerGenerator, GenerateResult (TemplatePath only), DevcontainerCLI; WriteToProject() uses copyTemplateDir() to walk template's .devcontainer/ subtree (processes .tmpl files, copies others); WriteAll() delegates to WriteToProject with ComposeResult.TemplateData
- `proxy.go` - Mitmproxy utility functions: proxy cert directory management (GetProxyCertDir, GetProxyCACertPath, ProxyCertExists), allowlist parsing from filter script (ReadAllowlistFromFilterScript, parseAllowlistFromScript), CleanupProxyConfigs
- `types.go` - Container, Session, Sidecar, CreateOptions (UseCompose flag), IsolationInfo, MountInfo (with JSON tags for external tool output), DevcontainerJSON (DockerComposeFile, Service, RemoteUser fields), BuildConfig, ProgressStep, ProgressCallback, HashTruncLen, state constants, label constants

## Gotchas
- Container IDs may be truncated; Create() does prefix matching on refresh
- Session is duplicated from tmux package to avoid import cycles
- RuntimePath() returns full binary path to bypass shell aliases
- Session.AttachCommand(runtime, user) requires both runtime and user parameters
- Claude auth token is auto-provisioned via `claude setup-token` if not present; token stored in `~/.claude/.devagent-claude-token` (XDG-aware)
- GitHub token is NOT auto-provisioned; user must manually create `~/.config/github/token` (XDG-aware); missing token is non-fatal (gh CLI will be unauthenticated)
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
- post-create.sh handles mitmproxy CA cert installation and project-specific setup (go mod download, uv sync, etc.); called via `bash` (not execute bit) from devcontainer.json postCreateCommand

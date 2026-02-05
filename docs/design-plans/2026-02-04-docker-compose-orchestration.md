# Docker Compose Orchestration Design

## Summary

This design migrates devagent's container orchestration from manual Docker API calls to Docker Compose, leveraging the devcontainer CLI's native `dockerComposeFile` support. Currently, devagent creates containers through a multi-step process: first manually creating a proxy sidecar via Docker API, then generating a devcontainer.json with baked-in network and environment configuration, and finally invoking `devcontainer up`. This approach requires custom orchestration logic for lifecycle operations (start, stop, destroy) and tight coupling between the Manager and low-level container APIs.

The new approach consolidates orchestration into a declarative docker-compose.yml file that defines both the application container and proxy sidecar as services with explicit dependencies. A new ComposeGenerator produces this file alongside Dockerfile.proxy and filter.py, all with project-specific values baked in at generation time. The devcontainer.json is simplified to merely reference the compose file, delegating all orchestration concerns to Docker Compose. This eliminates manual sidecar management code, reduces the surface area for runtime-specific bugs (Docker vs Podman), and makes the entire container stack reproducible from a single directory of generated files. Lifecycle operations become thin wrappers around compose commands, and the existing label-based discovery mechanism continues to work unchanged.

## Definition of Done

1. **Container orchestration uses Docker Compose** - The `Manager.Create()` flow generates a `docker-compose.yml` file and uses `devcontainer up` with `dockerComposeFile` property instead of manual container creation

2. **All runtime behaviors preserved** - Network isolation, proxy filtering, environment variables, mounts, and resource limits work identically to current implementation

3. **Works with Docker and Podman** - Compose files and devcontainer invocation work with both container runtimes

4. **Proxy always included** - Every project gets a proxy sidecar in the compose file

5. **Features baked into Dockerfiles** - Templates use Dockerfiles for tooling instead of devcontainer features

## Glossary

- **Devcontainer**: A specification for defining development environments in containers, with a CLI (`@devcontainers/cli`) that creates containers from `devcontainer.json` configuration files
- **Docker Compose**: A tool for defining and running multi-container Docker applications using YAML configuration files to declare services, networks, and volumes
- **Generator pattern**: A design pattern used in this codebase where a struct has a `Generate()` method returning a result struct and a `WriteToProject()` method that writes files to disk
- **Manager**: The core orchestration component in devagent (`internal/container/manager.go`) responsible for creating, starting, stopping, and destroying container environments
- **Mitmproxy**: A transparent HTTP/HTTPS proxy that can intercept and filter network traffic, used here to enforce domain allowlists
- **Podman**: A daemonless container engine that serves as an alternative to Docker, supporting the same CLI commands
- **RuntimeInterface**: An abstraction layer in devagent that wraps container runtime operations (Docker or Podman) behind a common interface
- **Sidecar**: A container pattern where an auxiliary container runs alongside the main application container to provide supporting functionality (here, network filtering via mitmproxy)

## Architecture

Replace manual container orchestration with Docker Compose, using devcontainer's native `dockerComposeFile` support.

**Current flow:**
```
Manager.Create()
  → createProxySidecar() [manual Docker API calls]
  → Generator.Generate() [devcontainer.json]
  → WriteToProject()
  → DevcontainerCLI.Up()
```

**New flow:**
```
Manager.Create()
  → ComposeGenerator.Generate() [docker-compose.yml + Dockerfile.proxy + filter.py]
  → DevcontainerGenerator.Generate() [simplified devcontainer.json]
  → WriteToProject() [writes all files]
  → DevcontainerCLI.Up() [devcontainer up with dockerComposeFile]
```

**Component responsibilities:**

- **ComposeGenerator** (`internal/container/compose.go`) — Generates `docker-compose.yml`, `Dockerfile.proxy`, and `filter.py` with baked per-project values
- **DevcontainerGenerator** (`internal/container/devcontainer.go`) — Modified to produce simplified devcontainer.json with `dockerComposeFile` property
- **RuntimeInterface** (`internal/container/runtime.go`) — Extended with compose commands for lifecycle management
- **Manager** (`internal/container/manager.go`) — Simplified to delegate orchestration to compose

**Generated files per project:**

```
.devcontainer/
├── docker-compose.yml    # Two services: app + proxy
├── devcontainer.json     # Points to compose file
├── Dockerfile            # App container (from template)
├── Dockerfile.proxy      # Proxy with baked filter script
├── filter.py             # Mitmproxy allowlist filter
└── home/                 # User config files (from template)
```

**Compose service structure:**

```yaml
services:
  app:
    build: { context: ., dockerfile: Dockerfile }
    container_name: devagent-{hash}-app
    depends_on:
      proxy: { condition: service_started }
    networks: [isolated]
    environment: { http_proxy, https_proxy, SSL_CERT_FILE, ... }
    volumes: [claude-config, proxy-certs, oauth-token]
    cap_drop: [ALL]
    mem_limit: 4g
    cpus: 2
    pids_limit: 512
    labels: { devagent.managed, devagent.project_path, ... }

  proxy:
    build: { context: ., dockerfile: Dockerfile.proxy }
    container_name: devagent-{hash}-proxy
    networks: [isolated]
    volumes: [proxy-certs]
    labels: { devagent.managed, devagent.sidecar_of, devagent.sidecar_type }

networks:
  isolated:
    name: devagent-{hash}-net
```

**Simplified devcontainer.json:**

```json
{
  "name": "devagent-container",
  "dockerComposeFile": "docker-compose.yml",
  "service": "app",
  "workspaceFolder": "/workspaces/{project-name}",
  "remoteUser": "vscode",
  "postCreateCommand": "... && sudo update-ca-certificates"
}
```

**Lifecycle mapping:**

| Operation | Implementation |
|-----------|----------------|
| Create | `devcontainer up` (compose handles network + both containers) |
| Start | `docker-compose start` |
| Stop | `docker-compose stop` |
| Destroy | `docker-compose down` |

## Existing Patterns

Investigation found current orchestration in `internal/container/`:

- **Generator pattern**: `DevcontainerGenerator` with `Generate()` returning a result struct and `WriteToProject()` writing files. New `ComposeGenerator` follows same pattern.
- **Naming conventions**: Network `devagent-{hash}-net`, containers `devagent-{hash}-app` and `devagent-{hash}-proxy`, using 12-char SHA256 hash of project path.
- **Label-based discovery**: Containers tagged with `devagent.managed`, `devagent.project_path`, `devagent.sidecar_of`, etc. Preserved in compose labels.
- **Mount paths**: Claude config at `~/.local/share/devagent/claude-configs/{hash}`, proxy certs at `~/.local/share/devagent/proxy-certs/{hash}`. Unchanged.
- **RuntimeInterface abstraction**: All Docker/Podman operations go through interface. Extended with compose commands.

No existing docker-compose usage in codebase. This design introduces compose as a new pattern for container orchestration.

## Implementation Phases

<!-- START_PHASE_1 -->
### Phase 1: Compose Generator

**Goal:** Create new generator that produces docker-compose.yml, Dockerfile.proxy, and filter.py

**Components:**
- `ComposeGenerator` struct in `internal/container/compose.go` — parallel to DevcontainerGenerator
- `ComposeResult` struct — holds generated YAML, Dockerfile.proxy content, and filter script
- `Generate()` method — accepts CreateOptions, returns ComposeResult with baked values
- Uses existing `proxy.GenerateFilterScript()` and `proxy.GenerateIgnoreHostsPattern()`

**Dependencies:** None (new file, uses existing proxy.go functions)

**Done when:** ComposeGenerator produces valid docker-compose.yml, Dockerfile.proxy, and filter.py for a given CreateOptions; unit tests verify output structure
<!-- END_PHASE_1 -->

<!-- START_PHASE_2 -->
### Phase 2: Modify DevcontainerGenerator

**Goal:** Simplify devcontainer.json output to use dockerComposeFile property

**Components:**
- Modified `DevcontainerGenerator.Generate()` in `internal/container/devcontainer.go` — removes runArgs, containerEnv, mounts (moved to compose)
- Adds `dockerComposeFile`, `service`, `workspaceFolder` properties to output
- Keeps `postCreateCommand`, `remoteUser`, `customizations`

**Dependencies:** Phase 1 (ComposeGenerator exists)

**Done when:** DevcontainerGenerator produces simplified devcontainer.json with dockerComposeFile property; existing tests updated
<!-- END_PHASE_2 -->

<!-- START_PHASE_3 -->
### Phase 3: Extend WriteToProject

**Goal:** Write all generated files to project's .devcontainer directory

**Components:**
- Modified `WriteToProject()` in `internal/container/devcontainer.go` — writes docker-compose.yml, Dockerfile.proxy, filter.py in addition to existing files
- Accepts both DevcontainerResult and ComposeResult

**Dependencies:** Phase 1, Phase 2

**Done when:** WriteToProject creates complete .devcontainer directory with all files; unit tests verify file creation
<!-- END_PHASE_3 -->

<!-- START_PHASE_4 -->
### Phase 4: RuntimeInterface Compose Commands

**Goal:** Add compose lifecycle commands to runtime abstraction

**Components:**
- New methods on `RuntimeInterface` in `internal/container/runtime.go`:
  - `ComposeUp(projectDir string) error`
  - `ComposeStart(projectDir string) error`
  - `ComposeStop(projectDir string) error`
  - `ComposeDown(projectDir string) error`
- Compose command detection: `docker compose` (v2), `docker-compose` (v1), or `podman-compose`
- Implementation in `Runtime` struct

**Dependencies:** None (independent of generators)

**Done when:** Runtime can execute compose commands; unit tests with mock executor verify command construction
<!-- END_PHASE_4 -->

<!-- START_PHASE_5 -->
### Phase 5: Simplify Manager.Create

**Goal:** Replace manual sidecar orchestration with compose-based creation

**Components:**
- Modified `Manager.Create()` in `internal/container/manager.go`:
  - Calls ComposeGenerator.Generate()
  - Calls DevcontainerGenerator.Generate()
  - Calls WriteToProject() with both results
  - Ensures proxy cert directory exists
  - Calls DevcontainerCLI.Up() (unchanged)
- Remove `createProxySidecar()`, `waitForProxyReady()`

**Dependencies:** Phases 1-4

**Done when:** Manager.Create() works with compose; E2E test creates container with working proxy
<!-- END_PHASE_5 -->

<!-- START_PHASE_6 -->
### Phase 6: Manager Lifecycle Operations

**Goal:** Use compose commands for start/stop/destroy

**Components:**
- Modified `Manager.Start()` — uses `runtime.ComposeStart()`
- Modified `Manager.Stop()` — uses `runtime.ComposeStop()`
- Modified `Manager.Destroy()` — uses `runtime.ComposeDown()`
- Remove `startSidecarsForProject()`, `stopSidecarsForProject()`, `destroySidecar()`
- Modified `Manager.Refresh()` — sidecar discovery still works via labels

**Dependencies:** Phase 4, Phase 5

**Done when:** All lifecycle operations work via compose; E2E tests for start/stop/destroy pass
<!-- END_PHASE_6 -->

<!-- START_PHASE_7 -->
### Phase 7: Template Migration

**Goal:** Ensure templates work with new compose-based flow

**Components:**
- Verify `config/templates/basic/` works with compose generation
- Verify `config/templates/go-project/` works with compose generation
- Update template documentation if needed

**Dependencies:** Phases 1-6

**Done when:** Both templates create working containers via compose; manual verification of proxy filtering
<!-- END_PHASE_7 -->

## Additional Considerations

**Podman compatibility:** The devcontainer CLI has a known unfixed bug (Issue #863) when using Podman with docker-compose files. Workarounds:
1. Pin `@devcontainers/cli` to v0.58.0
2. Use `docker-compose` binary with Podman as container backend

Document these workarounds in README.

**Proxy cert bootstrapping:** Mitmproxy generates CA certs on first run. With simultaneous startup via compose, the app container may start before certs exist. The existing `postCreateCommand` handles this — it runs after containers are up, by which time certs are generated.

**Coupled lifecycle:** With compose, start/stop affects both containers together. This is a simplification from the current decoupled model where sidecars could persist across container restarts. Accepted as trade-off for simpler orchestration.

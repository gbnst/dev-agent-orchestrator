# Container Isolation Design

## Summary

This design implements container isolation for devagent's development environments through a three-layer security model: Linux capability restrictions, resource limits, and network filtering. The approach leverages devcontainer.json's native security fields (`capAdd`, `securityOpt`) while extending them with devagent-specific configuration under `customizations.devagent.isolation`.

Network isolation is achieved through per-project mitmproxy sidecar containers that enforce domain allowlists. When a template enables network isolation, devagent creates a dedicated Docker network, launches a mitmproxy container configured with filtering rules, and configures the devcontainer to route all HTTP/HTTPS traffic through the proxy. TLS interception is handled transparently with auto-generated CA certificates that are persisted and injected into the devcontainer's trust store. The system defaults to secure-by-default: new templates inherit baseline isolation settings unless explicitly configured otherwise, ensuring that agent-controlled environments operate with minimal privileges and restricted network access.

## Definition of Done

1. **Template configuration schema**: Templates can specify isolation settings in `customizations.devagent` including capability drops, resource limits, and network allowlist domains

2. **Mitmproxy sidecar**: When a template enables network isolation, devagent automatically starts a dedicated mitmproxy container alongside the devcontainer, with:
   - Domain allowlist filtering (blocking non-allowed domains)
   - TLS interception with auto-bypass for certificate-pinned hosts
   - CA certificate automatically injected into the devcontainer

3. **Devcontainer generation**: `DevcontainerGenerator.Generate()` produces devcontainer.json with appropriate `capAdd`, `capDrop`, `securityOpt`, `runArgs` for resource limits, and proxy environment variables

4. **Isolated by default**: New templates inherit a secure baseline unless explicitly configured otherwise

## Glossary

- **devcontainer**: A development environment container defined by a `devcontainer.json` configuration file (part of Microsoft's Dev Containers specification)
- **sidecar container**: An auxiliary container that runs alongside a primary container to provide supporting services (in this design, mitmproxy)
- **mitmproxy**: A transparent HTTPS proxy that can intercept, inspect, and modify network traffic
- **TLS interception**: The process of a proxy terminating encrypted connections, inspecting traffic, and re-encrypting it (requires trusted CA certificate)
- **CA certificate**: Certificate Authority certificate used to sign TLS certificates; when trusted by the system, allows mitmproxy to intercept HTTPS traffic
- **certificate pinning**: Security technique where applications only accept specific certificates, preventing proxy interception
- **Linux capabilities**: Fine-grained permissions that divide root privileges into distinct units (e.g., `CAP_NET_ADMIN`, `CAP_SYS_ADMIN`)
- **allowlist**: A list of permitted domains; traffic to unlisted domains is blocked
- **RuntimeInterface**: Abstraction layer in the codebase that wraps Docker/Podman operations for testing and portability
- **DevcontainerGenerator**: Component that produces `devcontainer.json` files from templates and isolation configuration
- **runArgs**: Container runtime arguments passed to Docker/Podman when creating containers (e.g., `--cap-drop`, `--memory`)

## Architecture

Container isolation operates at three layers: capability restrictions, resource limits, and network filtering. All configuration lives in template `devcontainer.json` files under `customizations.devagent.isolation`.

**Capability and Resource Layer:**
`DevcontainerGenerator.Generate()` reads isolation config from templates and produces:
- `capAdd` / `securityOpt` (native devcontainer fields)
- `runArgs` with `--cap-drop`, `--memory`, `--cpus`, `--pids-limit`

**Network Isolation Layer:**
When `isolation.network.allowlist` is configured, devagent creates:
1. Per-project Docker network (`devagent-<hash>-net`)
2. Mitmproxy sidecar container connected to that network
3. Devcontainer configured with `HTTP_PROXY`/`HTTPS_PROXY` pointing to sidecar

**Data Flow:**
```
Devcontainer (HTTP_PROXY=http://proxy:8080)
    │
    ▼
Mitmproxy Sidecar (filter.py checks allowlist)
    │
    ├─► Allowed domain → Forward to internet
    └─► Blocked domain → Return 403
```

**TLS Interception:**
Mitmproxy performs TLS interception with auto-generated CA certificate. Certificate is persisted and mounted into devcontainer. Domains with certificate pinning bypass interception via `--ignore-hosts`.

## Existing Patterns

Investigation found existing patterns in the codebase that this design follows:

**Template System** (`internal/config/templates.go`):
- Templates loaded from `devcontainer.json` files
- `customizations.devagent` used for devagent-specific extensions
- This design adds `isolation` key following the same pattern

**Container Lifecycle** (`internal/container/manager.go`):
- `Manager` orchestrates container creation via `DevcontainerGenerator` + `DevcontainerCLI`
- Labels track metadata (`devagent.managed`, `devagent.project_path`, etc.)
- This design adds sidecar labels: `devagent.sidecar_of`, `devagent.sidecar_type`

**RuntimeInterface Abstraction** (`internal/container/runtime.go`):
- Wraps Docker/Podman CLI operations
- Enables mock testing without real containers
- This design extends with `CreateNetwork`, `RemoveNetwork`, `RunContainer`

**No existing multi-container patterns.** This design introduces sidecar management as a new capability.

## Implementation Phases

<!-- START_PHASE_1 -->
### Phase 1: Isolation Configuration Schema

**Goal:** Define and parse isolation configuration from templates

**Components:**
- `IsolationConfig`, `CapConfig`, `ResourceConfig`, `NetworkConfig` structs in `internal/config/templates.go`
- Parsing logic in `loadTemplate()` to extract `customizations.devagent.isolation`
- `DefaultIsolation` variable with secure defaults

**Dependencies:** None

**Done when:** Templates with `isolation` config load correctly, defaults applied when config absent, tests verify parsing
<!-- END_PHASE_1 -->

<!-- START_PHASE_2 -->
### Phase 2: Capability and Resource Generation

**Goal:** Generate devcontainer.json with capability and resource restrictions

**Components:**
- Modified `DevcontainerGenerator.Generate()` in `internal/container/devcontainer.go`
- Logic to populate `capAdd`, `securityOpt` fields on `DevcontainerJSON`
- Logic to append `--cap-drop`, `--memory`, `--cpus`, `--pids-limit` to `runArgs`

**Dependencies:** Phase 1 (isolation config available)

**Done when:** Generated devcontainer.json includes capability drops and resource limits from template config, tests verify correct runArgs generation
<!-- END_PHASE_2 -->

<!-- START_PHASE_3 -->
### Phase 3: RuntimeInterface Network Operations

**Goal:** Add network management to RuntimeInterface

**Components:**
- `CreateNetwork(ctx, name)` method in `internal/container/runtime.go`
- `RemoveNetwork(ctx, name)` method
- `RunContainer(ctx, opts RunContainerOptions)` method for running arbitrary containers
- `RunContainerOptions` struct with image, name, network, volumes, command, labels

**Dependencies:** None (parallel with Phase 2)

**Done when:** Can create/remove Docker networks, run containers with network attachment, mock runtime updated for testing
<!-- END_PHASE_3 -->

<!-- START_PHASE_4 -->
### Phase 4: Mitmproxy Filter Generation

**Goal:** Generate Python filter script and mitmproxy command from allowlist config

**Components:**
- `GenerateFilterScript(allowlist []string)` function in new file `internal/container/proxy.go`
- `GenerateProxyCommand(passthroughPinned []string)` function for `--ignore-hosts` flags
- Template for filter.py with embedded domain list
- File writing to `~/.local/share/devagent/proxy-configs/<hash>/filter.py`

**Dependencies:** Phase 1 (network config available)

**Done when:** Filter script generated correctly from allowlist, passthrough domains converted to `--ignore-hosts` regex patterns, tests verify script content
<!-- END_PHASE_4 -->

<!-- START_PHASE_5 -->
### Phase 5: CA Certificate Management

**Goal:** Manage mitmproxy CA certificates and inject into devcontainers

**Components:**
- `EnsureProxyCertDir(projectPath)` in `internal/container/proxy.go` — creates/returns cert directory
- Modified `DevcontainerGenerator.Generate()` to add cert mount and `postCreateCommand` for `update-ca-certificates`
- Logic to chain with existing `postCreateCommand` (string or array)

**Dependencies:** Phase 2 (devcontainer generation), Phase 4 (proxy config)

**Done when:** Cert directory created, mount added to devcontainer, postCreateCommand installs cert, tests verify mount and command generation
<!-- END_PHASE_5 -->

<!-- START_PHASE_6 -->
### Phase 6: Sidecar Container Management

**Goal:** Start/stop mitmproxy sidecar containers tied to devcontainer lifecycle

**Components:**
- `Sidecar` struct in `internal/container/types.go`
- `sidecars map[string]*Sidecar` field in `Manager`
- Modified `Manager.Create()` to create network and start proxy before devcontainer
- Modified `Manager.Start()` to start proxy first
- Modified `Manager.Stop()` to stop proxy after devcontainer
- Modified `Manager.Destroy()` to clean up proxy and network
- Modified `Manager.Refresh()` to populate sidecars from labels

**Dependencies:** Phase 3 (runtime network ops), Phase 4 (filter generation), Phase 5 (cert management)

**Done when:** Proxy container starts before devcontainer, stops after, cleanup on destroy, orphaned sidecars cleaned up on refresh
<!-- END_PHASE_6 -->

<!-- START_PHASE_7 -->
### Phase 7: Proxy Environment Configuration

**Goal:** Configure devcontainer to route traffic through proxy

**Components:**
- Modified `DevcontainerGenerator.Generate()` to add proxy environment variables
- Network attachment via `runArgs` (`--network=<name>`)
- `HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`, `REQUESTS_CA_BUNDLE`, `NODE_EXTRA_CA_CERTS` in `containerEnv`

**Dependencies:** Phase 5 (cert injection), Phase 6 (sidecar management)

**Done when:** Devcontainer connects to isolated network, proxy env vars set, applications route through mitmproxy
<!-- END_PHASE_7 -->

<!-- START_PHASE_8 -->
### Phase 8: Default Isolation and Allowlist Extension

**Goal:** Apply isolation by default and support allowlist extension

**Components:**
- `MergeIsolationConfig(template, defaults)` function in `internal/config/templates.go`
- `allowlistExtend` field support in `NetworkConfig`
- Logic to merge extended domains with defaults rather than replace
- Apply `DefaultIsolation` when template has no `isolation` key

**Dependencies:** All previous phases

**Done when:** Templates without isolation config get defaults applied, `allowlistExtend` merges with defaults, `isolation.enabled: false` disables all isolation
<!-- END_PHASE_8 -->

## Additional Considerations

**Proxy startup timing:** The mitmproxy container must be healthy before devcontainer starts. Consider adding a health check or brief sleep to ensure proxy is accepting connections.

**Allowlist maintenance:** The default allowlist (`api.anthropic.com`, `github.com`, etc.) may need updates as Claude Code or GitHub change domains. Consider documenting how users can extend the list.

**Debugging blocked requests:** When a request is blocked, the 403 response includes the blocked domain. Users can check mitmproxy logs or the response body to identify what to add to their allowlist.

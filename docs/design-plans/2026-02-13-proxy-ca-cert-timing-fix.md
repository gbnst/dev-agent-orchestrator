# Proxy CA Cert Timing Fix

## Summary

This design fixes a container startup timing issue that causes VS Code's "Reopen in Container" feature to hang when using mitmproxy for network isolation. The problem occurs because VS Code needs to download its server component through the TLS-intercepting proxy, but the proxy's CA certificate isn't trusted until after VS Code has already connected and timed out waiting for downloads. The fix moves CA certificate installation from the post-create hook (which runs after VS Code connects) into a container entrypoint script (which runs before VS Code connects). Additionally, the proxy service now declares a healthcheck that verifies its CA certificate file exists, preventing the app container from starting until the proxy is truly ready.

The implementation touches all three devagent templates (basic, go-project, python-fullstack) by: (1) adding Docker Compose healthchecks that gate app container startup on proxy readiness, (2) creating entrypoint scripts that install the mitmproxy CA cert into the system trust store before VS Code connects, and (3) adding VS Code CDN and marketplace domains to the proxy's allowlist.

## Definition of Done
Update all three devagent templates (basic, go-project, python-fullstack) so that containers using the mitmproxy sidecar work correctly with VS Code "Reopen in Container" by:

1. Adding a healthcheck to the proxy service that verifies the CA cert exists, and changing the app service dependency from `service_started` to `service_healthy`.
2. Creating an `entrypoint.sh` that installs the mitmproxy CA cert into the system trust store (with a wait loop) before VS Code connects, and removing the CA cert installation from `post-create.sh`.
3. Adding VS Code CDN/marketplace domains to each template's proxy allowlist and emptying the passthrough list.

**Success criteria:** A fresh container created from any template can be opened with VS Code "Reopen in Container" without hanging on VS Code Server download.

**Out of scope:** Resource limits, port forwarding, extra volumes, selective logging patterns, or any other project-specific configuration.

## Glossary

- **mitmproxy**: An open-source TLS-intercepting proxy that sits between the container and the internet, allowing devagent to enforce domain allowlists for network isolation.
- **CA certificate (CA cert)**: The root certificate authority certificate that mitmproxy generates for TLS interception. Systems must trust this cert for TLS connections through the proxy to succeed.
- **Docker Compose healthcheck**: A container health verification mechanism that tests whether a service is truly ready (not just started), allowing dependent services to wait for readiness.
- **entrypoint.sh**: A script that runs as the container's entrypoint before any devcontainer lifecycle hooks execute, making it the earliest point where code can run after container creation.
- **system trust store**: The operating system's central repository of trusted CA certificates, updated via `update-ca-certificates` on Debian/Ubuntu.
- **sidecar container**: A supporting container that runs alongside the main application container, providing infrastructure services like proxying.
- **allowlist**: A list of permitted domains that the proxy allows traffic to reach, blocking all other destinations for network isolation.
- **postCreateCommand**: A devcontainer lifecycle hook that runs after VS Code connects to the container, typically used for project setup like installing dependencies.
- **`exec "$@"`**: A shell pattern that replaces the current process with the command passed as arguments, used in entrypoint scripts to preserve the container's intended main process (in this case, `sleep infinity`).
- **`service_healthy` vs `service_started`**: Docker Compose dependency conditions — `service_started` means the container process launched, while `service_healthy` means the container's healthcheck passed.

## Architecture

The root cause is a timing issue: VS Code needs to download its server component through the mitmproxy sidecar, but the mitmproxy CA certificate isn't trusted until `postCreateCommand` runs — which happens *after* VS Code connects. Without a trusted CA, TLS handshakes fail silently and VS Code hangs.

The fix addresses three layers:

1. **Proxy readiness gate.** The proxy service declares a healthcheck that verifies its CA certificate file exists. The app service dependency changes from `service_started` to `service_healthy`, so Docker won't start the app container until the proxy's CA cert is generated.

2. **Early cert installation via entrypoint.** A new `entrypoint.sh` script runs before VS Code connects (entrypoint executes before the container is "ready" for devcontainer attachment). It waits for the CA cert file, copies it to the system trust store, and runs `update-ca-certificates`. The `exec "$@"` pattern preserves the existing `sleep infinity` command. The CA cert installation block is removed from `post-create.sh`.

3. **VS Code domains in allowlist.** VS Code Server downloads, extension marketplace, and CDN domains are added to each template's `filter.py` allowlist. With the CA cert trusted before VS Code connects, normal TLS interception works — no passthrough needed.

The container startup sequence becomes:
```
docker compose up
  → proxy starts, generates CA cert
  → proxy healthcheck passes (cert file exists)
  → app starts, entrypoint.sh runs
  → entrypoint installs CA cert into system trust store
  → exec "sleep infinity" keeps container alive
  → VS Code connects, downloads server through proxy (TLS works)
  → postCreateCommand runs project-specific setup
```

## Existing Patterns

All three templates (`basic`, `go-project`, `python-fullstack`) share identical `docker-compose.yml.tmpl` structure and identical CA cert installation code in `post-create.sh`. The proxy `filter.py` files differ only in their domain allowlists (language-specific package registries).

This design follows the existing pattern of keeping template files identical across templates where possible. The new `entrypoint.sh` will be identical across all three. The compose and post-create changes are identical. Only the `filter.py` changes add the same VS Code domains to each template's existing allowlist.

No new Go template variables are needed — `entrypoint.sh` is a static file (no `.tmpl` extension) and the compose changes use hardcoded values.

## Implementation Phases

<!-- START_PHASE_1 -->
### Phase 1: Proxy Healthcheck and Service Dependency

**Goal:** Ensure the app container doesn't start until the proxy's CA certificate exists.

**Components:**
- `config/templates/basic/.devcontainer/docker-compose.yml.tmpl` — add healthcheck to proxy service, change app dependency to `service_healthy`
- `config/templates/go-project/.devcontainer/docker-compose.yml.tmpl` — same changes
- `config/templates/python-fullstack/.devcontainer/docker-compose.yml.tmpl` — same changes

**Dependencies:** None (first phase)

**Done when:** `docker compose config` validates successfully for a rendered template. Proxy service has healthcheck block. App service depends on proxy with `condition: service_healthy`.
<!-- END_PHASE_1 -->

<!-- START_PHASE_2 -->
### Phase 2: Entrypoint CA Cert Installation

**Goal:** Install the mitmproxy CA cert before VS Code connects, replacing the post-create approach.

**Components:**
- New file `config/templates/basic/.devcontainer/entrypoint.sh` — waits for CA cert, installs to system trust store, `exec "$@"`
- New file `config/templates/go-project/.devcontainer/entrypoint.sh` — identical
- New file `config/templates/python-fullstack/.devcontainer/entrypoint.sh` — identical
- `config/templates/basic/.devcontainer/docker-compose.yml.tmpl` — add `entrypoint` directive pointing to `{{.WorkspaceFolder}}/.devcontainer/entrypoint.sh`
- `config/templates/go-project/.devcontainer/docker-compose.yml.tmpl` — same
- `config/templates/python-fullstack/.devcontainer/docker-compose.yml.tmpl` — same
- `config/templates/basic/.devcontainer/post-create.sh` — remove CA cert installation block
- `config/templates/go-project/.devcontainer/post-create.sh` — remove CA cert block, keep `go mod download`
- `config/templates/python-fullstack/.devcontainer/post-create.sh` — remove CA cert block, keep `uv sync`

**Dependencies:** Phase 1 (healthcheck ensures cert exists before entrypoint runs)

**Done when:** `entrypoint.sh` exists in all three templates. Compose templates reference the entrypoint. `post-create.sh` no longer contains CA cert installation code.
<!-- END_PHASE_2 -->

<!-- START_PHASE_3 -->
### Phase 3: VS Code Domains in Proxy Allowlist

**Goal:** Allow VS Code Server downloads and extension marketplace traffic through the proxy.

**Components:**
- `config/templates/basic/.devcontainer/containers/proxy/opt/devagent-proxy/filter.py` — add VS Code domains to ALLOWED_DOMAINS
- `config/templates/go-project/.devcontainer/containers/proxy/opt/devagent-proxy/filter.py` — same
- `config/templates/python-fullstack/.devcontainer/containers/proxy/opt/devagent-proxy/filter.py` — same

VS Code domains to add:
- `update.code.visualstudio.com`
- `*.vscode-cdn.net`
- `vscode.download.prss.microsoft.com`
- `az764295.vo.msecnd.net`
- `*.gallerycdn.vsassets.io`
- `marketplace.visualstudio.com`
- `*.vo.msecnd.net`

**Dependencies:** None (independent of Phases 1-2, but logically completes the fix)

**Done when:** All three `filter.py` files include the VS Code domains in ALLOWED_DOMAINS.
<!-- END_PHASE_3 -->

## Additional Considerations

**Entrypoint uses `/bin/sh`, not `/bin/bash`.** The entrypoint script uses POSIX shell for maximum compatibility across base images. The `post-create.sh` scripts continue using bash since they run after the container is fully started.

**Wait timeout reduced from 30s to 10s.** The healthcheck is the primary readiness gate. The entrypoint wait loop is a belt-and-suspenders fallback. 10 seconds is sufficient since the healthcheck already guarantees the cert exists before the app container starts.

**Passthrough list already empty.** The current templates already have `PASSTHROUGH_DOMAINS: list[str] = []`. No change needed there — just adding the VS Code domains to the allowlist is sufficient.

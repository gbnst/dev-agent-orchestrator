# Template File Consolidation Design

## Summary

This design consolidates template configuration by embedding all container orchestration settings directly into template files, eliminating the separate `devcontainer.json` and `allowlist.txt` files and the Go code that parses them. Currently, templates store their configuration across multiple formats: `devcontainer.json` contains isolation settings (capabilities, resources, network allowlists) that are parsed by Go code at runtime, and `allowlist.txt` files list allowed domains that are processed into mitmproxy filter scripts. This split requires significant Go infrastructure for parsing, merging, and transforming configuration data.

The new approach makes template directories the single source of truth by converting all configuration into Go templates (`.tmpl` files) processed at container creation time. The `filter.py.tmpl` replaces both `allowlist.txt` and `GenerateFilterScript()` by embedding allowed domains, passthrough domains, and flags directly as Python code. Passthrough domains use mitmproxy's addon API (`ctx.options.ignore_hosts`) instead of CLI flag generation. The `docker-compose.yml.tmpl` already contains hardcoded capabilities and resource limits. The `devcontainer.json.tmpl` becomes self-contained per template with its own postCreateCommand. This eliminates all Go code that parsed isolation config, built proxy commands, generated filter scripts, and merged settings — shrinking the `Template` struct to just `Name`, `Path`, and `PostCreateCommand`.

## Definition of Done

- `devcontainer.json` removed from all template directories (basic, go-project)
- `allowlist.txt` removed from all template directories
- `filter.py.tmpl` added to each template with inlined allowlist domains, blockGitHubPRMerge flag, and passthrough domains
- `devcontainer.json.tmpl` per template includes its own postCreateCommand (with cert install suffix)
- Template discovery in `config.LoadTemplatesFrom()` uses `docker-compose.yml.tmpl` as the marker file
- `Template` struct stripped to: `Name`, `Path`, `PostCreateCommand`
- Removed Go code: `LoadAllowlist`, `AllowlistConfig`, `GenerateFilterScript`, `filterScriptTemplate`, `GenerateIgnoreHostsPattern`, `buildProxyCommand`, `parseIsolationConfig`, `getEffectiveIsolation`, `copyIsolationConfig`, `IsolationConfig`, `CapConfig`, `ResourceConfig`, `NetworkConfig`
- `ComposeOptions.Isolation` removed
- `TemplateData.ProxyCommand` removed (proxy command hardcoded in docker-compose.yml.tmpl)
- `ReadAllowlistFromFilterScript` still works (deployed filter.py has same `ALLOWED_DOMAINS` format)
- All existing tests updated or removed to reflect new structure
- `make test` and `make lint` pass (no new lint issues in changed files)

## Glossary

- **CA certificate**: A digital certificate used to verify website identity; mitmproxy generates its own CA cert that must be installed in containers to enable HTTPS interception
- **Devcontainer**: A specification and CLI tool (`@devcontainers/cli`) for creating reproducible development environments inside Docker containers using configuration files
- **Docker Compose**: A tool for defining and running multi-container applications using declarative YAML files; used here to orchestrate both the development container and its proxy sidecar
- **Go template (text/template)**: Go's standard library templating engine that processes files with `{{.Variable}}` placeholders to generate output with instance-specific values substituted
- **Mitmproxy**: A transparent HTTP/HTTPS proxy that can intercept, inspect, and filter network traffic; used here to enforce domain allowlists
- **postCreateCommand**: A devcontainer property that specifies shell commands to run after the container is created; used here to install mitmproxy's CA certificate into the system trust store
- **Sidecar pattern**: A container design pattern where an auxiliary container runs alongside the main application container to provide supporting functionality (here, network filtering)
- **Passthrough domains**: Domains that bypass mitmproxy's TLS interception entirely, needed for services that use certificate pinning

## Architecture

Template directories become the single source of truth for all container orchestration config. Each template directory contains five files:

```
config/templates/<name>/
├── Dockerfile              # App container image (static)
├── Dockerfile.proxy        # Proxy sidecar image (static)
├── docker-compose.yml.tmpl # Compose orchestration (Go template)
├── devcontainer.json.tmpl  # Devcontainer CLI config (Go template)
└── filter.py.tmpl          # Mitmproxy filter script (Go template)
```

All instance-specific values (project hash, project path, container name, workspace folder, claude config dir) are substituted via Go's `text/template` at generation time. Everything else is hardcoded in the template files themselves.

**Data flow at container creation:**

1. `ComposeGenerator.Generate()` processes `docker-compose.yml.tmpl` and `filter.py.tmpl` with `TemplateData`
2. `DevcontainerGenerator.Generate()` processes `devcontainer.json.tmpl` with `TemplateData`
3. `Dockerfile.proxy` is loaded as a static file
4. All output is written to the project's `.devcontainer/` directory

**What moves where:**

| Was in | Data | Moves to |
|--------|------|----------|
| `devcontainer.json` | isolation.caps, isolation.resources | Hardcoded in `docker-compose.yml.tmpl` |
| `devcontainer.json` | isolation.network.allowlist | Inlined in `filter.py.tmpl` |
| `devcontainer.json` | isolation.network.blockGitHubPRMerge | Flag in `filter.py.tmpl` |
| `devcontainer.json` | isolation.network.passthrough | Passthrough list in `filter.py.tmpl` (set via `ctx.options.ignore_hosts`) |
| `devcontainer.json` | postCreateCommand | Hardcoded per-template in `devcontainer.json.tmpl` |
| `devcontainer.json` | features | Already in `Dockerfile` |
| `devcontainer.json` | name, image, remoteUser, injectCredentials, defaultAgent | Unused in compose mode; deleted |
| `allowlist.txt` | domains, flags, passthrough | All moved to `filter.py.tmpl` |

**Passthrough domains via mitmproxy addon API:** Instead of generating `--ignore-hosts` regex on the CLI, the filter script sets `ctx.options.ignore_hosts` in its `load()` hook. This eliminates `GenerateIgnoreHostsPattern()` and the conditional proxy command logic. The proxy command in `docker-compose.yml.tmpl` becomes a static string.

**Template struct simplification:** The `config.Template` struct drops all fields that only existed to pass data from `devcontainer.json` into Go code. It retains `Name` (from directory name), `Path` (directory path), and `PostCreateCommand` (read from a minimal metadata source — see Phase 1).

**`TemplateData` changes:**

- Remove: `ProxyCommand` (hardcoded in template)
- Add: `AllowedDomains []string`, `BlockGitHubPRMerge string`, `PassthroughDomains []string` — but these are NOT populated by Go code. They don't exist on the struct because domains are hardcoded directly in `filter.py.tmpl`. The template IS the data.
- Keep: `ProjectHash`, `ProjectPath`, `ProjectName`, `WorkspaceFolder`, `ClaudeConfigDir`, `TemplateName`, `ContainerName`, `PostCreateCommand`, `CertInstallCommand`

Correction: since domains are hardcoded in the template files, `TemplateData` does NOT need allowlist fields. The only new field is `CertInstallCommand` (replacing the Go-side cert install chaining logic).

## Existing Patterns

This design extends the template-driven approach introduced in the docker-compose orchestration work (commit `80a3b57`). That work already moved docker-compose.yml generation from Go string building to `docker-compose.yml.tmpl` and added `devcontainer.json.tmpl`. This design completes that migration by also making `filter.py` template-driven and eliminating the remaining legacy files.

Template loading follows the existing pattern in `config/templates.go:LoadTemplatesFrom()` — scan subdirectories, look for a marker file, extract minimal metadata. The marker file changes from `devcontainer.json` to `docker-compose.yml.tmpl`.

The `processTemplate()` function in `compose.go` already handles Go template processing for `.tmpl` files. `filter.py.tmpl` reuses this same function.

## Implementation Phases

<!-- START_PHASE_1 -->
### Phase 1: Template loading and struct cleanup

**Goal:** Change template discovery to use `docker-compose.yml.tmpl` as the marker file. Strip `Template` struct to minimal fields. Remove isolation config parsing.

**Components:**
- `internal/config/templates.go` — change `LoadTemplatesFrom()` marker from `devcontainer.json` to `docker-compose.yml.tmpl`, simplify `loadTemplate()` to read only `PostCreateCommand` (from a small JSON or YAML sidecar, or parse from devcontainer.json.tmpl), strip `Template` struct
- `internal/config/templates.go` — delete `parseIsolationConfig()`, `getEffectiveIsolation()`, `copyIsolationConfig()`, `IsolationConfig`, `CapConfig`, `ResourceConfig`, `NetworkConfig`
- `internal/config/templates_test.go` — update all template loading tests

**Dependencies:** None

**Done when:** `LoadTemplatesFrom()` discovers templates via `docker-compose.yml.tmpl`, `Template` struct has only `Name`/`Path`/`PostCreateCommand`, all template loading tests pass
<!-- END_PHASE_1 -->

<!-- START_PHASE_2 -->
### Phase 2: Add filter.py.tmpl and remove allowlist.txt

**Goal:** Create `filter.py.tmpl` template files, remove `allowlist.txt`, and update `ComposeGenerator` to process the new template.

**Components:**
- `config/templates/basic/filter.py.tmpl` — mitmproxy filter with inlined allowlist, blockGitHubPRMerge flag, passthrough domains using `ctx.options.ignore_hosts` in `load()` hook
- `config/templates/go-project/filter.py.tmpl` — same structure, same domains (both templates currently share the same allowlist)
- `internal/container/compose.go` — replace `LoadAllowlist()` + `GenerateFilterScript()` with `processTemplate()` call on `filter.py.tmpl`, delete `LoadAllowlist`, `AllowlistConfig`, `buildProxyCommand`
- `internal/container/proxy.go` — delete `GenerateFilterScript`, `filterScriptTemplate`, `GenerateIgnoreHostsPattern`
- `internal/container/compose.go` — remove `ProxyCommand` from `TemplateData`, add `CertInstallCommand`
- Delete `config/templates/basic/allowlist.txt` and `config/templates/go-project/allowlist.txt`

**Dependencies:** Phase 1 (Template struct no longer has Isolation field)

**Done when:** `ComposeGenerator.Generate()` processes `filter.py.tmpl` instead of generating filter script from parsed allowlist, `allowlist.txt` files deleted, compose and proxy tests pass
<!-- END_PHASE_2 -->

<!-- START_PHASE_3 -->
### Phase 3: Consolidate devcontainer.json.tmpl and remove devcontainer.json

**Goal:** Embed `postCreateCommand` (with cert install) directly in each template's `devcontainer.json.tmpl`. Remove static `devcontainer.json` files. Update `DevcontainerGenerator`.

**Components:**
- `config/templates/basic/devcontainer.json.tmpl` — add `{{.CertInstallCommand}}` as postCreateCommand (basic has no template-specific command)
- `config/templates/go-project/devcontainer.json.tmpl` — hardcode `"go mod download || true && {{.CertInstallCommand}}"` as postCreateCommand
- `internal/container/devcontainer.go` — simplify `generateFromTemplate()` to no longer read `tmpl.PostCreateCommand` or chain cert install in Go; the template handles it
- `internal/container/compose.go` — remove `PostCreateCommand` from `TemplateData` (moved to template), keep `CertInstallCommand`
- Delete `config/templates/basic/devcontainer.json` and `config/templates/go-project/devcontainer.json`

**Dependencies:** Phase 1 (template loading no longer reads devcontainer.json)

**Done when:** `devcontainer.json` files deleted, `devcontainer.json.tmpl` is self-contained per template, devcontainer generation tests pass
<!-- END_PHASE_3 -->

<!-- START_PHASE_4 -->
### Phase 4: Clean up ComposeOptions and manager

**Goal:** Remove `Isolation` from `ComposeOptions` and clean up the manager's compose creation path.

**Components:**
- `internal/container/compose.go` — remove `Isolation` field from `ComposeOptions`, remove fallback logic that read from `opts.Isolation`
- `internal/container/manager.go` — remove `tmpl.GetEffectiveIsolation()` call and `effectiveIsolation` variable in `CreateWithCompose()`
- `internal/container/types.go` — remove any unused types related to isolation or proxy config that are no longer referenced
- `internal/container/manager_test.go` — update compose creation tests
- `internal/container/compose_test.go` — update all compose generation tests to not pass Isolation

**Dependencies:** Phase 2, Phase 3

**Done when:** No references to `IsolationConfig` remain in container package, `ComposeOptions` has no `Isolation` field, all tests pass
<!-- END_PHASE_4 -->

<!-- START_PHASE_5 -->
### Phase 5: Simplify docker-compose.yml.tmpl proxy command

**Goal:** Hardcode the proxy command in `docker-compose.yml.tmpl` now that `--ignore-hosts` is handled by the filter script.

**Components:**
- `config/templates/basic/docker-compose.yml.tmpl` — replace `{{.ProxyCommand}}` with static command `["mitmdump", "--listen-host", "0.0.0.0", "--listen-port", "8080", "-s", "/home/mitmproxy/filter.py"]`
- `config/templates/go-project/docker-compose.yml.tmpl` — same change
- `internal/container/compose.go` — remove `ProxyCommand` field from `TemplateData` if not already removed

**Dependencies:** Phase 2 (filter.py.tmpl handles passthrough via `ctx.options.ignore_hosts`)

**Done when:** No `ProxyCommand` in `TemplateData`, proxy command is static in templates, compose tests pass
<!-- END_PHASE_5 -->

## Additional Considerations

**`ReadAllowlistFromFilterScript` compatibility:** The deployed `filter.py` files written to `.devcontainer/` retain the same `ALLOWED_DOMAINS = [...]` Python format. The `ReadAllowlistFromFilterScript()` function in `proxy.go` (used by `GetContainerIsolationInfo()` for TUI display) continues to work without changes.

**`WriteFilterScript` in proxy.go:** This function is used by the non-compose sidecar path. It may now be dead code if all orchestration goes through compose. Verify during implementation and remove if unused.

**Template `PostCreateCommand` loading:** With `devcontainer.json` gone, `LoadTemplatesFrom()` needs a way to discover `PostCreateCommand` for templates that have one (only go-project currently). Options: parse it from `devcontainer.json.tmpl` (fragile), or accept that it's now hardcoded in the template and remove `PostCreateCommand` from the `Template` struct entirely. The latter is cleaner — each `devcontainer.json.tmpl` is self-contained.

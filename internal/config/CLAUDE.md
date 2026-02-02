# Config Domain

Last verified: 2026-02-01

## Purpose
Loads and validates application configuration (config.yaml) and devcontainer templates. Provides isolation configuration schema and defaults for container security settings.

## Contracts
- **Exposes**: `Config`, `Template`, `IsolationConfig`, `CapConfig`, `ResourceConfig`, `NetworkConfig`, `DefaultIsolation`, `MergeIsolationConfig`, `LoadTemplates`
- **Guarantees**: Templates loaded from `~/.config/devagent/templates/` (XDG-compliant). DefaultIsolation provides secure defaults. MergeIsolationConfig handles template override and allowlistExtend merging.
- **Expects**: Valid YAML/JSON in config and template files. Template directories contain devcontainer.json.

## Dependencies
- **Uses**: os, encoding/json (stdlib only)
- **Used by**: container.Manager, container.DevcontainerGenerator, TUI
- **Boundary**: Configuration loading only; no container operations

## Key Decisions
- Template.Isolation parsed from `customizations.devagent.isolation` in devcontainer.json
- DefaultIsolation applied when template has no isolation config; disabled with `enabled: false`
- AllowlistExtend appends to defaults; Allowlist replaces defaults entirely
- Passthrough domains bypass TLS interception (for cert-pinned services)
- GetEffectiveIsolation() on Template returns merged config with defaults

## Invariants
- DefaultIsolation never nil; provides sensible security defaults
- MergeIsolationConfig returns nil only when isolation explicitly disabled
- Template isolation settings override defaults (not merged additively, except allowlistExtend)
- Capability drops/adds from template replace defaults, not append

## Key Files
- `config.go` - Config struct, loading, credential management
- `templates.go` - Template loading, IsolationConfig schema, DefaultIsolation, MergeIsolationConfig

## Default Isolation Values
- Caps.Drop: NET_RAW, SYS_ADMIN, SYS_PTRACE, MKNOD, NET_ADMIN, SYS_MODULE, SYS_RAWIO, SYS_BOOT, SYS_NICE, SYS_RESOURCE
- Resources: 4GB memory, 2 CPUs, 512 pids limit
- Network.Allowlist: api.anthropic.com, github.com, *.github.com, registry.npmjs.org, pypi.org, proxy.golang.org, etc.

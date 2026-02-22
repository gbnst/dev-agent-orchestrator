# Config Domain

Last verified: 2026-02-22

## Purpose
Loads and validates application configuration (config.yaml) and devcontainer templates.

## Contracts
- **Exposes**: `Config`, `WebConfig`, `TailscaleConfig`, `Template`, `LoadTemplates`, `LoadTemplatesFrom`, `SetTemplatesPath`, `ResolvePathFunc`, `LookPathFunc`
- **Guarantees**: Templates loaded from `~/.config/devagent/templates/` (XDG-compliant). Templates discovered by presence of `docker-compose.yml.tmpl` marker file. Template struct contains only `Name` and `Path`. Token paths (`ClaudeTokenPath`, `GitHubTokenPath`) are user-configurable; `ResolveTokenPath()` expands `~/` prefix. If a token path is empty/omitted, that token is skipped entirely. `TailscaleConfig.Validate()` checks name non-empty, funnel_only requires funnel, auth key file exists on disk.
- **Expects**: Valid YAML in config files. Template directories contain `docker-compose.yml.tmpl`.

## Dependencies
- **Uses**: os, path/filepath (stdlib only)
- **Used by**: container.Manager, container.ComposeGenerator, container.DevcontainerGenerator, TUI, web.Server, tsnsrv
- **Boundary**: Configuration loading only; no container operations

## Key Decisions
- Template discovery uses `docker-compose.yml.tmpl` as marker file (not `devcontainer.json`)
- All orchestration config (caps, resources, network allowlists) is hardcoded in template files
- No `IsolationConfig` types — isolation is entirely template-driven

## Invariants
- Template.Name always equals the directory name
- Template.Path is the absolute path to the template directory
- Templates without `docker-compose.yml.tmpl` are ignored during discovery

## Key Files
- `config.go` - Config struct, loading
- `templates.go` - Template loading, discovery

# Config Domain

Last verified: 2026-06-10

## Purpose
Loads and validates application configuration (config.yaml) and devcontainer templates, and provisions both into the user profile from defaults embedded in the binary.

## Contracts
- **Exposes**: `Config`, `WebConfig`, `TailscaleConfig`, `Template`, `LoadTemplates`, `LoadTemplatesFrom`, `SetTemplatesPath`, `ResolvePathFunc`, `LookPathFunc`, `ResolveScanPaths`, `DefaultConfigDir`, `BuiltinAssets`, `ProvisionResult`, `EnsureUserConfig`, `PlanTemplateSync`, `TemplateSyncPlan`, `TemplatesNeedSync`
- **Guarantees**: Templates loaded from `~/.config/devagent/templates/` (XDG-compliant; single location — no override/merge layering). Templates discovered by presence of `docker-compose.yml.tmpl` marker file. Template struct contains only `Name` and `Path`. Token paths (`ClaudeTokenPath`, `GitHubTokenPath`) are user-configurable; `ResolveTokenPath()` expands `~/` prefix. If a token path is empty/omitted, that token is skipped entirely. `TailscaleConfig.Validate()` checks name non-empty, funnel_only requires funnel, auth key file exists on disk. `ScanPaths` is an optional list of directories for project discovery; `ResolveScanPaths()` expands `~/` in each path.
- **Provisioning**: `EnsureUserConfig(configDir, BuiltinAssets, now)` materializes the embedded defaults (templates fs.FS + default config.yaml, supplied by `main` via `//go:embed`) into the profile. `config.yaml` is written only when absent (never overwritten — holds user secrets/paths). Templates are (re)written when `.templates-version` marker ≠ binary version (`TemplatesNeedSync`); per-file plan from `PlanTemplateSync` writes new/changed files and backs up on-disk files that DIVERGE from the embedded copy to `templates.backup-<now>/` before overwriting. On-disk files absent from the embed are left untouched (user-added templates survive). This is the mechanism by which template/security fixes reach existing users on upgrade, and it self-heals pre-marker profiles (empty marker ⇒ resync). Only `main` provisions the default profile (`runTUI`, guarded on empty `--config-dir`); an explicit `--config-dir` like `make dev` is never provisioned.
- **Expects**: Valid YAML in config files. Template directories contain `docker-compose.yml.tmpl`. `main` injects `BuiltinAssets` (templates re-rooted via `fs.Sub` to the templates dir).

## Dependencies
- **Uses**: os, path/filepath (stdlib only)
- **Used by**: container.Manager, container.ComposeGenerator, TUI, web.Server, tsnsrv, discovery (via ResolveScanPaths)
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
- `config.go` - Config struct, loading, `DefaultConfigDir`
- `templates.go` - Template loading, discovery
- `provision_plan.go` - Functional Core: `PlanTemplateSync` (per-file write/backup plan), `TemplatesNeedSync` (version-marker check)
- `provision.go` - Imperative Shell: `EnsureUserConfig` seeds config.yaml + syncs embedded templates into the profile (conflict-backup, version marker)

## Embedding
- Defaults are embedded in the binary by `main` (`assets.go`): `//go:embed all:config/templates` (the `all:` prefix is required to include `.devcontainer/` and `.gitignore.tmpl`) and `//go:embed config/config.default.yaml` (curated default, distinct from the dev `config/config.yaml`). Releases ship only the binary; the embed + `EnsureUserConfig` is what populates the profile.

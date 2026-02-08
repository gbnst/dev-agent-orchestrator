# Image Cleanup Design

## Summary

The current Dockerfiles copy dotfiles and config files into images during build, which breaks Docker's layer cache every time you create a new container. This design moves those files to volume mounts instead, so Dockerfiles only contain `FROM` + `RUN` commands (installing packages and tools). With no project-specific content baked into images, Docker automatically hits its layer cache on subsequent builds using the same template.

This change requires consolidating scattered config. Right now, each project gets a directory under `~/.local/share/devagent/claude-configs/<hash>/` for Claude settings, plus separate template files copied to `.devcontainer/`. This design collapses everything into `.devcontainer/` — dotfiles under `home/vscode/`, proxy config under `proxy/`. The proxy also switches from building a custom Dockerfile to using the upstream `mitmproxy/mitmproxy:latest` image directly, with filter.py mounted as a volume.

## Definition of Done
1. Dockerfiles have no COPY commands — only FROM + RUN commands for package/tool installation
2. Dotfiles, .claude config, and filter.py are volume-mounted via compose instead of baked into images
3. All config consolidated under `<project>/.devcontainer/` — no more scattered directories in `~/.local/share/devagent/claude-configs/`
4. Dead code removed: `getContainerClaudeDir()`, `ensureClaudeDir()`, `copyClaudeTemplateFiles()`, `ClaudeConfigDir` field from TemplateData
5. Docker layer caching makes subsequent project creation fast automatically (no custom caching mechanism)
6. Proxy container uses a single volume mount for all host-side files (filter.py, logs, and any future needs)

## Glossary

- **Docker layer cache**: Docker's incremental build optimization that reuses unchanged layers from previous builds. When a Dockerfile has identical instructions up to a certain point, Docker skips rebuilding those layers.
- **Volume mount**: Docker mechanism for mapping a host filesystem path into a container. Changes to mounted files don't require rebuilding the image.
- **Devcontainer**: VS Code specification for containerized development environments, defined via `.devcontainer/devcontainer.json` and associated Dockerfiles or compose files.
- **mitmproxy**: Open-source HTTPS proxy used here as a sidecar container for network isolation and domain allowlisting.
- **Sidecar container**: A secondary container that runs alongside a primary container, providing supporting functionality (here, network filtering via mitmproxy).
- **filter.py**: Python script for mitmproxy that defines traffic filtering rules (domain allowlists, blocklists, PR merge blocking).
- **Go text/template**: Go's standard library templating engine, used to generate docker-compose.yml and devcontainer.json from `.tmpl` files with placeholder substitution.

## Architecture

Two changes that reinforce each other: simplify Dockerfiles by removing COPY commands, and consolidate scattered config into `.devcontainer/`.

**Why they're connected:** The Dockerfiles currently COPY dotfiles and config into images. Removing COPY requires an alternative — volume mounts. Volume mounts work best when the source files are organized in one place. The config consolidation provides that organization.

### Dockerfile simplification

App Dockerfiles (`config/templates/basic/Dockerfile`, `config/templates/go-project/Dockerfile`) lose their `COPY` line. What remains:

- `FROM` base image
- `RUN` package installation (tmux for basic template)
- `RUN curl claude.ai/install.sh | bash`

With no COPY and no project-specific content, Docker's layer cache hits automatically on every subsequent build using the same template. No custom caching mechanism needed.

`Dockerfile.proxy` is eliminated entirely. The proxy service switches from `build:` to `image: mitmproxy/mitmproxy:latest` in compose. filter.py is mounted as a volume instead of COPYed.

### Volume mount changes

**App container:** Replace the `ClaudeConfigDir` mount with a mount of `.devcontainer/home/vscode` → `/home/vscode`. This single mount covers dotfiles (`.bashrc`, `.zshrc`, `.tmux.conf`) and Claude config (`.claude/settings.json`). `WriteToProject()` already copies `template/home/` → `.devcontainer/home/`, so no new copy logic is needed.

**Proxy container:** Replace the separate `proxy-logs` mount with a single mount of `.devcontainer/proxy/` → `/opt/devagent-proxy/`. This directory contains `filter.py` and `logs/`. The `proxy-certs` named volume stays unchanged — it's shared between app and proxy containers for mitmproxy CA cert trust.

### Config consolidation

Stop creating `~/.local/share/devagent/claude-configs/<hash>/` directories. All per-project config lives under `<project>/.devcontainer/`:

```
<project>/.devcontainer/
├── docker-compose.yml          # Generated from template
├── devcontainer.json           # Generated from template
├── Dockerfile                  # Copied from template
├── .gitignore                  # Copied from template
├── home/vscode/                # Copied from template
│   ├── .bashrc
│   ├── .zshrc
│   ├── .tmux.conf
│   └── .claude/
│       └── settings.json
└── proxy/                      # Copied from template + runtime
    ├── filter.py               # Copied from template
    └── logs/                   # Created at runtime
```

### Template directory restructure

Template directories gain a `proxy/` subdirectory and a `.gitignore`. Static files (filter.py, .gitignore) are no longer processed as Go templates — they're copied verbatim.

```
config/templates/basic/
├── Dockerfile                  # Simplified (no COPY)
├── devcontainer.json.tmpl      # Go template (unchanged)
├── docker-compose.yml.tmpl     # Go template (modified mounts + proxy image)
├── .gitignore                  # Static, copied verbatim
├── home/vscode/                # Static, copied verbatim
│   ├── .bashrc
│   ├── .zshrc
│   ├── .tmux.conf
│   └── .claude/settings.json
└── proxy/
    └── filter.py               # Static, copied verbatim (was filter.py.tmpl)
```

## Existing patterns

`WriteToProject()` in `devcontainer.go:278-322` already copies template directories to `.devcontainer/`. It copies `Dockerfile` and the `home/` directory. This design extends that pattern to also copy `proxy/` and `.gitignore`.

`WriteComposeFiles()` in `devcontainer.go:324-369` writes generated compose files and creates the `proxy-logs/` directory. This design modifies it to create `proxy/logs/` instead and to stop writing `Dockerfile.proxy` and `filter.py` (both are now static template files copied by `WriteToProject()`).

The `ComposeGenerator` in `compose.go` processes `.tmpl` files through Go's `text/template`. This design removes `processFilterTemplate()` and `loadDockerfileProxy()` / `generateDockerfileProxy()` since filter.py becomes a static copy and Dockerfile.proxy is eliminated.

## Implementation phases

<!-- START_PHASE_1 -->
### Phase 1: Template restructure

**Goal:** Reorganize template directories to the new layout. Rename filter.py.tmpl to filter.py, add proxy/ subdirectory, add .gitignore, simplify Dockerfiles.

**Components:**
- `config/templates/basic/Dockerfile` — remove COPY line
- `config/templates/go-project/Dockerfile` — remove COPY line
- `config/templates/basic/Dockerfile.proxy` — delete
- `config/templates/go-project/Dockerfile.proxy` — delete
- `config/templates/basic/filter.py.tmpl` — move to `proxy/filter.py`, remove `.tmpl` extension
- `config/templates/go-project/filter.py.tmpl` — move to `proxy/filter.py`, remove `.tmpl` extension
- `config/templates/basic/.gitignore` — new static file
- `config/templates/go-project/.gitignore` — new static file

**Dependencies:** None

**Done when:** Template directories match the new layout. No functional changes yet — Go code still references old paths (will break until Phase 2).
<!-- END_PHASE_1 -->

<!-- START_PHASE_2 -->
### Phase 2: Update compose template and TemplateData

**Goal:** Modify docker-compose.yml.tmpl to use volume mounts instead of COPY, switch proxy to `image:` instead of `build:`, and update TemplateData.

**Components:**
- `config/templates/basic/docker-compose.yml.tmpl` — replace `ClaudeConfigDir` mount with `home/vscode` mount, replace proxy `build:` with `image:`, update proxy volumes to single `.devcontainer/proxy` mount, update mitmproxy command path
- `config/templates/go-project/docker-compose.yml.tmpl` — same changes
- `internal/container/compose.go` — remove `ClaudeConfigDir` from `TemplateData`, remove `processFilterTemplate()`, remove `loadDockerfileProxy()` / `generateDockerfileProxy()`, update `ComposeResult` to drop `DockerfileProxy` and `FilterScript` fields

**Dependencies:** Phase 1

**Done when:** Templates generate correct compose YAML with new mount paths. `TemplateData` has no `ClaudeConfigDir` field. Compose generation code no longer references filter.py or Dockerfile.proxy.
<!-- END_PHASE_2 -->

<!-- START_PHASE_3 -->
### Phase 3: Update file writing and config consolidation

**Goal:** Modify `WriteToProject()` to copy new static files (proxy/, .gitignore). Modify `WriteComposeFiles()` to stop writing Dockerfile.proxy and filter.py. Remove dead claude-config code.

**Components:**
- `internal/container/devcontainer.go` — update `WriteToProject()` to copy `proxy/` directory and `.gitignore` from template. Update `WriteComposeFiles()` to stop writing `Dockerfile.proxy` and `filter.py`, create `proxy/logs/` instead of `proxy-logs/`. Remove `getContainerClaudeDir()`, `ensureClaudeDir()`, `copyClaudeTemplateFiles()`, and all references to `~/.local/share/devagent/claude-configs/`.
- `internal/container/devcontainer.go` — update `Generate()` to stop calling `ensureClaudeDir()` and `copyClaudeTemplateFiles()`

**Dependencies:** Phase 2

**Done when:** Container creation uses new file layout. No references to `claude-configs` directory remain. `WriteToProject()` copies proxy/ and .gitignore. `WriteComposeFiles()` only writes docker-compose.yml. Tests pass.
<!-- END_PHASE_3 -->

<!-- START_PHASE_4 -->
### Phase 4: End-to-end verification

**Goal:** Verify the full container creation flow works with the new layout.

**Components:**
- Existing E2E tests in `internal/e2e/` — verify they pass with changes
- Manual verification: create a container, confirm dotfiles are mounted, confirm proxy has filter.py, confirm proxy logs work, confirm Claude config is present

**Dependencies:** Phase 3

**Done when:** E2E tests pass. A container created with the basic template has correct mounts, proxy filtering works, and Docker layer cache hits on second creation with same template.
<!-- END_PHASE_4 -->

## Additional considerations

**Existing containers:** No backward compatibility needed. Existing containers created with the old layout will need to be recreated. This is acceptable per project requirements.

**Proxy log path change:** The `.gitignore` in the template should ignore `proxy/logs/` and `*.jsonl`. The Go code that reads proxy logs (`internal/tui/` or `internal/container/`) may reference the old `proxy-logs/` path and will need updating if so.

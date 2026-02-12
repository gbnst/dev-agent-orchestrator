# GitHub CLI Authentication Design

## Summary

This design adds GitHub CLI authentication to devagent-managed containers by mirroring the existing Claude token injection pattern. Users place a GitHub Personal Access Token at `~/.config/github/token` on the host, and devagent mounts it read-only into containers at `/run/secrets/github-token`, then exports it as the `GH_TOKEN` environment variable via shell profiles. The implementation follows the established pattern from the Claude token design: a token resolution function reads the file (falling back to `/dev/null` if missing), template data wires the path through to docker-compose mounts, and shell profiles conditionally export the environment variable. Unlike Claude tokens, GitHub tokens are not auto-provisioned — the user is responsible for creating and placing the token file, and devagent logs a non-blocking warning if it's missing.

The design also introduces a small structural improvement: adding logger plumbing to `ComposeGenerator` so token-related warnings can be logged during template generation. The `gh` CLI is installed in template Dockerfiles via the official GitHub apt repository, and existing containers (like nfl_analysis, which already has `gh` installed) will need to be recreated to pick up the new mount and shell profile changes.

## Definition of Done

1. `gh` CLI is installed in template Dockerfiles
2. devagent reads `~/.config/github/token` from the host and mounts it read-only at `/run/secrets/github-token` in containers
3. Shell profiles (`.bashrc`/`.zshrc`) export `GH_TOKEN` from that mount
4. devagent logs a warning if the token file is missing (non-blocking)
5. Running `gh auth status` inside a container succeeds when a valid token is present

## Glossary

- **devagent**: The Go-based TUI orchestrator that manages devcontainer lifecycles via Docker Compose.
- **devcontainer**: A containerized development environment defined by a `.devcontainer/devcontainer.json` file. devagent creates and manages these using Docker Compose.
- **docker-compose.yml.tmpl**: A Go template file used to generate `docker-compose.yml` configuration for each managed container. Variables like `{{.GitHubTokenPath}}` are populated at generation time.
- **gh CLI**: GitHub's official command-line tool for interacting with GitHub (pull requests, issues, repositories, etc.). Authenticates via `GH_TOKEN` environment variable or config files.
- **GH_TOKEN**: Environment variable used by `gh` CLI for authentication. Contains a GitHub Personal Access Token.
- **GitHub Personal Access Token**: A token generated in GitHub settings that grants API access with scoped permissions (alternative to password authentication).
- **mitmproxy**: A transparent HTTP/HTTPS proxy used by devagent as a sidecar container to enforce domain allowlists for network isolation.
- **ScopedLogger**: A logging abstraction in devagent that prefixes log messages with a scope identifier (e.g., `container`, `tmux`, `tui`).
- **TemplateData**: A Go struct in `internal/container/compose.go` that holds variables used to populate Go template files like `docker-compose.yml.tmpl`.
- **XDG-aware**: Respects the XDG Base Directory specification (`$XDG_CONFIG_HOME`, defaulting to `~/.config/` if unset) for locating user config files.

## Architecture

Mirror the existing Claude token injection pattern for GitHub CLI authentication. A user-managed token file on the host is mounted read-only into containers and exported as `GH_TOKEN` via shell profiles.

**Token flow:**
1. User places a GitHub Personal Access Token at `~/.config/github/token` on the host
2. `ensureGitHubToken()` in `internal/container/devcontainer.go` reads the file path (XDG-aware: `$XDG_CONFIG_HOME/github/token`)
3. If the file is missing, a warning is logged and `/dev/null` is used as the mount source (same Docker empty-mount pattern as Claude token)
4. `buildTemplateData()` in `internal/container/compose.go` populates `GitHubTokenPath` in `TemplateData`
5. `docker-compose.yml.tmpl` mounts `{{.GitHubTokenPath}}:/run/secrets/github-token:ro`
6. Shell profiles (`.bashrc`/`.zshrc`) export `GH_TOKEN` from the mounted file at container startup

**Logger plumbing:** `ComposeGenerator` gains a `logger` field (`*logging.ScopedLogger`) so `ensureGitHubToken()` can log warnings when the token file is missing. The logger is passed from `Manager` via `NewComposeGenerator()`.

**gh CLI installation:** The basic template Dockerfile installs `gh` from the official GitHub apt repository. The nfl_analysis container already has `gh` installed.

## Existing Patterns

This design directly mirrors the Claude token injection pattern established in the `2026-02-04-claude-token-auto-provisioning` design:

- **Token resolution function:** `ensureClaudeToken()` in `internal/container/devcontainer.go` — reads token file, returns `(path, token)` or empty strings. New `ensureGitHubToken()` follows the same signature and behavior, minus auto-provisioning.
- **TemplateData field:** `ClaudeTokenPath` in `internal/container/compose.go` — populated in `buildTemplateData()`, falls back to `/dev/null`. New `GitHubTokenPath` follows the same pattern.
- **Compose mount:** `{{.ClaudeTokenPath}}:/run/secrets/claude-token:ro` in `docker-compose.yml.tmpl`. New mount at `/run/secrets/github-token:ro`.
- **Shell profile export:** `.bashrc`/`.zshrc` conditionally export `CLAUDE_CODE_OAUTH_TOKEN`. New block conditionally exports `GH_TOKEN`.

**Divergence from Claude pattern:** `ensureClaudeToken()` auto-provisions via `claude setup-token` when the token file is missing. `ensureGitHubToken()` does not auto-provision — it only reads from the user-managed file and logs a warning if missing. This is intentional: Claude tokens have a CLI-based provisioning path; GitHub tokens require manual creation in GitHub settings.

**Logger plumbing divergence:** `ensureClaudeToken()` has no logger access and fails silently. Adding a logger to `ComposeGenerator` for the GitHub warning is a small structural improvement that could also benefit Claude token handling in the future.

## Implementation Phases

<!-- START_PHASE_1 -->
### Phase 1: GitHub Token Resolution

**Goal:** Add `ensureGitHubToken()` function that reads the token file and returns the path, with warning support.

**Components:**
- `ensureGitHubToken()` in `internal/container/devcontainer.go` — reads `~/.config/github/token` (XDG-aware), returns `(tokenPath, token)` or `("", "")` if missing
- Logger plumbing: `ComposeGenerator` in `internal/container/compose.go` gains a `logger *logging.ScopedLogger` field, `NewComposeGenerator()` accepts a logger parameter
- `NewComposeGenerator()` call sites in `internal/container/manager.go` (lines 77, 115) updated to pass logger
- Warning logged via the plumbed logger when token file is missing
- Tests in `internal/container/compose_test.go` updated for new `NewComposeGenerator` signature

**Dependencies:** None

**Done when:** `ensureGitHubToken()` returns correct path when file exists, returns empty when missing, and warning is logged through the plumbed logger. All existing tests still pass with updated `NewComposeGenerator` signature.
<!-- END_PHASE_1 -->

<!-- START_PHASE_2 -->
### Phase 2: Template Data and Compose Mount

**Goal:** Wire the GitHub token path through template data into the docker-compose template.

**Components:**
- `GitHubTokenPath string` field added to `TemplateData` in `internal/container/compose.go`
- `buildTemplateData()` in `internal/container/compose.go` calls `ensureGitHubToken()`, falls back to `/dev/null`
- `docker-compose.yml.tmpl` in `config/templates/basic/` gains volume mount `{{.GitHubTokenPath}}:/run/secrets/github-token:ro`
- `TemplateData` usage in `internal/container/devcontainer.go` `generateFromTemplate()` also populates `GitHubTokenPath`

**Dependencies:** Phase 1

**Done when:** Generated `docker-compose.yml` includes the GitHub token mount. Template tests verify the mount appears in output.
<!-- END_PHASE_2 -->

<!-- START_PHASE_3 -->
### Phase 3: Shell Profile and Dockerfile Changes

**Goal:** Export `GH_TOKEN` in container shell profiles and install `gh` CLI in the basic template Dockerfile.

**Components:**
- `.bashrc` in `config/templates/basic/home/vscode/` — add `GH_TOKEN` export block after Claude token block
- `.zshrc` in `config/templates/basic/home/vscode/` — add `GH_TOKEN` export block after Claude token block
- `Dockerfile` in `config/templates/basic/` — install `gh` CLI via official apt repository (alongside existing tmux install)

**Dependencies:** Phase 2

**Done when:** `gh` CLI is installed in built containers, `GH_TOKEN` is exported when token file is present, and `gh auth status` succeeds inside a container with a valid token mounted.
<!-- END_PHASE_3 -->

## Additional Considerations

**Token security:** The token file is mounted read-only (`ro` flag) and lives at `/run/secrets/github-token`, consistent with the secrets convention used for the Claude token. The host file should have `0600` permissions (user responsibility).

**Proxy allowlist:** GitHub domains (`github.com`, `*.github.com`, `raw.githubusercontent.com`) are already in the mitmproxy allowlist. No changes needed for `gh` CLI network access. The `BLOCK_GITHUB_PR_MERGE` setting in `filter.py` continues to work independently.

**Existing containers:** Containers already running (like nfl_analysis) will need to be recreated to pick up the new mount and shell profile changes. The `gh` CLI is already installed in nfl_analysis but the token mount and `GH_TOKEN` export are new.

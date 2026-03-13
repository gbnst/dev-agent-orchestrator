# Compose Root Launch Design

## Summary

Devagent currently uses the `@devcontainers/cli` tool to bring up containers for each project and its git worktrees. That CLI requires a separate `docker-compose.yml` override file to be generated inside every worktree directory, which means port assignments and compose configuration are baked in at worktree-creation time. This design replaces that entire mechanism with direct `docker compose` invocations anchored to the project root — the single compose file at `<project>/.devcontainer/docker-compose.yml` is the only file ever used, for both the main project container and every worktree container derived from it.

Isolation between the resulting container groups is achieved through two complementary mechanisms. First, each group gets a unique compose project name (supplied via `docker compose -p`) so that Docker treats them as entirely independent deployments. Second, host ports are never hardcoded: the system parses the compose file to find port environment variables (`${APP_PORT:-8000}` style), allocates free ports at launch time, and passes them as environment variables when executing `docker compose up`. This eliminates port conflicts when multiple worktrees from the same project run simultaneously. The net result is a simpler, more predictable launch model with no generated override files, no dependency on the devcontainer CLI, and no per-worktree compose configuration to keep in sync.

## Definition of Done

1. **Compose launches from project root only** — `docker compose -p <name> -f <project>/.devcontainer/docker-compose.yml up -d`. The `.devcontainer/` inside worktrees is never used.

2. **Dynamic port allocation** — Parse project root's docker-compose.yml to discover port env vars (e.g., `${APP_PORT:-8000}`), find free ports on the host, and pass them when launching compose.

3. **Compose project naming** — Each worktree container group gets a unique compose project name via `-p <worktree_name>` flag, enabling multiple independent container groups from the same compose file.

4. **Raw docker compose** — Replace devcontainer CLI (`@devcontainers/cli`) usage with direct `docker compose` calls.

5. **Templates updated** — Existing templates updated to use env var ports and work with the new launch model. No worktree-specific compose overrides.

6. **Devcontainer CLI removed** — The `@devcontainers/cli` dependency and all related code paths are removed.

7. **Worktree .devcontainer cleanup** — Stop generating/using `docker-compose.worktree.yml` overrides in worktree directories. The `WriteComposeOverride()` pattern is removed.

**Out of scope:** VSCode/IDE integration for attaching to containers, worktree-specific volume overrides, changing how git worktrees are created.

## Acceptance Criteria

### compose-root-launch.AC1: Containers launch from project root
- **compose-root-launch.AC1.1 Success:** Container group starts via `docker compose -p <name> -f <project>/.devcontainer/docker-compose.yml up -d` from project root
- **compose-root-launch.AC1.2 Success:** Main project container uses sanitized directory name as compose project name
- **compose-root-launch.AC1.3 Success:** Worktree container uses worktree name as compose project name
- **compose-root-launch.AC1.4 Failure:** Launch fails gracefully when `.devcontainer/docker-compose.yml` doesn't exist in project root

### compose-root-launch.AC2: Dynamic port allocation
- **compose-root-launch.AC2.1 Success:** Port env vars are extracted from compose file port mappings (e.g., `${APP_PORT:-8000}:8000`)
- **compose-root-launch.AC2.2 Success:** Free host ports are allocated and passed as env vars when launching
- **compose-root-launch.AC2.3 Success:** Multiple container groups from the same project get distinct ports (no conflicts)
- **compose-root-launch.AC2.4 Edge:** Compose file with no env var ports (hardcoded ports) still launches correctly

### compose-root-launch.AC3: Compose project naming enables isolation
- **compose-root-launch.AC3.1 Success:** Two worktree containers from the same project run simultaneously with different compose project names
- **compose-root-launch.AC3.2 Success:** `docker compose down` with a compose project name only affects that container group
- **compose-root-launch.AC3.3 Success:** Container struct `ComposeProject` field is populated from Docker's `com.docker.compose.project` label

### compose-root-launch.AC4: Raw docker compose replaces devcontainer CLI
- **compose-root-launch.AC4.1 Success:** No `devcontainer` CLI invocations remain in the codebase

### compose-root-launch.AC5: Templates use env var ports
- **compose-root-launch.AC5.1 Success:** All template `docker-compose.yml.tmpl` files use `${VAR:-default}` syntax for host port mappings
- **compose-root-launch.AC5.2 Success:** No template directories contain `docker-compose.worktree.yml`

### compose-root-launch.AC6: Devcontainer CLI dependency removed
- **compose-root-launch.AC6.1 Success:** `DevcontainerCLI`, `DevcontainerGenerator`, and related code are deleted; `go build` succeeds

### compose-root-launch.AC7: Worktree compose overrides removed
- **compose-root-launch.AC7.1 Success:** `WriteComposeOverride()` no longer exists; worktree creation does not generate `docker-compose.worktree.yml`
- **compose-root-launch.AC7.2 Success:** Worktree destruction finds container by compose project name, not by path matching

## Glossary

- **Compose project name**: A label Docker Compose assigns to a group of containers started together. Set via the `-p` flag; stored on each container as the `com.docker.compose.project` label. Allows multiple independent deployments of the same compose file to coexist on one host.
- **devcontainer CLI (`@devcontainers/cli`)**: An official Node.js command-line tool that reads `devcontainer.json` and manages container lifecycle for development environments. This design removes it in favour of calling `docker compose` directly.
- **docker-compose.worktree.yml**: A generated compose override file that devagent currently writes into each worktree's `.devcontainer/` directory. This design eliminates these files.
- **`${VAR:-default}` syntax**: Docker Compose variable interpolation syntax. When the compose file contains `${APP_PORT:-8000}`, Compose substitutes the value of `APP_PORT` from the environment, falling back to `8000` if unset.
- **`net.Listen(":0")`**: A Go standard-library call that binds to port 0, causing the OS to assign an available ephemeral port. Used to discover free ports before passing them to `docker compose up`.
- **`RuntimeInterface`**: A Go interface in `internal/container/` that abstracts the container runtime (Docker or Podman), allowing compose operations without knowing which runtime is installed.
- **`ComposeGenerator`**: Devagent's internal template renderer that produces a `docker-compose.yml` from a template when bootstrapping a new project. Continues to exist but no longer bakes port values into rendered files.
- **`sanitizeComposeName()`**: A utility function that normalises a name into a valid Docker Compose project name (lowercase, non-alphanumeric replaced with hyphens). Moves from `internal/worktree/` to `internal/container/`.
- **git worktree**: A Git feature that checks out an additional working copy of a repository at a separate filesystem path, sharing the same underlying `.git` store.

## Architecture

Containers launch from the project root using raw `docker compose` instead of the devcontainer CLI. All container groups for a project (main and worktrees) share the same compose file at `<project>/.devcontainer/docker-compose.yml`. Isolation between container groups comes from two mechanisms: unique compose project names via `-p` flag, and dynamically allocated host ports passed as environment variables.

**Launch flow:**

1. Resolve project root (directory containing `.devcontainer/docker-compose.yml`)
2. Parse compose file to discover port env vars (e.g., `${APP_PORT:-8000}`) via regex on port mapping strings
3. Allocate free host ports for each discovered env var using `net.Listen(":0")`
4. Determine compose project name: sanitized directory name for main, worktree name for worktrees
5. Execute `docker compose -p <name> -f <project>/.devcontainer/docker-compose.yml up -d` with port env vars in process environment

**Container identity changes:**

Currently containers are identified by `ProjectPath` (the worktree directory). Since all containers now launch from the same project root, `ProjectPath` alone is insufficient. A new `ComposeProject` field on the `Container` struct serves as the primary identifier for compose operations and worktree-to-container association. A `Ports` field (`map[string]string`, env var name to allocated host port) tracks which ports each container group is using.

**What gets removed:**

- `DevcontainerCLI` — no more `devcontainer up` calls
- `DevcontainerGenerator` — no more devcontainer.json template processing
- `WriteComposeOverride()` — no more `docker-compose.worktree.yml` generation
- Worktree `.devcontainer/` usage — compose always reads from project root

**RuntimeInterface change:**

`ComposeUp` gains an `env map[string]string` parameter for passing port assignments. Other compose methods (`ComposeStart`, `ComposeStop`, `ComposeDown`) remain unchanged — Docker remembers the configuration after initial `up`.

## Existing Patterns

Investigation found the compose lifecycle methods (`ComposeUp`, `ComposeStart`, `ComposeStop`, `ComposeDown`) already use `-p` for project naming and `-f` for compose file paths. This design extends that pattern to include environment variable passing.

The `sanitizeComposeName()` function in `internal/worktree/compose.go` (lowercase, replace non-alphanumeric with hyphens) is reused for project name generation, but moves to `internal/container/` since naming is now the container layer's responsibility.

Container discovery via `runtime.ListContainers()` and label-based metadata extraction (`com.docker.compose.project`) continues unchanged. The `ComposeProject` field on `Container` is populated from the existing `com.docker.compose.project` Docker label.

Template rendering via `ComposeGenerator` continues for bootstrapping new projects but no longer bakes in port values — templates use `${VAR:-default}` syntax that Docker Compose interpolates at launch time.

**Divergence from existing patterns:**

- Two container creation paths (`CreateWithCompose` for new containers, `StartWorktreeContainer` for worktrees) merge into one unified path
- `DevcontainerCLI` and `DevcontainerGenerator` are removed entirely rather than adapted
- Container lookup by worktree shifts from path matching (`c.ProjectPath == wtDir`) to compose project name matching

## Implementation Phases

<!-- START_PHASE_1 -->
### Phase 1: Port Discovery and Allocation

**Goal:** Parse compose files to discover port env vars and allocate free host ports.

**Components:**
- Port parser in `internal/container/` — reads a compose file, extracts `${VAR:-default}` patterns from port mapping strings, returns map of var name to default value
- Port allocator in `internal/container/` — takes discovered port vars, finds free TCP ports on the host, returns map of var name to allocated port

**Dependencies:** None (new standalone functionality)

**Done when:** Port parser correctly extracts env vars from compose port mappings. Port allocator finds free ports that don't conflict. Covers `compose-root-launch.AC2.1` through `compose-root-launch.AC2.3`.
<!-- END_PHASE_1 -->

<!-- START_PHASE_2 -->
### Phase 2: Runtime and Manager Rework

**Goal:** Modify `ComposeUp` to accept env vars and unify container creation into a single path.

**Components:**
- `RuntimeInterface` in `internal/container/manager.go` — `ComposeUp` signature adds `env map[string]string` parameter
- `Runtime` in `internal/container/runtime.go` — implementation passes env vars to `docker compose` process
- `Manager` in `internal/container/manager.go` — `CreateWithCompose` and `StartWorktreeContainer` merge into unified creation method that uses port discovery, allocation, and compose project naming
- `Container` struct in `internal/container/types.go` — add `ComposeProject string` and `Ports map[string]string` fields
- `sanitizeComposeName()` moves from `internal/worktree/compose.go` to `internal/container/`

**Dependencies:** Phase 1 (port discovery and allocation)

**Done when:** Containers launch from project root with dynamic ports and `-p` naming. Container struct tracks compose project name and allocated ports. Covers `compose-root-launch.AC1.1` through `compose-root-launch.AC1.3`, `compose-root-launch.AC3.1` through `compose-root-launch.AC3.3`.
<!-- END_PHASE_2 -->

<!-- START_PHASE_3 -->
### Phase 3: Worktree Simplification

**Goal:** Remove compose override generation and update worktree-to-container association.

**Components:**
- `internal/worktree/compose.go` — remove `WriteComposeOverride()` and `sanitizeComposeName()` (moved in Phase 2)
- `internal/worktree/worktree.go` — `Create()` no longer calls `WriteComposeOverride()`
- `internal/worktree/destroy.go` — `DestroyWorktreeWithContainer()` finds container by compose project name instead of path matching

**Dependencies:** Phase 2 (unified creation path with compose project naming)

**Done when:** Worktree creation no longer generates `docker-compose.worktree.yml`. Worktree destruction correctly finds and removes containers by compose project name. Covers `compose-root-launch.AC7.1`, `compose-root-launch.AC7.2`.
<!-- END_PHASE_3 -->

<!-- START_PHASE_4 -->
### Phase 4: Remove Devcontainer CLI

**Goal:** Delete `DevcontainerCLI`, `DevcontainerGenerator`, and all code paths that reference them.

**Components:**
- `internal/container/devcontainer.go` — delete file (contains `DevcontainerGenerator` and `DevcontainerCLI`)
- `internal/container/manager.go` — remove `devCLI` and `generator` fields from `Manager`, remove any remaining `devcontainer up` call paths
- `go.mod` / `go.sum` — remove `@devcontainers/cli` related dependencies if any Go-side wrappers exist

**Dependencies:** Phase 2 (unified creation path no longer uses devcontainer CLI)

**Done when:** No references to devcontainer CLI remain in codebase. `go build` succeeds. Covers `compose-root-launch.AC4.1`, `compose-root-launch.AC6.1`.
<!-- END_PHASE_4 -->

<!-- START_PHASE_5 -->
### Phase 5: Template Updates

**Goal:** Update all templates to use env var port syntax and remove worktree override placeholders.

**Components:**
- `config/templates/basic/.devcontainer/docker-compose.yml.tmpl` — replace hardcoded ports with `${VAR:-default}` syntax
- `config/templates/go-project/.devcontainer/docker-compose.yml.tmpl` — same
- `config/templates/python-fullstack/.devcontainer/docker-compose.yml.tmpl` — same
- `config/templates/*/devcontainer.json.tmpl` — remove `docker-compose.worktree.yml` from `dockerComposeFile` array
- `config/templates/*/docker-compose.worktree.yml` — delete placeholder files
- `ComposeGenerator` in `internal/container/compose.go` — remove `ProxyPort` from `TemplateData` since ports are no longer baked in

**Dependencies:** Phase 2 (port discovery must work with new template format)

**Done when:** Templates render with `${VAR:-default}` port syntax. No worktree override placeholders exist. `ComposeGenerator` produces valid compose files. Covers `compose-root-launch.AC5.1`, `compose-root-launch.AC5.2`.
<!-- END_PHASE_5 -->

<!-- START_PHASE_6 -->
### Phase 6: TUI, Web, and CLI Integration

**Goal:** Update all UI and CLI code to use the unified container creation path.

**Components:**
- `internal/tui/update.go` — update `createWorktree`, `startWorktreeContainer`, `startMissingWorktreeContainer` to use unified creation path with compose project name
- `internal/web/api.go` — update `handleCreateContainer`, `handleStartWorktreeContainer`, `handleCreateWorktree` to use unified path; update container JSON serialization to include `ComposeProject` and `Ports`
- `internal/cli/worktree.go` — update CLI delegation to pass compose project name
- `internal/web/frontend/` — update TypeScript types and UI components if container data shape changes (new `composeProject` and `ports` fields)

**Dependencies:** Phase 2 (unified creation path), Phase 3 (worktree changes)

**Done when:** TUI, Web API, and CLI all launch containers via the unified path. Container details display allocated ports. End-to-end flow works for both main project and worktree containers. Covers `compose-root-launch.AC1.1` through `compose-root-launch.AC1.3` (integration-level verification).
<!-- END_PHASE_6 -->

## Additional Considerations

**Port conflicts:** If a previously allocated port becomes occupied between `up` and `start` (e.g., after a host reboot), `docker compose start` may fail. This is acceptable — the user can destroy and recreate the container group to get new ports. No automatic re-allocation on `start`.

**Podman compatibility:** The runtime already auto-detects Docker vs Podman. The `-p` flag and environment variable passing work identically with `podman-compose`. No Podman-specific changes needed.

**devcontainer.json retention:** Projects may still have a `devcontainer.json` for VSCode's "Reopen in Container" feature. Devagent no longer reads or generates this file, but its presence doesn't interfere with raw `docker compose` invocation.

# Human Test Plan: Compose Root Launch

## Prerequisites
- Docker Desktop (or Podman) running
- A project with `.devcontainer/docker-compose.yml` in `config/templates/basic/` format
- devagent built: `make frontend-build && make build`
- All unit tests passing: `make test` and `make frontend-test`

## Phase 1: Container Launch from Project Root

| Step | Action | Expected |
|------|--------|----------|
| 1 | Run `make dev` to start devagent | TUI starts, projects listed |
| 2 | Select a project with `.devcontainer/docker-compose.yml` | Project highlighted in tree |
| 3 | Press `s` to start the container | Loading spinner appears |
| 4 | Wait for container to start | Status changes to success |
| 5 | Run `docker ps --format '{{.Names}} {{.Labels}}'` | Container names follow `<project>-app-1`, `<project>-proxy-1` pattern |
| 6 | Run `docker compose ls` | Entry with compose project name matching sanitized directory name (lowercase, hyphens) |
| 7 | Check `config/orchestrator.log` for compose command | Log contains `-p <sanitized-name> -f <project>/.devcontainer/docker-compose.yml up -d` |
| 8 | Open web UI at `http://localhost:<port>` | Container visible with `compose_project` field in project view |

## Phase 2: Worktree Container Isolation

| Step | Action | Expected |
|------|--------|----------|
| 1 | From TUI, press `w` on a project, enter name `wt-alpha` | Worktree created, container starts |
| 2 | Press `w` again, enter name `wt-beta` | Second worktree created, container starts |
| 3 | Run `docker compose ls` | Two entries: `<project>-wt-alpha` and `<project>-wt-beta` |
| 4 | Run `docker ps` | Both groups running, no port conflicts |
| 5 | Run `docker compose -p <project>-wt-alpha down` | Only wt-alpha containers removed |
| 6 | Run `docker ps` | wt-beta containers still running |
| 7 | Run `docker compose ls` | Only `<project>-wt-beta` remains |

## Phase 3: Port Allocation

| Step | Action | Expected |
|------|--------|----------|
| 1 | Examine `docker-compose.yml` of a template using `${APP_PORT:-8000}:8000` | Port env var pattern present |
| 2 | Start container for a project using that template | Container starts without port conflicts |
| 3 | Run `docker ps` and check port mappings | Host ports are dynamically allocated (different from default) |
| 4 | Start a second worktree container for the same project | Both containers running with distinct host ports |

## Phase 4: Template Verification

| Step | Action | Expected |
|------|--------|----------|
| 1 | Run `grep -r '{{.ProxyPort}}' config/templates/` | Zero matches |
| 2 | Run `grep -r '8080' config/templates/*/.devcontainer/docker-compose.yml.tmpl` | Matches in all templates (hardcoded proxy port) |
| 3 | Run `find config/templates -name 'docker-compose.worktree.yml'` | Zero results |
| 4 | Open each `devcontainer.json.tmpl` | `dockerComposeFile` contains only `["docker-compose.yml"]` |

## Phase 5: Devcontainer CLI Removal Verification

| Step | Action | Expected |
|------|--------|----------|
| 1 | Run `grep -r 'devcontainer up' internal/` | Zero matches |
| 2 | Run `grep -r 'DevcontainerCLI' internal/` | Zero matches |
| 3 | Run `grep -r 'DevcontainerGenerator' internal/` | Zero matches |
| 4 | Run `grep -r 'WriteComposeOverride' internal/` | Zero matches |
| 5 | Run `go build ./...` | Clean build |

## End-to-End: Full Worktree Lifecycle

| Step | Action | Expected |
|------|--------|----------|
| 1 | Start devagent: `make dev` | TUI launches |
| 2 | Select a project, press `w`, enter name `e2e-test` | Worktree created, container starts |
| 3 | Run `docker compose ls` | Entry `<project>-e2e-test` visible |
| 4 | In TUI, select worktree container, check detail panel | ComposeProject field shows `<project>-e2e-test` |
| 5 | Open web UI, navigate to the project | Worktree visible with `compose_project` in JSON |
| 6 | In TUI, press `W` on worktree to delete | Confirmation dialog appears |
| 7 | Confirm deletion | Container stopped, destroyed, worktree removed |
| 8 | Run `docker compose ls` | `<project>-e2e-test` no longer listed |
| 9 | Run `docker ps -a` | No containers with that compose project |

## Traceability

| AC | Automated Test | Manual Step |
|----|----------------|-------------|
| AC1.1 | `TestComposeUp_Docker`, `TestCreateWithCompose_CallsComposeUpWithCorrectArgs` | Phase 1 steps 2-7 |
| AC1.2 | `TestSanitizeComposeName`, `TestCreateWithCompose_SanitizesProjectName` | Phase 1 step 6 |
| AC1.3 | `TestCreateWithCompose_WorktreeComposeProjNaming`, api_test worktree handlers | Phase 2 step 3 |
| AC1.4 | `TestCreateWithCompose_FailsWhenComposeFileMissing` | -- |
| AC2.1 | `TestParsePortEnvVarsFromContent`, `TestParsePortEnvVars` | Phase 3 step 1 |
| AC2.2 | `TestAllocateFreePorts`, `TestComposeUp_WithEnv_Docker`, `TestExecWithEnv_WithNonEmptyMap` | Phase 3 steps 2-3 |
| AC2.3 | `TestAllocateFreePorts/all_allocated_ports_are_distinct` | Phase 3 step 4 |
| AC2.4 | `TestParsePortEnvVarsFromContent/no_variables`, `TestComposeUp_WithEnv_FallsBackToExecForNilEnv` | -- |
| AC3.1 | -- | Phase 2 steps 1-4 |
| AC3.2 | -- | Phase 2 steps 5-7 |
| AC3.3 | `TestParseContainerList_SingleContainer`, `TestCreateWithCompose_SetsComposeProjectField` | Phase 1 step 8 |
| AC4.1 | Build verification | Phase 5 steps 1-5 |
| AC5.1 | `TestComposeGenerator_TemplateRenderingNoProxyPort` | Phase 4 steps 1-2 |
| AC5.2 | `TestComposeGenerator_TemplateRenderingNoProxyPort` | Phase 4 steps 3-4 |
| AC6.1 | `TestComposeGenerator_WriteToProject_*` | Phase 5 step 5 |
| AC7.1 | Build verification | Phase 5 step 4 |
| AC7.2 | `TestDestroyWorktreeWithContainer_*` | E2E steps 6-9 |

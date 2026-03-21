# VS Code Attach Design

## Summary

This design adds VS Code remote container attachment to the devagent TUI. When a user selects a running container in the tree view and presses `v`, devagent constructs a `vscode-remote://` URI that encodes the container's identity as a hex-encoded JSON payload and invokes the `code` CLI to open VS Code attached to that container's filesystem and environment. The feature also fixes a pre-existing bug in the action menu's "Open in VS Code" command, which was generating an incorrect URI scheme (`dev-container` instead of `attached-container`) and using a truncated container ID.

The implementation is structured in three layers following patterns already established in the codebase. First, the container runtime is updated to retrieve full 64-character container IDs (Docker truncates them by default). Second, the pure `GenerateVSCodeCommand()` function is corrected to accept a container ID directly and produce a properly encoded URI. Third, a new `v` key handler is wired into the Bubbletea update loop using the same guard and async command patterns used by existing container lifecycle operations — the `code` process is launched in a goroutine via `tea.Cmd`, and the result is returned as a message that updates the status bar.

## Definition of Done
When a running container is selected in the TUI tree and the user presses `v`, VS Code launches and attaches to the app container using the correct `attached-container` URI scheme. The `v` key executes `code --folder-uri "vscode-remote://attached-container+<hex-json>/<workspace-folder>"` where the hex payload encodes `{"containerName":"<full-app-container-id>"}`. Uses the full (not truncated) Docker container ID, targets the app container (not proxy sidecar), reads workspace folder from devcontainer.json, only works on running containers (no-op otherwise), shows status bar feedback, and also fixes `GenerateVSCodeCommand()` so the action menu shows the corrected command.

## Acceptance Criteria

### vscode-attach.AC1: Container IDs are full-length
- **vscode-attach.AC1.1 Success:** `Container.ID` contains the full 64-character hex hash after refresh

### vscode-attach.AC2: VS Code command generates correct URI
- **vscode-attach.AC2.1 Success:** URI uses `attached-container+` scheme (not `dev-container+`)
- **vscode-attach.AC2.2 Success:** Hex payload decodes to `{"containerName":"<full-64-char-id>"}`
- **vscode-attach.AC2.3 Success:** Workspace path from `ReadWorkspaceFolder()` appended after hex payload

### vscode-attach.AC3: v key launches VS Code on running containers
- **vscode-attach.AC3.1 Success:** Pressing `v` on a running container executes `code --folder-uri` with correct URI
- **vscode-attach.AC3.2 Success:** Status bar shows success message after launch
- **vscode-attach.AC3.3 Failure:** Pressing `v` on a stopped container is a no-op
- **vscode-attach.AC3.4 Failure:** Pressing `v` with no container selected is a no-op

## Glossary

- **Bubbletea**: A Go framework for building terminal UIs using the Elm architecture (Model-Update-View). All UI state mutations happen through `Update()`, and side-effectful work runs in `tea.Cmd` closures.
- **tea.Cmd**: A function type in Bubbletea that runs asynchronously in a goroutine and returns a message back to the `Update()` loop.
- **attached-container URI scheme**: The VS Code Remote Containers URI format (`vscode-remote://attached-container+<hex>/<path>`) used to attach to an already-running Docker container, as opposed to `dev-container` which opens a devcontainer definition.
- **hex payload**: The JSON object `{"containerName":"<id>"}` encoded as hexadecimal and embedded in the VS Code URI.
- **`--no-trunc`**: A Docker/Podman CLI flag for `docker ps` that returns full 64-character container IDs instead of truncated 12-character ones.
- **devcontainer.json**: A configuration file specifying how VS Code should configure a development container, including the workspace folder path.
- **Functional Core / Imperative Shell**: An architectural pattern where pure functions compute values without side effects, and impure shell code performs I/O.
- **sidecar / proxy container**: A companion container (mitmproxy) that runs alongside each app container for network isolation. The `v` key targets the app container, not this sidecar.

## Architecture

Press `v` on a running container in the TUI to launch VS Code attached to that container. The feature has three layers:

**Data layer:** `Runtime.ListContainers()` in `internal/container/runtime.go` adds `--no-trunc` to the `docker ps` call so `Container.ID` always contains the full 64-character hash. No struct changes — consumers transparently get longer IDs.

**Pure function:** `GenerateVSCodeCommand()` in `internal/tui/actions.go` changes signature from `(projectPath, workspacePath)` to `(containerID, workspacePath)`. It builds the JSON payload `{"containerName":"<containerID>"}`, hex-encodes it, and returns `code --folder-uri "vscode-remote://attached-container+<hex>/<workspacePath>"`. This fixes both the `v` key handler and the action menu's "Open in VS Code" command.

**TUI handler:** New `"v"` key case in `internal/tui/update.go` guarded by `m.selectedContainer != nil && m.selectedContainer.State == container.StateRunning` (same pattern as the `"t"` key). Dispatches a `tea.Cmd` that runs `os/exec.Command("code", "--folder-uri", uri)`. A new `vscodeLaunchMsg` message type carries the result back to `Update()`, which sets status bar feedback via `setSuccess()` or `setError()`.

## Existing Patterns

This design follows established patterns in the TUI codebase:

- **Key guard pattern:** The `"t"` key at `update.go:460-466` checks `selectedContainer != nil && State == StateRunning` before opening the action menu. The `"v"` key uses the identical guard.
- **Async command pattern:** Container lifecycle operations (`startContainer`, `stopContainer`, `destroyContainer` at `update.go:818-848`) return `tea.Cmd` functions that perform work in goroutines and send result messages back to `Update()`. The VS Code launch follows this pattern with `vscodeLaunchMsg`.
- **Status feedback pattern:** `containerActionMsg` handling at `update.go:599-617` uses `setSuccess()`/`setError()` for user feedback. The `vscodeLaunchMsg` handler follows the same pattern.
- **Pure action generation:** `GenerateContainerActions()` and `GenerateVSCodeCommand()` in `actions.go` are pure functions (Functional Core pattern). The signature change preserves this — no side effects in the function.

No new patterns are introduced. The only novel element is calling `os/exec.Command` from a `tea.Cmd`, which is a standard Go pattern not yet used in the TUI package but consistent with the Bubbletea concurrency model (Cmd closures run in goroutines).

## Implementation Phases

<!-- START_PHASE_1 -->
### Phase 1: Full Container IDs
**Goal:** Ensure `Container.ID` always contains the full 64-character Docker/Podman container ID.

**Components:**
- `Runtime.ListContainers()` in `internal/container/runtime.go` — add `--no-trunc` flag to `docker ps` args
- Existing tests in `internal/container/runtime_test.go` — verify parsing still works with full-length IDs

**Dependencies:** None

**Done when:** `Container.ID` contains full 64-char hashes after `Manager.Refresh()`. Existing tests pass. Covers `vscode-attach.AC1.1`.
<!-- END_PHASE_1 -->

<!-- START_PHASE_2 -->
### Phase 2: Fix VS Code Command Generation
**Goal:** Generate correct `attached-container` URI instead of broken `dev-container` URI.

**Components:**
- `GenerateVSCodeCommand()` in `internal/tui/actions.go` — new signature `(containerID, workspacePath string)`, builds `{"containerName":"<id>"}` JSON, hex-encodes, returns `attached-container+` URI
- `GenerateContainerActions()` in `internal/tui/actions.go` — pass `c.ID` instead of `c.ProjectPath`
- Tests in `internal/tui/actions_test.go` — verify correct URI format, hex encoding, workspace path

**Dependencies:** Phase 1 (full IDs available)

**Done when:** `GenerateVSCodeCommand()` returns `code --folder-uri "vscode-remote://attached-container+<hex-json>/<workspace>"` with correct hex-encoded JSON payload. Tests verify encoding correctness. Covers `vscode-attach.AC2.1`, `vscode-attach.AC2.2`, `vscode-attach.AC2.3`.
<!-- END_PHASE_2 -->

<!-- START_PHASE_3 -->
### Phase 3: VS Code Launch from TUI
**Goal:** `v` key launches VS Code attached to the selected running container.

**Components:**
- New `vscodeLaunchMsg` message type in `internal/tui/update.go`
- New `"v"` key case in `Update()` key switch in `internal/tui/update.go` — same guard as `"t"`, dispatches `tea.Cmd` that runs `exec.Command("code", "--folder-uri", uri)`
- `vscodeLaunchMsg` handler in `Update()` — calls `setSuccess()` or `setError()`
- Tests in `internal/tui/update_test.go` — verify no-op on stopped/nil containers, verify command dispatch on running containers

**Dependencies:** Phase 2 (correct URI generation)

**Done when:** Pressing `v` on a running container launches VS Code. No-op on stopped containers or when no container is selected. Status bar shows feedback. Covers `vscode-attach.AC3.1`, `vscode-attach.AC3.2`, `vscode-attach.AC3.3`, `vscode-attach.AC3.4`.
<!-- END_PHASE_3 -->

## Additional Considerations

**Error cases:** If `code` is not on PATH, `exec.Command` will return an error which surfaces via `setError()` in the status bar. No special handling needed beyond the standard error path.

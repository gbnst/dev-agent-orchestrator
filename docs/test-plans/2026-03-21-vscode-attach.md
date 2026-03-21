# Human Test Plan: VS Code Attach

## Prerequisites
- Development environment with Docker or Podman running
- At least one devagent-managed container running (`devagent` with a project configured)
- VS Code installed and `code` binary on PATH (for success path testing)
- Build the binary: `make frontend-build && make build`
- All unit tests passing: `make test`

## Phase 1: Full Container ID Verification

| Step | Action | Expected |
|------|--------|----------|
| 1.1 | Run `devagent` (or `make dev`) with at least one running container | TUI launches and shows container(s) in tree |
| 1.2 | Select a running container in the tree, then press `t` to open the action menu | Action menu appears with "Open in VS Code" and other options |
| 1.3 | Inspect the VS Code command shown in the action menu. Find the hex-encoded portion after `attached-container+` | The hex portion should decode (e.g., via `echo '<hex>' \| xxd -r -p`) to JSON like `{"containerName":"<64-char-id>"}` where the ID is exactly 64 hex characters |
| 1.4 | Alternatively, run `docker ps -a --no-trunc --filter label=devagent.managed=true --format json` and compare the ID field | The ID in the JSON output should be a full 64-character hex string matching what devagent displays |

## Phase 2: VS Code Launch (Success Path)

| Step | Action | Expected |
|------|--------|----------|
| 2.1 | Ensure `code` is on your PATH (`which code` returns a path) | Path to `code` binary is printed |
| 2.2 | Run `devagent`, select a running container in the tree | Container is highlighted, detail panel shows "running" state |
| 2.3 | Press `v` | VS Code opens a new window attached to the selected container. The status bar in devagent shows a green success message containing "VS Code" |
| 2.4 | In the VS Code window, verify the remote indicator (bottom-left corner) shows "Container <name>" | VS Code is connected to the correct container |
| 2.5 | In the VS Code terminal, run `pwd` | Output should be the workspace folder (e.g., `/workspaces`) |

## Phase 3: VS Code Launch (Error Path)

| Step | Action | Expected |
|------|--------|----------|
| 3.1 | Temporarily rename or remove `code` from PATH (e.g., `export PATH=$(echo $PATH \| tr ':' '\n' \| grep -v 'Visual Studio' \| tr '\n' ':')`) | `which code` returns nothing |
| 3.2 | Run `devagent`, select a running container, press `v` | The status bar shows a red error message indicating the `code` command could not be found |
| 3.3 | Restore PATH to its original value | `which code` works again |

## Phase 4: No-Op Edge Cases

| Step | Action | Expected |
|------|--------|----------|
| 4.1 | Run `devagent`, select a stopped container (shown with stopped indicator) | Container is highlighted |
| 4.2 | Press `v` | Nothing happens. No status bar message, no VS Code launch. The TUI remains unchanged |
| 4.3 | Navigate to a project node (not a container) so no container is selected | Project or "All Projects" row is highlighted |
| 4.4 | Press `v` | Nothing happens. No status bar message, no error |

## End-to-End: Full Lifecycle

1. Start `devagent` with a configured project that has a template
2. Press `c` to create a new container, fill in the form, submit
3. Wait for the container to reach "running" state
4. Select the newly created container and press `v`
5. Verify VS Code opens and attaches to the container with the correct workspace
6. Close VS Code, return to devagent
7. Press `x` to stop the container, wait for it to reach "stopped" state
8. Press `v` on the stopped container -- verify it is a no-op
9. Press `s` to restart the container, wait for "running" state
10. Press `v` again -- verify VS Code opens again successfully
11. Clean up: press `d` to destroy the container, confirm

## Traceability

| Acceptance Criterion | Automated Test | Manual Step |
|----------------------|----------------|-------------|
| vscode-attach.AC1.1 -- Full container IDs | `TestListContainers_CallsCorrectCommand` (--no-trunc flag) | Phase 1, steps 1.3-1.4 |
| vscode-attach.AC2.1 -- attached-container+ scheme | `TestGenerateVSCodeCommand`, `TestGenerateContainerActions_ContainsExpectedActions` | Phase 2, step 2.4 |
| vscode-attach.AC2.2 -- Hex payload decodes correctly | `TestGenerateVSCodeCommand` (hex decode + JSON unmarshal) | Phase 1, step 1.3 |
| vscode-attach.AC2.3 -- Workspace path appended | `TestGenerateVSCodeCommand` (HasSuffix check) | Phase 2, step 2.5 |
| vscode-attach.AC3.1 -- v key dispatches command | `TestVKey_RunningContainer_DispatchesCommand` | Phase 2, steps 2.2-2.3 |
| vscode-attach.AC3.2 -- Status bar messages | `TestVSCodeLaunchMsg_Success`, `TestVSCodeLaunchMsg_Error` | Phase 2 step 2.3, Phase 3 step 3.2 |
| vscode-attach.AC3.3 -- Stopped container no-op | `TestVKey_StoppedContainer_NoOp` | Phase 4, steps 4.1-4.2 |
| vscode-attach.AC3.4 -- No selection no-op | `TestVKey_NoContainerSelected_NoOp` | Phase 4, steps 4.3-4.4 |

# Tmux Domain

Last verified: 2026-02-26

## Purpose
Wraps tmux commands executed inside containers via a ContainerExecutor function. Provides session listing, creation, destruction, pane capture with offset support, and cursor position queries.

## Contracts
- **Exposes**: `Client`, `Session`, `CaptureOpts`, `ContainerExecutor` type, `ParseListSessions(containerID, output string) []Session` function
- **Guarantees**: ListSessions returns empty slice (not error) when no tmux server. ParseListSessions and Client.ListSessions handle malformed output gracefully. ParseListSessions can be used to parse tmux list-sessions output from any source (containers or host). Session.ContainerID is populated with the containerID parameter passed to ParseListSessions. CapturePane accepts CaptureOpts: Lines limits output to last N lines (trimmed in Go after capture); FromCursor captures from an absolute position by computing scrollback offset (set to -1 to disable). CaptureLines captures last N lines from scrollback history using `tmux capture-pane -S -N -p` (distinct from CapturePane which captures visible pane). CursorPosition returns absolute position (history_size + cursor_y) via `tmux display-message`, ensuring monotonic increase as output scrolls past the visible pane.
- **Expects**: ContainerExecutor that can run commands inside containers. Tmux installed in target containers.

## Dependencies
- **Uses**: logging.Manager (optional via LoggerProvider interface)
- **Used by**: container.Manager (for session operations), TUI indirectly
- **Boundary**: Tmux CLI wrapper only; no container lifecycle awareness

## Key Decisions
- Function-based executor: Decouples from specific container runtime
- Silent failure for no server: No tmux running is normal state, not error
- LoggerProvider interface: Avoids tight coupling to logging.Manager

## Invariants
- Client methods are nil-safe for logger (NopLogger default)
- Session.ContainerID always set by parsing functions
- Empty session name from parse means invalid line (skipped)

## Key Files
- `client.go` - Client struct, all tmux operations
- `parse.go` - Consolidated ParseListSessions parser (used by Client.ListSessions and web/host.go)
- `parse_test.go` - Tests for ParseListSessions covering edge cases and real-world tmux output
- `types.go` - Session type with helper methods

## Gotchas
- Session.AttachCommand(runtime, user) needs runtime name (docker/podman) and user (typically "vscode") from caller
- tmux list-sessions format varies slightly; parsing is lenient
- SendKeys auto-appends Enter; don't include in keys string

# Tmux Domain

Last verified: 2026-02-01

## Purpose
Wraps tmux commands executed inside containers via a ContainerExecutor function. Provides session listing, creation, destruction, and pane capture.

## Contracts
- **Exposes**: `Client`, `Session`, `Pane`, `ContainerExecutor` type
- **Guarantees**: ListSessions returns empty slice (not error) when no tmux server. Parsing handles malformed output gracefully.
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
- `types.go` - Session and Pane types with helper methods

## Gotchas
- Session.AttachCommand(runtime, user) needs runtime name (docker/podman) and user (typically "vscode") from caller
- tmux list-sessions format varies slightly; parsing is lenient
- SendKeys auto-appends Enter; don't include in keys string

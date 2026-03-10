# CLI Domain

Last verified: 2026-02-25

## Purpose
Command-line interface dispatch and delegation. Provides structured CLI commands that delegate to a running devagent TUI instance via HTTP. Includes session tailing with cursor-based polling and ANSI stripping.

## Contracts
- **Exposes**: `App`, `NewApp()`, `BuildApp()`, `Command`, `Group`, `Delegate`, `TailSession()`, `TailConfig`, `StripANSI()`, `PrintJSON()`, `ResolveDataDir()`
- **Guarantees**: `App.Execute()` returns true if TUI should be launched (no args), false otherwise. All delegated commands discover a running instance via `instance.Discover` and delegate via HTTP. Exit code 2 for "no running instance", exit code 1 for other errors, exit code 0 for success. `TailSession` polls via cursor-based capture, handles session-ended (404) and container-stopped (400) gracefully, retries connection failures once. `PrintJSON` pretty-prints with indentation when stdout is a terminal, raw bytes otherwise.
- **Expects**: Running devagent TUI instance for all delegated commands (container, session, worktree groups and list command). `instance.Discover` must be able to find the running instance via lock/port files.

## Dependencies
- **Uses**: instance.Discover, instance.Client, instance.Lock, instance.Cleanup
- **Used by**: main.go (BuildApp called in main, Execute dispatches or falls through to TUI)
- **Boundary**: CLI dispatch only; no container, TUI, or web server knowledge. All operations delegate to running instance via HTTP.

## Key Decisions
- Delegate pattern: `Delegate` struct encapsulates instance discovery, client creation, error classification, and exit code handling; `Run()` for fire-and-forget commands, `Client()` for commands needing ongoing client access (e.g., tail)
- Command groups: worktree, container, session -- each group requires a running instance
- Worktree create uses 120s client timeout (devcontainer builds can be slow)
- Tail uses cursor-based polling: initial capture gets last 10 lines, then polls with `from_cursor` parameter to get only new output; detects cursor resets (clear command) and does full recapture
- ANSI stripping via regex (CSI, OSC, and simple escape sequences)
- ExitFunc and Stderr are injectable on Delegate for testability

## Invariants
- Commands validate required arguments and return an error if missing (e.g., container start requires container ID). Delegate.Run() handles error reporting and exit codes.
- Tail exits cleanly (nil error) on context cancellation, session destroyed (404), or container stopped (400)
- Tail retries connection failures exactly once before returning error

## Key Files
- `app.go` - App, Command, Group types; Execute dispatch; help generation
- `commands.go` - BuildApp wiring, ResolveDataDir, list/cleanup/version commands
- `delegate.go` - Delegate struct with Run/Client methods, PrintJSON helper
- `container.go` - Container start/stop/destroy commands
- `worktree.go` - Worktree create command (with --no-start flag)
- `session.go` - Session create/destroy/readlines/send/tail commands
- `tail.go` - TailSession polling loop with cursor tracking and error recovery
- `ansi.go` - StripANSI utility (Functional Core)

# Process Domain

Last verified: 2026-02-22

## Purpose
Generic child process supervisor with configurable restart policies and graceful shutdown. Manages lifecycle of long-running external binaries (start, monitor, restart, stop).

## Contracts
- **Exposes**: `Supervisor`, `NewSupervisor()`, `Config`, `RestartPolicy` (Never, OnFailure, Always)
- **Guarantees**: Start() is non-blocking (launches goroutine). Stop() sends SIGTERM, waits up to 5s, then SIGKILL. Done() channel closes when supervisor exits. Double Start() returns error. Stdout/stderr are captured into ScopedLogger. Restart respects MaxRetries and RetryDelay.
- **Expects**: Valid binary path in Config. ScopedLogger for output capture.

## Dependencies
- **Uses**: logging.ScopedLogger, os/exec, syscall
- **Used by**: tsnsrv (for tsnsrv binary), main.go
- **Boundary**: Process lifecycle only; no knowledge of what binary it supervises

## Key Decisions
- SIGTERM then SIGKILL pattern with 5s grace period (not configurable)
- Stdout/stderr captured line-by-line via bufio.Scanner into structured logger
- RetryDelay defaults to 1s if unset in Config

## Invariants
- Running() reflects actual goroutine state (mutex-protected)
- Done() channel closed exactly once when supervisor loop exits
- cmd field nil when no process is active
- Context cancellation stops the process and exits the supervisor loop

## Key Files
- `process.go` - Supervisor struct, Config, RestartPolicy, lifecycle management

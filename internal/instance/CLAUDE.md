# Instance Domain

Last verified: 2026-02-25

## Purpose
Single-instance enforcement and CLI-to-TUI IPC. Ensures only one devagent TUI runs at a time via file locking, writes the web server address to a port file for discovery, and provides an HTTP client for CLI commands to delegate to the running instance.

## Contracts
- **Exposes**: `Lock()`, `WritePort()`, `Cleanup()`, `Discover()`, `NewClient()`, `NewClientWithTimeout()`, `Client` methods: `List()`, `StartContainer()`, `StopContainer()`, `DestroyContainer()`, `CreateSession()`, `DestroySession()`, `CreateWorktree()`, `ReadSession()`, `ReadLines()`, `ReadSessionFromCursor()`, `SendToSession()`
- **Guarantees**: Lock() uses exclusive file lock (gofrs/flock TryLock) -- non-blocking, returns immediately. Discover() verifies instance is running via lock check + port file read + /api/health probe. Cleanup() removes port file and releases lock (safe to call even if files are missing). All Client methods return ([]byte, error) and delegate to running TUI instance via HTTP; connection failures return "failed to connect to devagent: %w"; non-2xx responses extract error message from JSON `{"error": "..."}` field if present, else use raw body.
- **Expects**: dataDir exists and is writable. Running instance has a web server with /api/health, /api/projects, and container/session/worktree lifecycle endpoints.

## Dependencies
- **Uses**: gofrs/flock, net/http
- **Used by**: main.go (Lock/WritePort/Cleanup for TUI), cli package (Discover/Client for CLI delegation)
- **Boundary**: Lock and discovery only; no knowledge of container or TUI internals

## Key Decisions
- File-based locking (not PID files) for crash safety -- OS releases flock on process death
- Health check timeout is 2s; Client default timeout is 10s; NewClientWithTimeout() allows custom timeout for long-running operations (e.g. worktree creation)
- Port file stores raw "host:port" address (e.g. "127.0.0.1:12345")
- CLI commands (list, cleanup, container/session/worktree lifecycle) never start a Manager -- they delegate to the running instance
- HTTP helpers (post, delete, postJSON) are private; public typed methods compose them with correct API paths
- Project paths in URLs are base64-URL-encoded to avoid path separator issues

## Invariants
- Lock file: `{dataDir}/devagent.lock`
- Port file: `{dataDir}/devagent.port`
- Cleanup must be called (via defer) when the TUI exits to release lock and remove port file
- Discover fails fast if lock is not held (no instance running) before reading port file

## Key Files
- `lock.go` - Lock(), WritePort(), Cleanup()
- `discover.go` - Discover() with lock check + port read + health probe
- `client.go` - HTTP Client for delegating CLI commands to running instance

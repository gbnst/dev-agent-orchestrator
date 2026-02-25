# Web Domain

Last verified: 2026-02-25

## Purpose
HTTP/WebSocket server providing a REST API and embedded React SPA for managing containers and terminal sessions from a browser.

## Contracts
- **Exposes**: `Server`, `New()`, `Config`, `ContainerResponse`, `SessionResponse`, `CreateSessionRequest`, `ProjectResponse`, `WorktreeResponse`, `ProjectsListResponse`, `CreateWorktreeRequest`, `ResizeMessage`
- **Guarantees**: API responses are JSON. All mutations (session, container lifecycle, worktree) notify TUI via `p.Send(WebSessionActionMsg{})`. Frontend SPA is embedded via `//go:embed` and served with SPA fallback (unknown paths serve index.html). WebSocket terminal bridges PTY I/O to tmux sessions with resize support. Server disabled by default (port 0). Manager state changes push SSE "refresh" events to all connected browsers via `eventBroker`; frontend auto-refetches on each event. Host tmux sessions are managed directly via `os/exec` (no container runtime needed); host mutations use sentinel container ID `__host__` for TUI notifications. Container lifecycle endpoints (start/stop/destroy) delegate to Manager's compose operations. Worktree create auto-starts a container; worktree start creates a container for an existing containerless worktree (409 if container exists); worktree delete performs compound stop+destroy+remove.
- **Expects**: Valid `container.Manager`, `logging.LoggerProvider`, `func(tea.Msg)` for TUI notifications, and optional `func(context.Context) []discovery.DiscoveredProject` scanner for project discovery. Frontend must be built before Go binary (`make frontend-build`). Host session endpoints require `tmux` installed on the host (gracefully degrade to empty list if tmux is unavailable). If scanner is nil, `/api/projects` returns only unmatched containers.

## API Routes
- `GET /api/health` - Health check
- `GET /api/projects` - List projects with worktrees and matched containers; unmatched containers in separate list
- `GET /api/containers` - List all containers with sessions
- `GET /api/containers/{id}` - Get single container with sessions
- `GET /api/containers/{id}/sessions` - List sessions for container
- `POST /api/containers/{id}/sessions` - Create tmux session (body: `{"name": "..."}`)
- `DELETE /api/containers/{id}/sessions/{name}` - Destroy tmux session
- `GET /api/containers/{id}/sessions/{name}/terminal` - WebSocket terminal bridge
- `POST /api/containers/{id}/start` - Start stopped container (400 if already running)
- `POST /api/containers/{id}/stop` - Stop running container (400 if already stopped)
- `DELETE /api/containers/{id}` - Destroy container via compose down
- `POST /api/projects/{encodedPath}/worktrees` - Create worktree + auto-start container (body: `{"name": "..."}`)
- `POST /api/projects/{encodedPath}/worktrees/{name}/start` - Start container for containerless worktree via devcontainer up (409 if container exists)
- `DELETE /api/projects/{encodedPath}/worktrees/{name}` - Compound: stop container + destroy + git worktree remove
- `GET /api/host/sessions` - List host tmux sessions (returns empty array if tmux unavailable)
- `POST /api/host/sessions` - Create host tmux session (body: `{"name": "..."}`)
- `DELETE /api/host/sessions/{name}` - Destroy host tmux session
- `GET /api/host/sessions/{name}/terminal` - WebSocket terminal bridge for host tmux session
- `GET /api/events` - SSE stream; sends `event: connected` on open, `event: refresh` when container/session state changes
- `GET /` (and fallback) - Embedded SPA

## Dependencies
- **Uses**: container.Manager, logging.LoggerProvider, events.WebSessionActionMsg, discovery.DiscoveredProject, worktree (via worktreeOps interface and DestroyWorktreeWithContainer function), tmux.ParseListSessions, coder/websocket, creack/pty, os/exec (host tmux)
- **Used by**: main.go only
- **Boundary**: HTTP layer; delegates container business logic to container/tmux packages; worktree operations abstracted behind `worktreeOps` interface for testability and delegated to shared worktree.DestroyWorktreeWithContainer function; host tmux operations call `tmux` CLI directly via `os/exec`

## Key Decisions
- Listen/Serve split: Allows tests to obtain ephemeral port before blocking
- PTY bridge (container): Uses `docker exec -it` with tmux attach, matching Session.AttachCommand() flags
- PTY bridge (host): Uses `tmux -u attach-session` directly on host with TERM/COLORTERM env vars
- Binary frames for terminal data, text frames for control messages (resize)
- SPA fallback: All non-file paths serve index.html for client-side routing
- Frontend embedded at build time via `//go:embed frontend/dist`
- SSE push via Manager.SetOnChange: Server registers `eventBroker.Notify` as the Manager's onChange callback; eventBroker fans out to all SSE subscribers; frontend `useServerEvents` hook auto-refetches on each event
- Smart actions: Pluggable detector system scans terminal buffer text for patterns and shows floating overlay with one-click actions; detectors registered in `frontend/src/lib/detectors/index.ts`; `typeAndSubmit()` helper delays Enter keystroke to avoid Claude Code autocomplete interception
- worktreeOps interface: Abstracts worktree package functions (ValidateName, Create, Destroy, WorktreeDir) so handlers are unit-testable without git; `realWorktreeOps` delegates to worktree package; tests inject mocks via `SetWorktreeOpsForTest`
- Project path encoding: Project paths in URLs are base64-URL-encoded to avoid path separator issues; `decodeProjectPath` helper decodes them in handlers
- Project-container matching: `buildProjectResponses` indexes containers by ProjectPath for O(1) lookup, matches to worktrees, collects unmatched containers separately

## Invariants
- Server only starts when `config.Web.Port > 0`
- Session mutations always send `events.WebSessionActionMsg` to keep TUI in sync (host sessions use `ContainerID: "__host__"`)
- WebSocket uses `context.Background()` (not request context) after upgrade
- PTY read limit: 1 MB per WebSocket message
- Container lifecycle endpoints validate state before acting (start rejects running, stop rejects stopped)
- Worktree delete is a compound operation: stop (if running) -> destroy container -> git worktree remove; failure at any step aborts and returns error
- Worktree start resolves path via WorktreeDir first; falls back to project root for main worktrees (no .worktrees/main directory exists)

## Key Files
- `server.go` - Server struct, constructor, lifecycle (Listen/Serve/Start/Shutdown), SPA handler, health endpoint
- `api.go` - REST handlers for containers, sessions, projects, worktrees, and container lifecycle; JSON response types; project-container matching logic
- `events.go` - SSE event broker (subscribe/notify fan-out) and `/api/events` handler
- `terminal.go` - WebSocket terminal bridge with PTY I/O and resize (`bridgePTYWebSocket` shared helper, `HandleTerminal` for containers, `HandleHostTerminal` for host)
- `host.go` - Host tmux session handlers (list/create/destroy via `os/exec`); `parseHostSessions` uses consolidated `tmux.ParseListSessions` to parse output
- `host_test.go` - Tests for `parseHostSessions`
- `embed.go` - `//go:embed` directive for frontend/dist
- `frontend/` - React SPA (Vite + React + TypeScript + Tailwind)
- `frontend/src/lib/` - Shared utilities: `smartActions.ts` (types), `useSmartActions.ts` (hook), `useServerEvents.ts` (SSE hook)
- `frontend/src/lib/detectors/` - Pluggable smart action detectors (registry in `index.ts`); `handoffDetector.ts` detects Claude Code plugin handoff patterns
- `frontend/src/components/SmartActionOverlay.tsx` - Floating overlay for terminal smart actions (dismissible banners with one-click action buttons)
- `frontend/src/components/HostCard.tsx` - Host tmux session card (list/create/destroy); renders at top of container tree; uses sentinel ID `__host__`
- `frontend/src/components/ProjectCard.tsx` - Project view with worktree radio selection, container lifecycle actions (start/stop/destroy), worktree create/delete
- `frontend/src/components/ContainerCard.tsx` - Container card with inline lifecycle buttons (start/stop/destroy)
- `frontend/src/lib/useConfirmAction.ts` - Hook for inline destructive action confirmations with auto-dismiss timeout

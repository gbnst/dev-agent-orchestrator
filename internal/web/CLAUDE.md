# Web Domain

Last verified: 2026-02-22

## Purpose
HTTP/WebSocket server providing a REST API and embedded React SPA for managing containers and terminal sessions from a browser.

## Contracts
- **Exposes**: `Server`, `New()`, `Config`, `ContainerResponse`, `SessionResponse`, `CreateSessionRequest`, `ResizeMessage`
- **Guarantees**: API responses are JSON. Session mutations notify TUI via `p.Send(WebSessionActionMsg{})`. Frontend SPA is embedded via `//go:embed` and served with SPA fallback (unknown paths serve index.html). WebSocket terminal bridges PTY I/O to tmux sessions with resize support. Server disabled by default (port 0). Manager state changes push SSE "refresh" events to all connected browsers via `eventBroker`; frontend auto-refetches on each event. Host tmux sessions are managed directly via `os/exec` (no container runtime needed); host mutations use sentinel container ID `__host__` for TUI notifications.
- **Expects**: Valid `container.Manager`, `logging.LoggerProvider`, and `func(tea.Msg)` for TUI notifications. Frontend must be built before Go binary (`make frontend-build`). Host session endpoints require `tmux` installed on the host (gracefully degrade to empty list if tmux is unavailable).

## API Routes
- `GET /api/health` - Health check
- `GET /api/containers` - List all containers with sessions
- `GET /api/containers/{id}` - Get single container with sessions
- `GET /api/containers/{id}/sessions` - List sessions for container
- `POST /api/containers/{id}/sessions` - Create tmux session (body: `{"name": "..."}`)
- `DELETE /api/containers/{id}/sessions/{name}` - Destroy tmux session
- `GET /api/containers/{id}/sessions/{name}/terminal` - WebSocket terminal bridge
- `GET /api/host/sessions` - List host tmux sessions (returns empty array if tmux unavailable)
- `POST /api/host/sessions` - Create host tmux session (body: `{"name": "..."}`)
- `DELETE /api/host/sessions/{name}` - Destroy host tmux session
- `GET /api/host/sessions/{name}/terminal` - WebSocket terminal bridge for host tmux session
- `GET /api/events` - SSE stream; sends `event: connected` on open, `event: refresh` when container/session state changes
- `GET /` (and fallback) - Embedded SPA

## Dependencies
- **Uses**: container.Manager, logging.LoggerProvider, tui.WebSessionActionMsg, coder/websocket, creack/pty, os/exec (host tmux)
- **Used by**: main.go only
- **Boundary**: HTTP layer; delegates container business logic to container/tmux packages; host tmux operations call `tmux` CLI directly via `os/exec`

## Key Decisions
- Listen/Serve split: Allows tests to obtain ephemeral port before blocking
- PTY bridge (container): Uses `docker exec -it` with tmux attach, matching Session.AttachCommand() flags
- PTY bridge (host): Uses `tmux -u attach-session` directly on host with TERM/COLORTERM env vars
- Binary frames for terminal data, text frames for control messages (resize)
- SPA fallback: All non-file paths serve index.html for client-side routing
- Frontend embedded at build time via `//go:embed frontend/dist`
- SSE push via Manager.OnChange: Server registers `eventBroker.Notify` as the Manager's onChange callback; eventBroker fans out to all SSE subscribers; frontend `useServerEvents` hook auto-refetches on each event
- Smart actions: Pluggable detector system scans terminal buffer text for patterns and shows floating overlay with one-click actions; detectors registered in `frontend/src/lib/detectors/index.ts`; `typeAndSubmit()` helper delays Enter keystroke to avoid Claude Code autocomplete interception

## Invariants
- Server only starts when `config.Web.Port > 0`
- Session mutations always send `tui.WebSessionActionMsg` to keep TUI in sync (host sessions use `ContainerID: "__host__"`)
- WebSocket uses `context.Background()` (not request context) after upgrade
- PTY read limit: 1 MB per WebSocket message

## Key Files
- `server.go` - Server struct, constructor, lifecycle (Listen/Serve/Start/Shutdown), SPA handler, health endpoint
- `api.go` - REST handlers for containers and sessions, JSON response types
- `events.go` - SSE event broker (subscribe/notify fan-out) and `/api/events` handler
- `terminal.go` - WebSocket terminal bridge with PTY I/O and resize (`bridgePTYWebSocket` shared helper, `HandleTerminal` for containers, `HandleHostTerminal` for host)
- `host.go` - Host tmux session handlers (list/create/destroy via `os/exec`); `parseHostSessions` parses `tmux list-sessions` output
- `host_test.go` - Tests for `parseHostSessions`
- `embed.go` - `//go:embed` directive for frontend/dist
- `frontend/` - React SPA (Vite + React + TypeScript + Tailwind)
- `frontend/src/lib/` - Shared utilities: `smartActions.ts` (types), `useSmartActions.ts` (hook), `useServerEvents.ts` (SSE hook)
- `frontend/src/lib/detectors/` - Pluggable smart action detectors (registry in `index.ts`); `handoffDetector.ts` detects Claude Code plugin handoff patterns
- `frontend/src/components/SmartActionOverlay.tsx` - Floating overlay for terminal smart actions (dismissible banners with one-click action buttons)
- `frontend/src/components/HostCard.tsx` - Host tmux session card (list/create/destroy); renders at top of container tree; uses sentinel ID `__host__`

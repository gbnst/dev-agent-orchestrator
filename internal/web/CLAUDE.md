# Web Domain

Last verified: 2026-02-20

## Purpose
HTTP/WebSocket server providing a REST API and embedded React SPA for managing containers and terminal sessions from a browser.

## Contracts
- **Exposes**: `Server`, `New()`, `Config`, `ContainerResponse`, `SessionResponse`, `CreateSessionRequest`, `ResizeMessage`
- **Guarantees**: API responses are JSON. Session mutations notify TUI via `p.Send(WebSessionActionMsg{})`. Frontend SPA is embedded via `//go:embed` and served with SPA fallback (unknown paths serve index.html). WebSocket terminal bridges PTY I/O to tmux sessions with resize support. Server disabled by default (port 0). Manager state changes push SSE "refresh" events to all connected browsers via `eventBroker`; frontend auto-refetches on each event.
- **Expects**: Valid `container.Manager`, `logging.LoggerProvider`, and `func(tea.Msg)` for TUI notifications. Frontend must be built before Go binary (`make frontend-build`).

## API Routes
- `GET /api/health` - Health check
- `GET /api/containers` - List all containers with sessions
- `GET /api/containers/{id}` - Get single container with sessions
- `GET /api/containers/{id}/sessions` - List sessions for container
- `POST /api/containers/{id}/sessions` - Create tmux session (body: `{"name": "..."}`)
- `DELETE /api/containers/{id}/sessions/{name}` - Destroy tmux session
- `GET /api/containers/{id}/sessions/{name}/terminal` - WebSocket terminal bridge
- `GET /api/events` - SSE stream; sends `event: connected` on open, `event: refresh` when container/session state changes
- `GET /` (and fallback) - Embedded SPA

## Dependencies
- **Uses**: container.Manager, logging.LoggerProvider, tui.WebSessionActionMsg, coder/websocket, creack/pty
- **Used by**: main.go only
- **Boundary**: HTTP layer; delegates all business logic to container/tmux packages

## Key Decisions
- Listen/Serve split: Allows tests to obtain ephemeral port before blocking
- PTY bridge: Uses `docker exec -it` with tmux attach, matching Session.AttachCommand() flags
- Binary frames for terminal data, text frames for control messages (resize)
- SPA fallback: All non-file paths serve index.html for client-side routing
- Frontend embedded at build time via `//go:embed frontend/dist`
- SSE push via Manager.OnChange: Server registers `eventBroker.Notify` as the Manager's onChange callback; eventBroker fans out to all SSE subscribers; frontend `useServerEvents` hook auto-refetches on each event
- Smart actions: Pluggable detector system scans terminal buffer text for patterns and shows floating overlay with one-click actions; detectors registered in `frontend/src/lib/detectors/index.ts`; `typeAndSubmit()` helper delays Enter keystroke to avoid Claude Code autocomplete interception

## Invariants
- Server only starts when `config.Web.Port > 0`
- Session mutations always send `tui.WebSessionActionMsg` to keep TUI in sync
- WebSocket uses `context.Background()` (not request context) after upgrade
- PTY read limit: 1 MB per WebSocket message

## Key Files
- `server.go` - Server struct, constructor, lifecycle (Listen/Serve/Start/Shutdown), SPA handler, health endpoint
- `api.go` - REST handlers for containers and sessions, JSON response types
- `events.go` - SSE event broker (subscribe/notify fan-out) and `/api/events` handler
- `terminal.go` - WebSocket terminal bridge with PTY I/O and resize
- `embed.go` - `//go:embed` directive for frontend/dist
- `frontend/` - React SPA (Vite + React + TypeScript + Tailwind)
- `frontend/src/lib/` - Shared utilities: `smartActions.ts` (types), `useSmartActions.ts` (hook), `useServerEvents.ts` (SSE hook)
- `frontend/src/lib/detectors/` - Pluggable smart action detectors (registry in `index.ts`); `handoffDetector.ts` detects Claude Code plugin handoff patterns
- `frontend/src/components/SmartActionOverlay.tsx` - Floating overlay for terminal smart actions (dismissible banners with one-click action buttons)

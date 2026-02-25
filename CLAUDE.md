# devagent

Last verified: 2026-02-25

## Tech Stack
- Language: Go 1.21+
- TUI: Bubbletea + Bubbles + Lipgloss
- Logging: Zap + Lumberjack (file rotation)
- Container Runtime: Docker or Podman (auto-detected)
- Orchestration: Docker Compose (all containers created via compose)
- Devcontainers: @devcontainers/cli
- Network Isolation: mitmproxy (sidecar container with domain allowlist)
- Web UI: React + Vite + TypeScript + Tailwind (embedded SPA)
- Tailscale Exposure: tsnsrv (optional, proxies web UI onto tailnet)
- Process Supervision: internal/process (restart policies, graceful shutdown)
- Terminal Bridge: coder/websocket + creack/pty
- File Locking: gofrs/flock (cross-platform file-based locking for single-instance enforcement)

## Commands
- `make build` - Build binary
- `make run` - Run with ~/.config/devagent/
- `make dev` - Run with ./config/ (development)
- `make test` - Run unit tests
- `make test-race` - Run unit tests with race detector
- `make test-e2e` - Run E2E tests (requires container runtime)
- `make frontend-install` - Install frontend npm dependencies
- `make frontend-build` - Build frontend (required before `make build`)
- `make frontend-dev` - Run frontend dev server (hot reload)
- `make frontend-test` - Run frontend tests (vitest)
- `make lint` - Run linter (golangci-lint, configured via `.golangci.yml`)
- `devagent list` - Output JSON project hierarchy with containers (requires running TUI instance, delegates via HTTP to GET /api/projects)
- `devagent cleanup` - Remove stale lock/port files from a crashed instance

## Project Structure
- `internal/logging/` - Structured logging with dual sinks (file + TUI channel)
- `internal/tui/` - Bubbletea TUI with tree navigation, detail panel, log panel
- `internal/container/` - Container lifecycle management (see internal/container/CLAUDE.md for contracts)
- `internal/events/` - Shared message types between web and tui packages (WebSessionActionMsg, WebListenURLMsg, TailscaleURLMsg)
- `internal/instance/` - Single-instance enforcement, instance discovery, and HTTP client (see internal/instance/CLAUDE.md)
- `internal/tmux/` - Tmux session management within containers (see internal/tmux/CLAUDE.md)
- `internal/config/` - Configuration loading and validation (see internal/config/CLAUDE.md for contracts)
- `internal/discovery/` - Project scanner for scan_paths directories (see internal/discovery/CLAUDE.md)
- `internal/worktree/` - Git worktree lifecycle management (see internal/worktree/CLAUDE.md)
- `internal/process/` - Child process supervisor with restart policies (see internal/process/CLAUDE.md)
- `internal/tsnsrv/` - Tailscale tsnsrv integration (see internal/tsnsrv/CLAUDE.md)
- `internal/web/` - HTTP/WebSocket server with REST API and embedded SPA (see internal/web/CLAUDE.md for contracts)
- `internal/web/frontend/` - React SPA (Vite + TypeScript + Tailwind)
- `internal/e2e/` - E2E test utilities
- `config/` - Development config (config.yaml + templates/)
- `config/templates/<name>/` - Template directories (docker-compose.yml.tmpl, devcontainer.json.tmpl, Dockerfile, entrypoint.sh, post-create.sh, containers/)
- `docs/` - Design plans and implementation phases
- `docs/PODMAN.md` - Podman compatibility notes and workarounds

## Conventions
- Functional Core / Imperative Shell pattern (see file header comments)
- Bubbletea model-update-view architecture
- Catppuccin theming via styles.go
- Scoped logging: `container`, `tmux`, `tui` (prefix-matched via MatchesScope)

## Boundaries
- Safe to edit: `internal/`, `main.go`
- Never touch: `go.sum` (regenerate with go mod tidy)

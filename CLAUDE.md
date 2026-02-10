# devagent

Last verified: 2026-02-10

## Tech Stack
- Language: Go 1.21+
- TUI: Bubbletea + Bubbles + Lipgloss
- Logging: Zap + Lumberjack (file rotation)
- Container Runtime: Docker or Podman (auto-detected)
- Orchestration: Docker Compose (all containers created via compose)
- Devcontainers: @devcontainers/cli
- Network Isolation: mitmproxy (sidecar container with domain allowlist)

## Commands
- `make build` - Build binary
- `make run` - Run with ~/.config/devagent/
- `make dev` - Run with ./config/ (development)
- `make test` - Run unit tests
- `make test-race` - Run unit tests with race detector
- `make test-e2e` - Run E2E tests (requires container runtime)
- `make lint` - Run linter
- `devagent list` - Output JSON data about all managed containers (for external tool integration)

## Project Structure
- `internal/logging/` - Structured logging with dual sinks (file + TUI channel)
- `internal/tui/` - Bubbletea TUI with tree navigation, detail panel, log panel
- `internal/container/` - Container lifecycle management (see internal/container/CLAUDE.md for contracts)
- `internal/tmux/` - Tmux session management within containers
- `internal/config/` - Configuration loading and validation
- `internal/e2e/` - E2E test utilities
- `config/` - Development config (config.yaml + templates/)
- `config/templates/<name>/` - Template directories (docker-compose.yml.tmpl, devcontainer.json.tmpl, Dockerfile, .gitignore, home/, proxy/)
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

# devagent

Last verified: 2026-02-01

## Tech Stack
- Language: Go 1.21+
- TUI: Bubbletea + Bubbles + Lipgloss
- Logging: Zap + Lumberjack (file rotation)
- Container Runtime: Docker or Podman (auto-detected)
- Devcontainers: @devcontainers/cli

## Commands
- `make build` - Build binary
- `make run` - Run with ~/.config/devagent/
- `make dev` - Run with ./config/ (development)
- `make test` - Run unit tests
- `make test-e2e` - Run E2E tests (requires container runtime)
- `make lint` - Run linter

## Project Structure
- `internal/logging/` - Structured logging with dual sinks (file + TUI channel)
- `internal/tui/` - Bubbletea TUI with tree navigation, detail panel, log panel
- `internal/container/` - Container lifecycle management
- `internal/tmux/` - Tmux session management within containers
- `internal/config/` - Configuration loading and validation
- `internal/e2e/` - E2E test utilities
- `config/` - Development config (config.yaml + templates/)
- `config/templates/<name>/devcontainer.json` - Native devcontainer templates with devagent extensions
- `docs/` - Design plans and implementation phases

## Conventions
- Functional Core / Imperative Shell pattern (see file header comments)
- Bubbletea model-update-view architecture
- Catppuccin theming via styles.go
- Scoped logging: `container`, `tmux`, `tui` (prefix-matched via MatchesScope)

## Boundaries
- Safe to edit: `internal/`, `main.go`
- Never touch: `go.sum` (regenerate with go mod tidy)

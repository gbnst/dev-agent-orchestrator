# devagent

A TUI for orchestrating development agent containers with integrated Claude Code support.

## Features

- **Container Management**: Create, start, stop, and destroy devcontainers from templates
- **Session Management**: Create and manage tmux sessions within containers
- **Claude Code Integration**: Automatic auth token injection and persistent per-project configuration
- **Multi-Runtime Support**: Works with Docker or Podman (auto-detected)
- **Live Logging**: Real-time log panel with scope filtering

## Prerequisites

### Container Runtime (choose one)

#### Option A: Docker

1. Install Docker Desktop:
   - macOS: https://docs.docker.com/desktop/install/mac-install/
   - Or via Homebrew: `brew install --cask docker`

2. Start Docker Desktop

#### Option B: Podman (macOS)

1. Install Podman:
   ```bash
   brew install podman
   ```

2. Initialize and start the Podman machine:
   ```bash
   podman machine init --cpus 4 --memory 4096 --disk-size 100
   podman machine start
   ```

3. Install the Podman-Docker compatibility helper (for devcontainer CLI):
   ```bash
   sudo /opt/homebrew/Cellar/podman/$(podman --version | cut -d' ' -f3)/bin/podman-mac-helper install
   podman machine stop
   podman machine start
   ```

4. Create a `docker` symlink for the devcontainer CLI:
   ```bash
   sudo ln -sf /opt/homebrew/bin/podman /usr/local/bin/docker
   ```

### Devcontainer CLI

Install the devcontainer CLI:
```bash
npm install -g @devcontainers/cli
```

### Claude Code Authentication (Optional)

To enable automatic Claude Code authentication in containers, create a long-lived auth token:

```bash
# Create the auth token file (get token from Claude Code settings)
echo "your-auth-token" > ~/.claude/create-auth-token
```

This token will be automatically injected into containers as `CLAUDE_CODE_OAUTH_TOKEN`.

## Configuration

Configuration files live in `~/.config/devagent/`:

- `config.yaml` - Main settings (theme, runtime, credentials, base images, agents)
- `templates/` - Devcontainer templates

See `config/` directory for examples.

### Runtime Selection

In `config.yaml`, set the runtime explicitly:

```yaml
# Use Docker
runtime: docker

# Or use Podman
runtime: podman

# Or leave empty for auto-detection (tries docker first, then podman)
# runtime:
```

## Usage

```bash
# Build
make build

# Run (uses ~/.config/devagent/)
make run

# Development mode (uses ./config/)
make dev
```

### Keybindings

#### Navigation

| Key | Action |
|-----|--------|
| `↑/↓` | Navigate tree items |
| `Enter` | Expand/collapse containers |
| `→` | Open detail panel |
| `←/Esc` | Close detail panel / return focus to tree |
| `Tab` | Cycle panel focus (tree → detail → logs) |
| `l/L` | Toggle log panel |

#### Container Operations

| Key | Action |
|-----|--------|
| `c` | Create new container |
| `s` | Start selected container |
| `x` | Stop selected container |
| `d` | Destroy selected container (with confirmation) |
| `r` | Refresh container list |

#### Session Operations

| Key | Action |
|-----|--------|
| `t` | Create new tmux session |
| `a` | Attach to selected session |
| `k` | Kill selected session (with confirmation) |

#### General

| Key | Action |
|-----|--------|
| `Ctrl+d` | Quit |
| `Ctrl+c Ctrl+c` | Quit (double-press) |

## Development

```bash
make test         # Run unit tests
make test-e2e     # Run E2E tests (requires Docker/Podman)
make lint         # Run linter
make clean        # Clean build artifacts
```

## Data Storage

devagent stores persistent data in XDG-compliant directories:

- `~/.config/devagent/` - Configuration files
- `~/.local/share/devagent/claude-configs/` - Per-container Claude Code settings (persists across container recreations)

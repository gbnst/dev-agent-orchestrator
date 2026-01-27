# devagent

A TUI for orchestrating development agent containers.

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

#### Container List View

| Key | Action |
|-----|--------|
| `c` | Create new container |
| `s` | Start selected container |
| `x` | Stop selected container |
| `d` | Destroy selected container |
| `r` | Refresh container list |
| `Enter` | Open session view for selected container |
| `q` | Quit |

#### Session View

| Key | Action |
|-----|--------|
| `t` | Create new tmux session |
| `k` | Kill selected session |
| `↑/↓` | Navigate sessions |
| `Esc` | Back to container list |
| `q` | Back to container list |

## Development

```bash
make test         # Run unit tests
make test-e2e     # Run E2E tests (requires Docker/Podman)
make lint         # Run linter
make clean        # Clean build artifacts
```

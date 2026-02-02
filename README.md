# devagent

A TUI for orchestrating development agent containers with integrated Claude Code support.

## Features

- **Container Management**: Create, start, stop, and destroy devcontainers from templates
- **Session Management**: Create and manage tmux sessions within containers
- **Claude Code Integration**: Automatic auth token injection and persistent per-project configuration
- **Multi-Runtime Support**: Works with Docker or Podman (auto-detected)
- **Container Isolation**: Security hardening with capability dropping, resource limits, and network allowlisting
- **Network Isolation**: Domain-based egress filtering via mitmproxy sidecar containers
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

### Container Isolation

devagent applies security isolation to containers by default. Isolation settings are configured per-template in the `customizations.devagent.isolation` section of `devcontainer.json`.

#### Default Isolation

When no isolation config is specified in a template, devagent applies secure defaults:

**Capabilities Dropped:**
- `NET_RAW` - Prevents raw socket access (mitigates network attacks)
- `SYS_ADMIN` - Prevents mount namespace manipulation
- `SYS_PTRACE` - Prevents process tracing
- `MKNOD` - Prevents device node creation
- `NET_ADMIN` - Prevents network configuration changes
- `SYS_MODULE` - Prevents kernel module loading
- `SYS_RAWIO` - Prevents raw I/O operations
- `SYS_BOOT` - Prevents reboot
- `SYS_NICE` - Prevents priority manipulation
- `SYS_RESOURCE` - Prevents resource limit manipulation

**Resource Limits:**
- Memory: 4GB
- CPUs: 2
- Process limit: 512

**Network Allowlist (default domains):**
- `api.anthropic.com` - Claude API
- `github.com`, `*.github.com`, `api.github.com`, `raw.githubusercontent.com`, `objects.githubusercontent.com` - GitHub
- `registry.npmjs.org` - npm
- `pypi.org`, `files.pythonhosted.org` - Python packages
- `proxy.golang.org`, `sum.golang.org`, `storage.googleapis.com`, `pkg.go.dev` - Go modules

#### Template Isolation Configuration

Configure isolation in your template's `devcontainer.json`:

```json
{
  "customizations": {
    "devagent": {
      "isolation": {
        "enabled": true,
        "caps": {
          "drop": ["NET_RAW", "SYS_ADMIN"],
          "add": []
        },
        "resources": {
          "memory": "4g",
          "cpus": "2",
          "pidsLimit": 512
        },
        "network": {
          "allowlist": ["api.anthropic.com", "github.com"],
          "allowlistExtend": ["my-internal-api.example.com"],
          "passthrough": ["pinned-cert-service.example.com"]
        }
      }
    }
  }
}
```

**Configuration Options:**

| Field | Description |
|-------|-------------|
| `enabled` | Set to `false` to disable isolation entirely (default: `true`) |
| `caps.drop` | Linux capabilities to drop (replaces defaults if specified) |
| `caps.add` | Linux capabilities to add (use sparingly) |
| `resources.memory` | Memory limit (e.g., "4g", "512m") |
| `resources.cpus` | CPU limit (e.g., "2", "0.5") |
| `resources.pidsLimit` | Maximum number of processes |
| `network.allowlist` | Allowed domains (replaces defaults if specified) |
| `network.allowlistExtend` | Additional domains to add to defaults |
| `network.passthrough` | Domains that bypass TLS interception (for cert-pinned services) |

#### Network Isolation

When a network allowlist is configured, devagent creates a mitmproxy sidecar container that filters egress traffic:

- Only domains in the allowlist can be accessed
- HTTPS traffic is intercepted to enforce domain filtering
- Domains in `passthrough` bypass TLS interception (for services with certificate pinning)
- The proxy's CA certificate is automatically installed in the devcontainer

The sidecar is automatically managed:
- Created before the devcontainer starts
- Destroyed when the devcontainer is destroyed
- Shares a dedicated Docker network with the devcontainer

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

**Container Creation:**

When creating a container (`c`), the form displays real-time progress with step-by-step feedback:

1. **Network creation** - Creates isolated Docker network (if network isolation enabled)
2. **Proxy sidecar** - Starts mitmproxy container (if network isolation enabled)
3. **Config generation** - Generates devcontainer.json with all settings
4. **Devcontainer startup** - Builds and starts the container

Each step shows a spinner while in progress and a checkmark when complete. Press `Esc` to cancel during creation, or `Enter`/`Esc` to close after completion.

#### Session Operations

| Key | Action |
|-----|--------|
| `t` | Open action menu (on container) / Create new tmux session (on session) |
| `k` | Kill selected session (with confirmation) |

**Action Menu (`t` on running container):**

The action menu shows copyable commands for interacting with the container:
- Open in VS Code
- Create tmux session (named or auto-named)
- Interactive shell

Use `↑/↓` to navigate and `Enter` to copy the selected command to clipboard. Press `Esc` to close.

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

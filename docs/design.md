# Devagent Design Document

**Status:** Draft
**Last Updated:** 2025-01-26

## MVP Goal

A TUI that lets you:
1. **Create** a devcontainer from a template (select template + project path)
2. **Start** the container
3. **See** container status and tmux sessions in the TUI
4. **Attach** to a tmux session in a separate terminal (manually: `docker exec -it <container> tmux attach`)
5. **Stop** and **destroy** containers from the TUI

This is the core workflow before adding OTEL monitoring or agent state detection.

---

## Definition of Done

1. **A Go TUI application** that runs on the host machine

2. **Devcontainer management:**
   - Reads high-level config files (XDG-compliant) that generate devcontainer.json
   - Creates devcontainers via devcontainer CLI, manages lifecycle via Docker/Podman CLI
   - Injects agent credentials as environment variables at container start
   - Supports multiple containers per project (parallel experiments)

3. **Tmux session management:**
   - Creates and tracks tmux sessions inside containers
   - Provides session list/status in TUI
   - User attaches to sessions separately for interaction
   - TUI can inject input via send-keys (for environment setup), observe via capture-pane

4. **Agent state monitoring (passive):** *(future)*
   - Embedded OTEL collector receives telemetry from agents
   - Agent-specific adapters parse state from multiple sources (OTEL, filesystem via docker exec, terminal output)
   - Displays agent state in TUI (working, waiting for input, idle, error)
   - Claude Code adapter first; abstraction layer for future agents

5. **Configuration system:**
   - XDG-compliant config directory with templates
   - Defines available agents, base images, credential references
   - Helps user create new devcontainer configs from templates

---

## Resolved Trade-offs

| Trade-off | Decision |
|-----------|----------|
| OTEL as sole state source vs. multi-source | **Multi-source**: OTEL + filesystem + terminal parsing per agent type |
| Devcontainer CLI only vs. Docker directly | **Hybrid**: devcontainer CLI for up/config, Docker CLI for lifecycle |
| TUI suspends for interaction vs. observer model | **Observer + injector**: TUI monitors via capture-pane, injects via send-keys, user attaches separately |
| Go program controls agents vs. passive monitoring | **Passive**: Go configures env vars, monitors state; user starts/prompts agents manually |

---

## Scope

### In Scope

- Devcontainer lifecycle (create, start, stop, destroy)
- Tmux session lifecycle (create, list, destroy)
- Tmux observation (capture-pane) and injection (send-keys for setup)
- OTEL telemetry collection and display
- Agent state detection via multiple sources
- Config file management and template system
- Agent credential injection via environment variables

### Out of Scope

- Agent lifecycle control (starting/stopping agents)
- Agent command/prompt generation
- Git operations or code management
- IDE integration
- Multi-user/auth scenarios

---

## Technical Constraints

| Area | Constraint | Rationale |
|------|------------|-----------|
| Container runtime | Docker CLI (Podman compatible) | Both implement Docker CLI interface; no abstraction needed |
| Devcontainer CLI | Shell out, no Go SDK | Official CLI is TypeScript; `devcontainers-go` only parses config |
| OTEL | Claude Code only agent with native support | Other agents require terminal/filesystem parsing |
| TUI Framework | Bubbletea + Bubbles + Lipgloss | Matches cc_session_mon; mature ecosystem |
| Container:Project | 1:1, but many containers can work on same project | Supports parallel experiments |

---

## Architecture

### Project Structure

```
devagent/
├── main.go
├── go.mod / go.sum
├── flake.nix / flake.lock
├── Makefile
├── config.example.yaml
├── CLAUDE.md
├── internal/
│   ├── config/                     # XDG-compliant configuration
│   │   ├── config.go              # Main config loading
│   │   ├── templates.go           # Devcontainer template management
│   │   └── agents.go              # Agent definitions
│   │
│   ├── container/                  # Container lifecycle
│   │   ├── types.go               # Container, ContainerState
│   │   ├── runtime.go             # Docker CLI wrapper
│   │   ├── devcontainer.go        # devcontainer CLI + JSON generation
│   │   └── manager.go             # Container lifecycle orchestration
│   │
│   ├── tmux/                       # Tmux session management
│   │   ├── types.go               # Session, Pane types
│   │   ├── client.go              # Tmux CLI wrapper
│   │   └── manager.go             # Session lifecycle
│   │
│   ├── agent/                      # Agent state monitoring
│   │   ├── types.go               # AgentState, StateSource interface
│   │   ├── detector.go            # Multi-source state aggregation
│   │   ├── claude.go              # Claude Code adapter
│   │   └── registry.go            # Agent type registry
│   │
│   ├── otel/                       # OpenTelemetry collector
│   │   ├── receiver.go            # Embedded OTLP gRPC receiver
│   │   ├── store.go               # In-memory telemetry store
│   │   └── events.go              # Event types from agents
│   │
│   └── tui/                        # Bubbletea TUI
│       ├── model.go               # Application state
│       ├── update.go              # Event handling
│       ├── view.go                # Main view rendering
│       ├── views/
│       │   ├── containers.go
│       │   ├── sessions.go
│       │   └── detail.go
│       ├── styles.go
│       └── delegates.go
```

### Domain Model

Hierarchical: Container → Sessions → Agent State

```go
type Container struct {
    ID          string
    Name        string
    ProjectPath string
    Config      *DevcontainerConfig
    State       ContainerState  // created, running, stopped
    Sessions    []*TmuxSession
    CreatedAt   time.Time
}

type TmuxSession struct {
    Name        string
    ContainerID string
    Panes       []*Pane
    AgentState  *AgentState
    CreatedAt   time.Time
}

type AgentState struct {
    Type         AgentType    // claude, codex, etc.
    Status       AgentStatus  // working, waiting, idle, error
    LastActivity time.Time
    Sources      []StateSource
}
```

---

## Configuration System

### File Locations (XDG)

- `~/.config/devagent/config.yaml` - Main settings
- `~/.config/devagent/templates/` - Devcontainer templates

### Main Config

See `config/config.example.yaml` for full example.

Key sections:
- `theme` - Catppuccin color theme
- `otel.grpc_port` - Port for embedded OTLP receiver
- `credentials` - Map of credential names to host env var names
- `base_images` - Named base images for templates
- `agents` - Agent definitions with OTEL env vars and state sources

### Templates

Templates define reusable devcontainer configurations:
- Reference base images by name
- Specify devcontainer features and customizations
- List credentials to inject
- Set default agent type

Generated `devcontainer.json` merges:
1. Base image from config
2. Template settings
3. Project-specific overrides
4. Agent OTEL environment variables
5. Credential values from host environment

---

## Component Details

### Container Runtime (`internal/container/`)

Uses Docker CLI commands (Podman compatible):
- `docker ps` - List containers
- `docker start/stop/rm` - Lifecycle
- `docker exec` - Run commands inside container

Devcontainer CLI for:
- `devcontainer up` - Create/start from config
- `devcontainer read-configuration` - Parse devcontainer.json

### Tmux Client (`internal/tmux/`)

Executes tmux commands inside containers via `docker exec`:
- `tmux new-session -d -s <name>` - Create session
- `tmux list-sessions` - List sessions
- `tmux capture-pane -t <session> -p` - Read pane content
- `tmux send-keys -t <session> "<input>" Enter` - Inject input
- `tmux kill-session -t <name>` - Destroy session

User attaches separately: `docker exec -it <container> tmux attach -t <session>`

### OTEL Receiver (`internal/otel/`)

Embedded OTLP gRPC receiver using `go.opentelemetry.io/collector/receiver/otlpreceiver`:
- Listens on configurable port (default 4317)
- Receives metrics, logs, traces from agents
- Stores recent telemetry in memory
- Exposes events to TUI via channel or polling

### Agent State Detection (`internal/agent/`)

Each agent type implements state detection from multiple sources:

```go
type StateSource interface {
    Detect(ctx context.Context, session *TmuxSession) (AgentStatus, error)
}

type AgentAdapter interface {
    Name() string
    OTELEnvVars() map[string]string
    StateSources() []StateSource
}
```

Claude Code adapter uses:
1. **OTEL events** - `tool_result`, `user_prompt` timing
2. **Terminal parsing** - Prompt patterns in capture-pane output
3. **Filesystem** - Session JSONL files via `docker exec cat`

State aggregation: Most recent signal wins, with confidence weighting.

### TUI (`internal/tui/`)

Follows cc_session_mon patterns:
- Elm Architecture (Model-View-Update)
- Bubbletea framework with Bubbles list component
- Catppuccin theming via Lipgloss
- Multiple views: Containers, Sessions, Detail

Key messages:
- `containerDiscoveredMsg` - New container found
- `sessionEventMsg` - Session state change
- `otelEventMsg` - Telemetry received
- `tickMsg` - Periodic refresh

---

## Data Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                         Host Machine                             │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    devagent TUI                          │    │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐               │    │
│  │  │Container │  │  Tmux    │  │  Agent   │               │    │
│  │  │ Manager  │  │ Manager  │  │ Detector │               │    │
│  │  └────┬─────┘  └────┬─────┘  └────┬─────┘               │    │
│  │       │              │             │                     │    │
│  │       │              │             │    ┌─────────────┐  │    │
│  │       │              │             └────│ OTEL Store  │  │    │
│  │       │              │                  └──────┬──────┘  │    │
│  │       │              │                         │         │    │
│  └───────┼──────────────┼─────────────────────────┼─────────┘    │
│          │              │                         │              │
│    docker│cli      docker│exec              :4317 │gRPC          │
│          │              │                         │              │
│  ┌───────▼──────────────▼─────────────────────────▼─────────┐    │
│  │                   Devcontainer                            │    │
│  │  ┌─────────────────────────────────────────────────────┐ │    │
│  │  │                  tmux server                         │ │    │
│  │  │  ┌─────────────┐  ┌─────────────┐                   │ │    │
│  │  │  │  Session 1  │  │  Session 2  │                   │ │    │
│  │  │  │ Claude Code │  │ Claude Code │                   │ │    │
│  │  │  │     ↓       │  │     ↓       │                   │ │    │
│  │  │  │ OTEL ──────────────────────────→ host:4317       │ │    │
│  │  │  └─────────────┘  └─────────────┘                   │ │    │
│  │  └─────────────────────────────────────────────────────┘ │    │
│  └──────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

---

## User Workflow

1. **Launch devagent** - TUI starts, OTEL receiver starts listening
2. **Create devcontainer** - Select template, specify project path → container created
3. **Create tmux session** - Inside container, with agent env vars configured
4. **Attach to session** - User runs `docker exec -it <container> tmux attach -t <session>`
5. **Start agent** - User launches claude/codex and provides initial prompt
6. **Monitor** - TUI shows all containers/sessions with agent status
7. **Detach** - User detaches from tmux (Ctrl+B D), agent continues
8. **Watch status** - TUI updates with OTEL events and terminal state
9. **Intervene** - User reattaches when agent needs input

---

## Dependencies

Based on cc_session_mon patterns:

**Core TUI:**
- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/bubbles`
- `github.com/charmbracelet/lipgloss`
- `github.com/catppuccin/go`

**Configuration:**
- `gopkg.in/yaml.v3`

**OTEL:**
- `go.opentelemetry.io/collector/receiver/otlpreceiver`
- `go.opentelemetry.io/proto/otlp` (for proto types)

**Devcontainer config parsing (optional):**
- `github.com/kontainment/devcontainers-go`

---

## Open Questions

1. **Project naming** - "devagent"? Something else?
2. **State persistence** - Should container/session state persist across TUI restarts?
3. **Multiple OTEL ports** - One port for all containers, or per-container ports?
4. **Template discovery** - Scan templates dir, or explicit registration?

---

## Implementation Phases

### Phase 1: Foundation ✅
- Project scaffolding (Nix flake, Makefile, go.mod)
- Config loading from XDG paths
- Basic TUI with container list view

### Phase 2: Container Management ✅
- Docker CLI wrapper
- Devcontainer JSON generation from templates
- Container lifecycle (start, stop, destroy) via keybindings

### Phase 2b: Container Creation UI ✅
- TUI form for creating containers: select template, enter project path, name
- `c` keybinding opens creation form
- Calls devcontainer CLI to create and start

### Phase 3: Tmux Integration ✅
- Tmux CLI wrapper via docker exec
- List sessions per container in TUI (Enter to open session view)
- Create session from TUI (`t` keybinding)
- Display attach command for user to copy
- Session destroy from TUI (`k` keybinding)

### Phase 4: OTEL Receiver *(future)*
- Embedded OTLP gRPC receiver
- In-memory event store
- Wire events to TUI

### Phase 5: Agent State Detection *(future)*
- Claude Code adapter
- Multi-source state aggregation
- Status display in TUI

### Phase 6: Polish *(future)*
- Template management UI
- Credential validation
- Error handling and recovery

---

## Current Status

**Implemented:**
- Config parsing with full schema (theme, runtime, otel, credentials, base_images, agents)
- Template loading from `~/.config/devagent/templates/`
- Docker/Podman CLI wrapper with injectable executor for testing
- Devcontainer JSON generation from templates
- Container manager for lifecycle orchestration
- TUI with container list, keybindings (s/x/d/r/q/c), auto-refresh
- Container creation form (`c` opens form, Enter submits)
- Runtime validation at startup (validates docker/podman binary exists)
- Explicit runtime selection via config with `--docker-path` flag to devcontainer CLI
- Tmux session management:
  - Session view (Enter on container to see sessions)
  - Create session (`t` in session view)
  - Kill session (`k` in session view)
  - Attach command display
- E2E test framework with TUITestRunner

**Not yet implemented:**
- OTEL receiver
- Agent state detection

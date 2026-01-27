# TUI Overhaul and Logging Infrastructure Design

## Summary

This design overhauls the TUI to provide comprehensive visibility into container and session operations. The current implementation has a centered container list with limited feedback during operations, making it difficult to diagnose failures or track what's happening. This redesign introduces a full-width, tab-based interface with persistent tabs for Containers and Sessions, a toggleable split-panel log viewer, and comprehensive status feedback through both inline spinners and a persistent status bar. The approach mirrors patterns proven in the cc_session_mon reference project while maintaining consistency with the existing Bubbletea model-update-view architecture.

The logging infrastructure is rebuilt around Zap with dual output sinks: a rotating JSON file for grep-friendly post-mortem analysis, and a buffered channel feeding the TUI's live log panel. Named loggers are scoped hierarchically (e.g., `container.<id>`, `session.<ctr>.<name>`) enabling automatic filtering based on UI context. When viewing a specific container or investigating an error, pressing L opens logs filtered to the relevant scope. The entire implementation is sequenced across seven phases, starting with the logging foundation and progressively layering in layout, navigation, feedback mechanisms, and integration throughout the application.

## Definition of Done

**Deliverables:**
1. Structured logging with Zap + Lumberjack (rotation, named loggers, file + TUI sinks)
2. Tab-based UI with persistent tabs (Containers, Sessions) following cc_session_mon patterns
3. Split-panel log viewer (L key, 40/60 split, live tail filtered by scope)
4. Status feedback system (inline spinners + persistent status bar)
5. Drill-down navigation (select container → view its sessions)

**Success criteria:**
- All container operations show immediate visual feedback
- Logs are filterable by scope in TUI and greppable in file
- UI matches cc_session_mon tab/layout patterns
- Works on macOS and Linux with Docker Desktop or Podman

**Out of scope:**
- OTEL/telemetry integration
- Agent state monitoring
- Windows testing

## Glossary

- **Bubbletea**: A Go framework for building terminal user interfaces using the Elm architecture (model-update-view pattern)
- **Bubbles**: Official component library for Bubbletea providing reusable UI elements like lists, viewports, and text inputs
- **Catppuccin**: A pastel color scheme used for TUI theming
- **cc_session_mon**: Reference project providing design patterns for tab-based layouts and split panels
- **Delegate**: A Bubbletea pattern for customizing how list items are rendered and styled
- **Drill-down navigation**: UI pattern where selecting an item in one view navigates to a detail view focused on that item
- **Lipgloss**: Go library for styling and layout in terminal interfaces, providing CSS-like composition
- **Lumberjack**: Go library providing log rotation by size, age, and backup count
- **Model-Update-View**: Architecture pattern separating state (model), state transitions (update), and rendering (view)
- **Ring buffer**: Fixed-size circular buffer where new entries overwrite oldest entries, used here to limit memory for log storage
- **Scope**: Hierarchical identifier for logs (e.g., `app`, `container.abc123`, `session.ctr.mysession`) enabling contextual filtering
- **Sink**: In logging systems, a destination for log output (file, console, network, etc.)
- **Spinner**: Animated indicator showing an operation is in progress
- **tea.Cmd**: Bubbletea type representing an asynchronous side effect that produces a message when complete
- **Tee core**: Zap logging configuration that duplicates log entries to multiple sinks simultaneously
- **Viewport**: Scrollable view component from Bubbles for displaying content larger than available screen space
- **Zap**: High-performance structured logging library for Go with support for multiple output formats and sinks

## Architecture

Two major components: a centralized logging infrastructure and a redesigned TUI layout.

### Logging Infrastructure

```
┌─────────────────────────────────────────────────────────────┐
│                      LogManager                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              Zap Logger (Tee Core)                   │    │
│  │  ┌──────────────────┐  ┌──────────────────────────┐ │    │
│  │  │   File Sink      │  │   Channel Sink           │ │    │
│  │  │  (JSON + rotate) │  │  (Console → TUI buffer)  │ │    │
│  │  └──────────────────┘  └──────────────────────────┘ │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  Named Logger Registry: map[string]*zap.Logger              │
│    "app"                    → base application logs          │
│    "container.<id>"         → per-container logs             │
│    "session.<ctr>.<name>"   → per-session logs               │
└─────────────────────────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────────────────────┐
│                    TUI Log Consumer                          │
│  - Batched channel reads (up to 50 entries)                 │
│  - Ring buffer (last 1000 entries)                          │
│  - Runtime filtering by scope prefix                        │
└─────────────────────────────────────────────────────────────┘
```

**LogManager** (`internal/logging/manager.go`): Singleton holding the base Zap logger configured with a Tee core. File sink writes JSON with lumberjack rotation (10MB max, 5 backups, 7 days). Channel sink writes console-formatted entries to a buffered channel for TUI consumption.

**Named Loggers**: Components request loggers via `LogManager.For(scope)`. Scopes are hierarchical strings. The registry caches loggers; cleanup happens when containers/sessions are destroyed.

**LogEntry** (`internal/logging/entries.go`): Structured type for TUI display containing timestamp, level, scope, message, and fields.

### TUI Layout

```
┌─────────────────────────────────────────────────────────────┐
│  devagent                              theme: mocha         │ Header (2 lines)
│  Development Agent Orchestrator                             │
├─────────────────────────────────────────────────────────────┤
│  1 Containers  2 Sessions ──────────────────────────────────│ Tabs (1 line)
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Main Content Area (40% when logs open, 100% otherwise)     │
│    - Container list (Tab 1)                                 │
│    - Session list (Tab 2)                                   │
│    - Session detail (Tab 2 + Enter)                         │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  ───────────────────────────────────────────────────────────│ Separator (when logs open)
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Log Panel (60% when open)                                  │
│    - Live tail of filtered log entries                      │
│    - Scrollable viewport                                    │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  ✓ Container started                    c:create L:logs    │ Status Bar (1-4 lines)
└─────────────────────────────────────────────────────────────┘
```

**Layout struct** computes regions based on terminal dimensions. Height budget: 6 fixed lines (header 2, tabs 1, status 1, margins 2) plus dynamic content area. When log panel is open, content splits 40/60 vertically.

**Navigation flow**:
- Tab 1 (Containers): List of managed containers. Enter selects container and switches to Tab 2.
- Tab 2 (Sessions): List of tmux sessions for selected container. Enter opens session detail.
- Session Detail: Placeholder content area for future expansion. Esc returns to session list.

**Log Panel**: Toggled with `L` key. Filter automatically set based on current context (all, container scope, or session scope). Uses Bubbles viewport for scrolling.

**Status Bar**: Shows operation progress (spinner), success (green), error (red), or info (white). Errors display 2 lines of recent error logs plus "(L for full error log)" hint. Contextual help aligned right.

## Existing Patterns

Investigation of current codebase (`internal/tui/`) found:

**Model-Update-View pattern**: Single `Model` struct with all state, `Update()` method handling messages, `View()` method rendering. This design extends this pattern rather than introducing submodels.

**tea.Cmd for async**: All I/O operations (container lifecycle, session management) wrapped in `tea.Cmd` functions returning custom message types. This design follows the same pattern for log consumption and operation feedback.

**Bubbles list.Model**: Used for container list with custom delegate. This design adds a second list for sessions (currently manual rendering) and viewport for logs.

**Catppuccin theming**: Styles defined in `styles.go` using Catppuccin colors. This design extends with new styles for tabs, status bar, and log panel.

**Modal overlays**: Forms rendered as full-screen overlays. This design keeps forms modal but moves session view from modal to persistent tab.

Investigation of reference project (`cc_session_mon`) found:

**Tab bar rendering**: Metadata-driven slice of tabs, conditional styling, gap fill with "─" character.

**Height budgeting**: Fixed 9-line reserve for chrome, explicit minimum heights.

**Split panel**: 58%/42% ratio with separator character, `lipgloss.JoinHorizontal`.

**Keyboard dispatch**: Handler chain pattern with `(Model, bool)` returns indicating whether key was handled.

This design adopts these patterns from cc_session_mon while maintaining consistency with existing devagent patterns.

## Implementation Phases

<!-- START_PHASE_1 -->
### Phase 1: Logging Infrastructure

**Goal:** Establish structured logging with file rotation and TUI-consumable channel sink.

**Components:**
- `internal/logging/manager.go` — LogManager with Zap Tee core, named logger registry
- `internal/logging/sink.go` — ChannelSink implementing zapcore.WriteSyncer
- `internal/logging/entries.go` — LogEntry struct for TUI consumption
- `go.mod` updates — Add `go.uber.org/zap` and `gopkg.in/natefinch/lumberjack.v2`

**Dependencies:** None (first phase)

**Done when:**
- LogManager can be instantiated with file path
- Named loggers can be created and retrieved
- Log entries appear in both file (JSON) and channel (LogEntry structs)
- File rotation triggers at configured size
- Unit tests pass for logger creation, entry routing, and rotation
<!-- END_PHASE_1 -->

<!-- START_PHASE_2 -->
### Phase 2: Layout System

**Goal:** Replace centered layout with full-width tab-based layout using Layout struct for region computation.

**Components:**
- `internal/tui/layout.go` — Layout and Region types, computeLayout() method
- `internal/tui/model.go` — Add TabMode enum, currentTab, logPanelOpen state
- `internal/tui/view.go` — Refactor View() to use layout regions, add renderTabs()
- `internal/tui/styles.go` — Add tab styles (ActiveTabStyle, InactiveTabStyle, TabGapStyle)

**Dependencies:** Phase 1 (logging available for debugging)

**Done when:**
- TUI renders full-width with persistent tab bar
- Tab 1 shows container list, Tab 2 shows "Select container" placeholder
- Layout adjusts correctly on terminal resize
- Visual appearance matches cc_session_mon tab pattern
<!-- END_PHASE_2 -->

<!-- START_PHASE_3 -->
### Phase 3: Tab Navigation and Drill-Down

**Goal:** Implement tab switching and container-to-session drill-down flow.

**Components:**
- `internal/tui/update.go` — Tab switching handlers (1/2 keys, h/l arrows), drill-down on Enter
- `internal/tui/model.go` — Add selectedContainer, selectedSession, sessionDetailOpen state
- `internal/tui/view.go` — Add renderSessionList(), renderSessionDetail() using Layout regions
- `internal/tui/delegates.go` — Add sessionDelegate for consistent list rendering

**Dependencies:** Phase 2 (layout system)

**Done when:**
- Number keys (1/2) and h/l switch between tabs
- Enter on container selects it and switches to Tab 2
- Tab 2 shows sessions for selected container
- Enter on session opens detail view, Esc returns to list
- Backspace in Tab 2 returns to Tab 1
<!-- END_PHASE_3 -->

<!-- START_PHASE_4 -->
### Phase 4: Status Bar

**Goal:** Add persistent status bar with operation feedback and contextual help.

**Components:**
- `internal/tui/model.go` — Add statusMessage, statusLevel, statusSpinner, pendingOperations state
- `internal/tui/view.go` — Add renderStatusBar() with dynamic height for errors
- `internal/tui/styles.go` — Add SuccessStyle, ErrorStyle, InfoStyle
- `internal/tui/update.go` — Add operationStartMsg, operationSuccessMsg, operationErrorMsg handlers

**Dependencies:** Phase 2 (layout system)

**Done when:**
- Status bar shows at bottom of screen
- Operations show spinner while in progress
- Success shows green checkmark, error shows red X
- Errors display 2 log lines + "(L for full error log)"
- Esc clears error state
- Contextual help displays on right side
<!-- END_PHASE_4 -->

<!-- START_PHASE_5 -->
### Phase 5: Inline Operation Feedback

**Goal:** Show spinners on list items during container/session operations.

**Components:**
- `internal/tui/model.go` — pendingOperations map[string]string tracking containerID → operation
- `internal/tui/delegates.go` — Update containerDelegate to show spinner for pending items
- `internal/tui/update.go` — Wire container actions to set/clear pending state

**Dependencies:** Phase 4 (status bar provides spinner component)

**Done when:**
- Starting a container shows spinner on that list item
- Stopping a container shows spinner on that list item
- Spinner clears on success or error
- Multiple concurrent operations show independent spinners
<!-- END_PHASE_5 -->

<!-- START_PHASE_6 -->
### Phase 6: Log Panel

**Goal:** Implement toggleable split-panel log viewer with scope-based filtering.

**Components:**
- `internal/tui/model.go` — Add logEntries ring buffer, logViewport, logFilter, logAutoScroll state
- `internal/tui/view.go` — Add renderLogPanel(), update renderContent() for split layout
- `internal/tui/update.go` — Add L key handler, logEntriesMsg handler, log consumption command
- `internal/tui/styles.go` — Add LogHeaderStyle, log level badge styles

**Dependencies:** Phase 1 (logging infrastructure), Phase 2 (layout system)

**Done when:**
- L key toggles log panel (40% main / 60% logs split)
- Logs filter based on current context (all, container, session)
- New entries appear in real-time (live tail)
- Viewport scrollable with j/k keys
- Error state + L opens logs filtered to error scope
<!-- END_PHASE_6 -->

<!-- START_PHASE_7 -->
### Phase 7: Integration and Polish

**Goal:** Wire logging throughout application and refine user experience.

**Components:**
- `main.go` — Initialize LogManager, pass to TUI
- `internal/container/manager.go` — Add logging to container operations
- `internal/tmux/client.go` — Add logging to tmux operations
- `internal/tui/model.go` — Add LogManager dependency, log TUI events

**Dependencies:** All previous phases

**Done when:**
- All container operations logged with appropriate scopes
- All tmux operations logged with appropriate scopes
- TUI initialization and key events logged at debug level
- Logs visible in both file and TUI panel
- E2E tests pass with new UI structure
<!-- END_PHASE_7 -->

## Additional Considerations

**Error context preservation:** When container operations fail, the error scope is captured so pressing L opens logs filtered to that context. This provides immediate access to relevant diagnostic information.

**Log panel focus:** The log panel does not steal keyboard focus from the main content. Navigation keys (j/k, arrows) work for both the main list and log scrolling based on context. This avoids mode confusion.

**Form compatibility:** Creation forms remain modal overlays rendered on top of the tab structure. This preserves existing form behavior while integrating with the new layout.

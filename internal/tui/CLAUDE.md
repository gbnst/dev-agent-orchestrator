# TUI Domain

Last verified: 2026-02-19

## Purpose
Provides terminal UI for orchestrating development containers. Tree-based navigation showing containers with nested sessions, optional detail panel, live log panel with selectable entries, and log details panel for HTTP request inspection.

## Contracts
- **Exposes**: `Model`, `NewModel()`, `NewModelWithTemplates()`, `StatusLevel`, `TreeItemType`, `TreeItem`, `PanelFocus`, `ActionCommand`, `GenerateContainerActions`, `GenerateVSCodeCommand`, `WebSessionActionMsg`
- **Guarantees**: Operations show immediate visual feedback (spinners). Log panel filters by current context (both container.* and proxy.* scopes). Log entries are selectable with details panel for HTTP request inspection. Forms are modal overlays. Destructive operations require confirmation. Container creation shows real-time progress steps.
- **Expects**: Valid config and LogManager. Container runtime available for operations.

## Dependencies
- **Uses**: logging.Manager (required), container.Manager, config.Config
- **Used by**: main.go, web.Server (via WebSessionActionMsg)
- **Boundary**: UI layer; delegates all business logic to container/tmux packages

## Key Decisions
- Single Model struct: Follows existing Bubbletea pattern over submodels
- Tree navigation (↑/↓, enter): Containers at top level, sessions nested underneath
- 40/60 split: Tree/detail panel when detail panel open; also 40/60 for log list/log details
- Ring buffer (1000): Bounds log memory in TUI
- Confirmation dialogs: Required for destroy container (d) and kill session (k) operations
- Panel header styling: Uses underline to indicate focus (not background color)
- Action menu: Shows copyable commands for container operations (t key on running containers)
- Container creation progress: Real-time step-by-step feedback in creation form via OnProgress callback
- All container lifecycle commands (start/stop/destroy) dispatch directly to compose methods (no IsComposeContainer branching)
- Log filtering: When container selected, matches both container.<name> and proxy.<name> scopes
- Log details panel: Shows full HTTP request/response for proxy logs (headers, bodies) or Fields for regular logs

## Invariants
- NewModel requires non-nil LogManager
- selectedContainer set when container selected in tree
- pendingOperations cleared on success or error
- logAutoScroll true by default; j/k/g/G disable it
- panelFocus defaults to FocusTree (zero value)
- confirmOpen blocks other input until confirmed/cancelled
- cachedIsolationInfo cleared on selection change, refreshed async
- actionMenuOpen blocks other input until closed
- selectedLogIndex reset to end of list when filter changes
- logDetailsOpen only set when log panel has entries

## Key Files
- `model.go` - Model struct, constructors, state management, tree operations, confirmation dialog state
- `update.go` - Message handlers, key dispatch, confirmation dialog handling
- `view.go` - View rendering, tree view, detail panel, log panel, status bar, renderConfirmDialog()
- `actions.go` - Action command generators for container action menu (Functional Core)
- `layout.go` - Layout/Region computation from terminal dimensions
- `styles.go` - Catppuccin-based styling, PanelHeaderFocusedStyle/PanelHeaderUnfocusedStyle (underline-based)
- `form.go` - Form rendering, input handling, validateForm() (Functional Core: pure, returns error string)
- `delegates.go` - List item rendering with spinner support

## Navigation
- `↑/↓` - Navigate tree items (or log entries when log panel focused)
- `enter` - Expand/collapse containers (y/n in confirmation dialogs); open log details when log panel focused
- `→` - Open detail panel (or log details when log panel focused)
- `←/esc` - Close detail panel (esc also returns focus from detail/logs to tree, cancels dialogs, closes log details)
- `tab` - Cycle panel focus (tree → detail → logs → tree)
- `l/L` - Toggle log panel
- `c` - Create container
- `s/x/d` - Start/stop/destroy container (d shows confirmation)
- `t` - Open action menu (running containers) / Create tmux session (on session nodes)
- `k` - Kill session (shows confirmation)
- `ctrl+c ctrl+c` - Quit (double-press within 500ms)
- `ctrl+d` - Quit (immediate)
- Pressing `esc` twice with nothing to close shows quit hint in status bar

## Gotchas
- consumeLogEntries() must be called in Init() to start log flow
- setLogFilterFromContext() called by syncSelectionFromTree() and explicitly after setError() in containerActionMsg handler; callers must invoke it explicitly (setError does not call it)
- setLogFilterFromContext() resets selectedLogIndex to end of filtered list
- rebuildTreeItems() must be called after container list changes
- Layout.ContentListHeight() accounts for list chrome (subtract 2)
- Form inputs are trimmed of whitespace before validation
- Log details panel requires logDetailsReady before rendering

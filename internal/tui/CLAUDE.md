# TUI Domain

Last verified: 2026-02-02

## Purpose
Provides terminal UI for orchestrating development containers. Tree-based navigation showing containers with nested sessions, optional detail panel, and live log panel for debugging operations.

## Contracts
- **Exposes**: `Model`, `NewModel()`, `NewModelWithTemplates()`, `StatusLevel`, `TreeItemType`, `TreeItem`, `PanelFocus`, `ActionCommand`, `GenerateContainerActions`, `GenerateVSCodeCommand`
- **Guarantees**: Operations show immediate visual feedback (spinners). Log panel filters by current context. Forms are modal overlays. Destructive operations require confirmation. Container creation shows real-time progress steps.
- **Expects**: Valid config and LogManager. Container runtime available for operations.

## Dependencies
- **Uses**: logging.Manager (required), container.Manager, config.Config
- **Used by**: main.go only
- **Boundary**: UI layer; delegates all business logic to container/tmux packages

## Key Decisions
- Single Model struct: Follows existing Bubbletea pattern over submodels
- Tree navigation (↑/↓, enter): Containers at top level, sessions nested underneath
- 40/60 split: Tree/detail panel when detail panel open
- Ring buffer (1000): Bounds log memory in TUI
- Confirmation dialogs: Required for destroy container (d) and kill session (k) operations
- Panel header styling: Uses underline to indicate focus (not background color)
- Action menu: Shows copyable commands for container operations (t key on running containers)
- Container creation progress: Real-time step-by-step feedback in creation form via OnProgress callback

## Invariants
- NewModel requires non-nil LogManager
- selectedContainer set when container selected in tree
- pendingOperations cleared on success or error
- logAutoScroll true by default; j/k/g/G disable it
- panelFocus defaults to FocusTree (zero value)
- confirmOpen blocks other input until confirmed/cancelled
- cachedIsolationInfo cleared on selection change, refreshed async
- actionMenuOpen blocks other input until closed

## Key Files
- `model.go` - Model struct, constructors, state management, tree operations, confirmation dialog state
- `update.go` - Message handlers, key dispatch, confirmation dialog handling
- `view.go` - View rendering, tree view, detail panel, log panel, status bar, renderConfirmDialog()
- `actions.go` - Action command generators for container action menu (Functional Core)
- `layout.go` - Layout/Region computation from terminal dimensions
- `styles.go` - Catppuccin-based styling, PanelHeaderFocusedStyle/PanelHeaderUnfocusedStyle (underline-based)
- `delegates.go` - List item rendering with spinner support

## Navigation
- `↑/↓` - Navigate tree items
- `enter` - Expand/collapse containers (y/n in confirmation dialogs)
- `→` - Open detail panel
- `←/esc` - Close detail panel (esc also returns focus from detail/logs to tree, cancels dialogs)
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
- setLogFilterFromContext() called by syncSelectionFromTree(); keeps filter in sync with tree selection
- rebuildTreeItems() must be called after container list changes
- Layout.ContentListHeight() accounts for list chrome (subtract 2)
- Form inputs are trimmed of whitespace before validation

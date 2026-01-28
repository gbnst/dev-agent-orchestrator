# TUI Domain

Last verified: 2026-01-28

## Purpose
Provides terminal UI for orchestrating development containers. Tree-based navigation showing containers with nested sessions, optional detail panel, and live log panel for debugging operations.

## Contracts
- **Exposes**: `Model`, `NewModel()`, `NewModelWithTemplates()`, `StatusLevel`, `TreeItemType`, `TreeItem`
- **Guarantees**: Operations show immediate visual feedback (spinners). Log panel filters by current context. Forms are modal overlays.
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

## Invariants
- NewModel requires non-nil LogManager
- selectedContainer set when container selected in tree
- pendingOperations cleared on success or error
- logAutoScroll true by default; j/k/g/G disable it

## Key Files
- `model.go` - Model struct, constructors, state management, tree operations
- `update.go` - Message handlers, key dispatch
- `view.go` - View rendering, tree view, detail panel, log panel, status bar
- `layout.go` - Layout/Region computation from terminal dimensions
- `styles.go` - Catppuccin-based styling
- `delegates.go` - List item rendering with spinner support

## Navigation
- `↑/↓` - Navigate tree items
- `enter` - Expand/collapse containers
- `→` - Open detail panel
- `←/esc` - Close detail panel
- `l/L` - Toggle log panel
- `c` - Create container
- `s/x/d` - Start/stop/destroy container
- `q` - Quit

## Gotchas
- consumeLogEntries() must be called in Init() to start log flow
- setLogFilterFromContext() auto-sets filter when entering error state
- rebuildTreeItems() must be called after container list changes
- Layout.ContentListHeight() accounts for list chrome (subtract 2)

# TUI Domain

Last verified: 2026-01-27

## Purpose
Provides terminal UI for orchestrating development containers. Tab-based navigation between containers and sessions with live log panel for debugging operations.

## Contracts
- **Exposes**: `Model`, `NewModel()`, `NewModelWithTemplates()`, `TabMode`, `StatusLevel`
- **Guarantees**: Operations show immediate visual feedback (spinners). Log panel filters by current context. Forms are modal overlays.
- **Expects**: Valid config and LogManager. Container runtime available for operations.

## Dependencies
- **Uses**: logging.Manager (required), container.Manager, config.Config
- **Used by**: main.go only
- **Boundary**: UI layer; delegates all business logic to container/tmux packages

## Key Decisions
- Single Model struct: Follows existing Bubbletea pattern over submodels
- Tab navigation (1/2, h/l): Matches cc_session_mon reference project
- 40/60 split: Content/logs ratio when panel open
- Ring buffer (1000): Bounds log memory in TUI

## Invariants
- NewModel requires non-nil LogManager
- Tab 2 (Sessions) requires selectedContainer to be set
- pendingOperations cleared on success or error
- logAutoScroll true by default; j/k/g/G disable it

## Key Files
- `model.go` - Model struct, constructors, state management
- `update.go` - Message handlers, key dispatch
- `view.go` - View rendering, tab bar, log panel, status bar
- `layout.go` - Layout/Region computation from terminal dimensions
- `styles.go` - Catppuccin-based styling
- `delegates.go` - List item rendering with spinner support

## Gotchas
- consumeLogEntries() must be called in Init() to start log flow
- setLogFilterFromContext() auto-sets filter when entering error state
- Layout.ContentListHeight() accounts for list chrome (subtract 2)

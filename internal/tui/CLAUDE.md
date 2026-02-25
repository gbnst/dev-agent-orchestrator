# TUI Domain

Last verified: 2026-02-25

## Purpose
Provides terminal UI for orchestrating development containers and git worktrees. Tree-based navigation showing projects with nested worktrees, containers, and sessions. Optional detail panel, live log panel with selectable entries, and log details panel for HTTP request inspection. Supports worktree creation/destruction within projects.

## Contracts
- **Exposes**: `Model`, `NewModel()`, `NewModelWithTemplates()`, `SetDiscoveredProjects()`, `StatusLevel`, `TreeItemType`, `TreeItem`, `PanelFocus`, `ActionCommand`, `GenerateContainerActions`, `GenerateVSCodeCommand`
- **Guarantees**: Operations show immediate visual feedback (spinners). Log panel filters by current context (both container.* and proxy.* scopes). Log entries are selectable with details panel for HTTP request inspection. Forms are modal overlays. Destructive operations require confirmation. Container creation and worktree creation show forms with input validation. Header displays active listen URLs (web + tailscale).
- **Expects**: Valid config and LogManager. Container runtime available for operations. Git binary available for worktree operations.

## Dependencies
- **Uses**: logging.Manager (required), container.Manager, config.Config, discovery.Scanner, worktree package (via DestroyWorktreeWithContainer compound operation)
- **Used by**: main.go, web.Server (via WebSessionActionMsg)
- **Boundary**: UI layer; delegates all business logic to container/tmux/worktree/discovery packages

## Key Decisions
- Single Model struct: Follows existing Bubbletea pattern over submodels
- Tree structure (Phase 3): Projects at top level, worktrees nested under projects (including "main" branch), containers nested under worktrees. "Other" group for unmatched containers when projects exist.
- Form overlay strategy: worktreeFormOpen checked BEFORE formOpen in Update() so worktree form takes precedence
- Worktree form: Simpler than container form (just branch name input), reuses form styling
- Worktree deletion: Only allowed for non-main worktrees (W key, shows confirmation). Uses shared worktree.DestroyWorktreeWithContainer function to perform compound stop+destroy+remove operation, aligning semantics with Web API.
- Project scanning: async rescanProjects() command after worktree create/destroy to refresh tree
- 40/60 split: Tree/detail panel when detail panel open; also 40/60 for log list/log details
- Ring buffer (1000): Bounds log memory in TUI
- Confirmation dialogs: Required for destroy container (d), kill session (k), and destroy worktree (W) operations
- Panel header styling: Uses underline to indicate focus (not background color)
- Action menu: Shows copyable commands for container operations (t key on running containers)
- Container creation progress: Real-time step-by-step feedback in creation form via OnProgress callback
- All container lifecycle commands (start/stop/destroy) dispatch directly to compose methods (no IsComposeContainer branching)
- Log filtering: Hierarchical scope — container selected filters to that container's name, worktree selected filters to all containers matching that worktree path, project selected filters to all containers under the project. Matches both container.<name> and proxy.<name> scopes
- Log details panel: Shows full HTTP request/response for proxy logs (headers, bodies) or Fields for regular logs

## Invariants
- NewModel requires non-nil LogManager
- SetDiscoveredProjects() called before Bubbletea program starts; sets discoveredProjects field (used in Phase 3)
- selectedContainer set when container selected in tree; cleared when project/worktree selected
- pendingOperations cleared on success or error
- pendingWorktrees cleared on success or error; spinner ticks when len(pendingWorktrees) > 0
- logAutoScroll true by default; j/k/g/G disable it
- panelFocus defaults to FocusTree (zero value)
- confirmOpen blocks other input until confirmed/cancelled
- cachedIsolationInfo cleared on selection change, refreshed async
- actionMenuOpen blocks other input until closed
- worktreeFormOpen blocks other input until closed or Esc pressed
- selectedLogIndex reset to end of list when filter changes
- logDetailsOpen only set when log panel has entries
- expandedProjects map tracks expansion state for each project (keyed by projectPath, "__other__" for unmatched group)
- rebuildTreeItems() must be called after discoveredProjects change, containerList change, or project/container expansion toggle

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
- `enter` - Expand/collapse projects/containers (y/n in confirmation dialogs); open log details when log panel focused
- `→` - Open detail panel (or log details when log panel focused)
- `←/esc` - Close detail panel (esc also returns focus from detail/logs to tree, cancels dialogs, closes log details)
- `tab` - Cycle panel focus (tree → detail → logs → tree)
- `l/L` - Toggle log panel
- `c` - Create container
- `w` - Create worktree (opens form for selected project or first project if "All Projects" selected)
- `W` - Delete worktree (shows confirmation, only on non-main worktrees)
- `s/x/d` - Start/stop/destroy container (d shows confirmation); `s` on containerless worktree starts a new container via devcontainer up
- `t` - Open action menu (running containers) / Create tmux session (on session nodes)
- `k` - Kill session (shows confirmation)
- `ctrl+c ctrl+c` - Quit (double-press within 500ms)
- `ctrl+d` - Quit (immediate)
- Pressing `esc` twice with nothing to close shows quit hint in status bar

## Gotchas
- consumeLogEntries() must be called in Init() to start log flow
- setLogFilterFromContext() called by syncSelectionFromTree() and explicitly after setError() in containerActionMsg handler; callers must invoke it explicitly (setError does not call it)
- setLogFilterFromContext() resets selectedLogIndex to end of filtered list
- rebuildTreeItems() must be called after container list changes or discovered projects change
- worktreeFormOpen checked BEFORE formOpen in View() and Update() so worktree form takes precedence over container form
- rescanProjects() uses config.ResolveScanPaths() to get scan directories; must match what discovery.Scanner was initialized with
- projectsRefreshedMsg triggers refreshContainers() to keep container list in sync after project rescan
- Layout.ContentListHeight() accounts for list chrome (subtract 2)
- Form inputs are trimmed of whitespace before validation
- Log details panel requires logDetailsReady before rendering

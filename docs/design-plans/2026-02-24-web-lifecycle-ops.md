# Web Lifecycle Operations Design

## Summary

This design extends the devagent web UI to support the full project/worktree/container hierarchy that the TUI already exposes, and adds lifecycle controls — start, stop, destroy, create worktree, delete worktree, create session, attach session — so users can operate their development environments entirely through a browser.

The approach has two halves. On the backend, a new `GET /api/projects` endpoint assembles a structured response by combining the discovery scanner's project list (which already knows about git worktrees on disk) with the container manager's live container state, matching them by filesystem path. Three additional endpoint groups cover container lifecycle, worktree lifecycle, and reuse the existing session endpoints. On the frontend, the current flat list of `ContainerCard` components is replaced by `ProjectCard` components that mirror the TUI's tree hierarchy: projects at the top level, worktrees nested inside, tmux sessions nested inside worktrees. A radio-button selection model within each card drives a context-sensitive action bar at the card's bottom, keeping the UI compact while surfacing different actions depending on what is selected and what state it is in. SSE events already in use for the existing session UI continue to drive live re-fetching, so the new cards stay in sync without polling.

## Definition of Done
The web UI displays containers grouped under collapsible project cards (matching the TUI's tree hierarchy), with worktrees and their containers nested inside each project card. The host card remains unchanged at the top. Users can start, stop, and destroy containers; create worktrees (with automatic container start) scoped to a project; and delete worktrees — all through the web UI. No details pane, log pane, template-based container creation, or action menu is included. Backend API endpoints exist for all new operations, with SSE events keeping the UI in sync.

## Acceptance Criteria

### web-lifecycle-ops.AC1: Projects and worktrees displayed in hierarchy
- **web-lifecycle-ops.AC1.1 Success:** `GET /api/projects` returns projects with worktrees and nested containers matched by path
- **web-lifecycle-ops.AC1.2 Success:** Main worktree appears with `is_main: true`, other worktrees with `is_main: false`
- **web-lifecycle-ops.AC1.3 Edge:** Project with no containers returns worktrees with `container: null`
- **web-lifecycle-ops.AC1.4 Success:** Web UI renders each project as a collapsible card with worktrees and sessions nested inside
- **web-lifecycle-ops.AC1.5 Success:** Expand/collapse state persists across page reloads via localStorage

### web-lifecycle-ops.AC2: Container lifecycle operations
- **web-lifecycle-ops.AC2.1 Success:** `POST /api/containers/{id}/start` starts a stopped container
- **web-lifecycle-ops.AC2.2 Success:** `POST /api/containers/{id}/stop` stops a running container
- **web-lifecycle-ops.AC2.3 Success:** `DELETE /api/containers/{id}` destroys a container
- **web-lifecycle-ops.AC2.4 Failure:** Start/stop/destroy on nonexistent container returns 404
- **web-lifecycle-ops.AC2.5 Failure:** Start on already-running container returns 400
- **web-lifecycle-ops.AC2.6 Success:** Action bar shows correct buttons based on container state (start for stopped, stop+destroy for running)
- **web-lifecycle-ops.AC2.7 Success:** Destroy requires inline confirmation before executing

### web-lifecycle-ops.AC3: Worktree lifecycle operations
- **web-lifecycle-ops.AC3.1 Success:** `POST /api/projects/{path}/worktrees` creates worktree and auto-starts container
- **web-lifecycle-ops.AC3.2 Failure:** Create worktree with invalid name returns 400
- **web-lifecycle-ops.AC3.3 Failure:** Create worktree with duplicate branch name returns 409
- **web-lifecycle-ops.AC3.4 Success:** `DELETE /api/projects/{path}/worktrees/{name}` performs compound stop+destroy+git-remove
- **web-lifecycle-ops.AC3.5 Failure:** Delete worktree with dirty working tree returns error with descriptive message
- **web-lifecycle-ops.AC3.6 Success:** "New worktree" radio selection shows branch name input and Create button
- **web-lifecycle-ops.AC3.7 Success:** Delete worktree requires inline confirmation ("This will also destroy its container")

### web-lifecycle-ops.AC4: Confirmation and error handling
- **web-lifecycle-ops.AC4.1 Success:** Destructive actions (destroy container, delete worktree, destroy session) show inline "Confirm?" before executing
- **web-lifecycle-ops.AC4.2 Success:** Error messages from failed operations display in the action bar area
- **web-lifecycle-ops.AC4.3 Success:** Action buttons show loading/disabled state during async operations

### web-lifecycle-ops.AC5: Session actions and unchanged behavior
- **web-lifecycle-ops.AC5.1 Success:** Session radio selection shows Attach and Destroy buttons
- **web-lifecycle-ops.AC5.2 Success:** Attach opens terminal tab (existing terminal view behavior)
- **web-lifecycle-ops.AC5.3 Success:** Running worktree selection includes Create Session action
- **web-lifecycle-ops.AC5.4 Success:** Unmatched containers appear under "Other" card with start/stop/destroy only

## Glossary

- **Worktree**: A git feature that checks out an additional branch from the same repository into a separate directory. In devagent, each worktree gets its own devcontainer so multiple branches can run in parallel without interfering with each other.
- **Main worktree**: The original checkout directory for a project (as opposed to linked worktrees created with `git worktree add`). Represented with `is_main: true` in the API response.
- **Devcontainer**: A containerized development environment defined by `.devcontainer/docker-compose.yml` and `devcontainer.json`. devagent manages these containers' full lifecycle.
- **TUI**: Terminal User Interface — the existing keyboard-driven interface built with Bubbletea. The web UI is being built to offer the same capabilities in a browser.
- **SSE (Server-Sent Events)**: A browser API for receiving a stream of server-pushed messages over a single HTTP connection. devagent uses SSE to push `refresh` signals to the frontend whenever backend state changes, triggering a re-fetch.
- **ProjectCard**: A new React component (introduced in this design) that renders one discovered project as a collapsible card, containing its worktrees, sessions, and an action bar.
- **ContainerTree**: The existing React component responsible for the top-level layout of cards. This design updates it to consume `GET /api/projects` instead of `GET /api/containers`.
- **Action bar**: A strip at the bottom of each `ProjectCard` that renders context-sensitive buttons — the set of buttons changes depending on which radio item is selected and what state it is in.
- **Radio selection model**: A UI pattern where exactly one item within a card can be selected at a time (like an HTML radio group), determining what the action bar shows.
- **Inline confirmation**: A two-click pattern for destructive actions — the button first changes its label to "Confirm?" and executes only on the second click, avoiding a modal dialog.
- **Compound operation**: A server-side sequence of steps treated as a single API call. Worktree deletion is a compound operation: stop container → destroy container → git worktree remove.
- **Discovery scanner**: The `internal/discovery` package that walks `scan_paths` directories, identifies devagent-managed projects by a compose label, and runs `git worktree list` for each one.
- **Dirty worktree**: A git worktree that has uncommitted changes. `git worktree remove` refuses to delete a dirty worktree by default; the design surfaces this as a descriptive error.
- **Path encoding (base64)**: Project paths are absolute filesystem paths that cannot be safely embedded as URL path segments; the design encodes them with `base64.URLEncoding` in Go and `btoa()` in the browser.
- **Compose override file**: A supplementary `docker-compose.worktree.yml` file that the worktree package writes alongside the project's main `docker-compose.yml`, adding worktree-specific volume mounts without modifying the original file.

## Architecture

Single hierarchical API endpoint (`GET /api/projects`) returns the full project tree with containers and sessions nested inside. The frontend replaces its current flat container list with collapsible `ProjectCard` components that render the tree. A radio-button selection model within each card drives a dynamic action bar at the card's bottom.

### Backend: New Endpoint and Lifecycle APIs

**`GET /api/projects`** returns the full hierarchy:

```go
type ProjectResponse struct {
    Name        string              `json:"name"`
    Path        string              `json:"path"`
    HasMakefile bool                `json:"has_makefile"`
    Worktrees   []WorktreeResponse  `json:"worktrees"`
}

type WorktreeResponse struct {
    Name      string              `json:"name"`       // branch name ("main" for root)
    Path      string              `json:"path"`       // absolute worktree path
    IsMain    bool                `json:"is_main"`    // true for project root
    Container *ContainerResponse  `json:"container"`  // null if no container exists
}
```

The endpoint calls a scanner function to get discovered projects, then matches containers from `manager.List()` to worktrees by comparing `container.ProjectPath` to worktree paths — same logic the TUI uses in `findContainersForPath`.

**Container lifecycle endpoints:**

- `POST /api/containers/{id}/start` — calls `manager.StartWithCompose(ctx, id)`
- `POST /api/containers/{id}/stop` — calls `manager.StopWithCompose(ctx, id)`
- `DELETE /api/containers/{id}` — calls `manager.DestroyWithCompose(ctx, id)`

**Worktree lifecycle endpoints:**

- `POST /api/projects/{encodedPath}/worktrees` — body: `{"name": "branch-name"}`. Calls `worktree.Create(projectPath, name)` then `manager.StartWorktreeContainer(ctx, worktreePath)`. Returns created worktree info.
- `DELETE /api/projects/{encodedPath}/worktrees/{name}` — compound operation: stop container (if running) → destroy container → `worktree.Destroy(projectPath, name)`. Returns error if git refuses (dirty worktree, unmerged branch).

Project paths are base64-encoded in URLs since they're absolute filesystem paths that don't work as URL path segments.

All mutating endpoints trigger `s.events.broadcast("refresh")` and `s.notifyTUI(...)`, following the existing pattern from session endpoints.

### Backend: Plumbing

`web.New()` gains a `scanner func(context.Context) []discovery.DiscoveredProject` parameter. In `main.go`, this wraps `discovery.Scanner.ScanAll()`. The `Server` struct stores this function and calls it on each `GET /api/projects` request.

The server also needs access to `worktree.Create` and `worktree.Destroy` — these are package-level functions, so no additional dependency injection is needed.

### Frontend: ProjectCard Component

`ProjectCard` is a collapsible card (top-level item) containing:

1. **Worktree rows** — each showing branch name + status indicator (● running, ○ stopped, ◌ no container)
2. **Session rows** — nested under each worktree, showing session name
3. **"New worktree" radio option** — at the bottom of the worktree list
4. **Action bar** — at the card's bottom, renders context-sensitive buttons

Each entity (worktree, session, "new worktree") has a radio button. Only one can be selected at a time within a card. The action bar updates dynamically:

| Selection | Actions |
|-----------|---------|
| Main worktree (running) | Stop, Destroy, Create Session |
| Main worktree (stopped) | Start |
| Main worktree (no container) | — |
| Other worktree (running) | Stop, Delete, Create Session |
| Other worktree (stopped) | Start, Delete |
| Other worktree (no container) | Delete |
| Session | Attach, Destroy |
| New worktree | Branch name input + Create |

"Delete worktree" is a compound operation handled server-side: the frontend calls `DELETE /api/projects/{path}/worktrees/{name}` and the backend handles stop → destroy → git remove.

"Create Session" shows a name input inline in the action bar. "Attach" opens a terminal tab via the existing terminal view.

### Frontend: Data Flow

`ContainerTree` switches from `GET /api/containers` to `GET /api/projects` as its primary data source. On SSE `refresh` events, it re-fetches `GET /api/projects` and re-renders all ProjectCards.

Selection state is local to each `ProjectCard`. No global selection state needed.

### Frontend: "Other" Group

Containers not matched to any project appear under an "Other" card — matching TUI behavior. This card only shows container lifecycle actions (start/stop/destroy), no worktree operations.

### Confirmation and Error Handling

**Destructive actions require inline confirmation** — button changes to "Confirm?" on first click, executes on second. This keeps the UI compact versus a modal dialog.

- Destroy container: confirmation required
- Delete worktree: confirmation required ("This will also destroy its container")
- Stop container: no confirmation (reversible)
- Destroy session: confirmation required

**Error states** display in the action bar area:
- "Failed to start container: [message]"
- "Cannot delete worktree: uncommitted changes" (git safety)
- "Cannot delete worktree: branch not fully merged" (git safety)

**Loading states:** Action buttons show spinner/disabled state during async operations. Worktree creation shows "Creating..." then "Starting container..." as it progresses.

## Existing Patterns

### Backend Patterns Followed

**SSE refresh + TUI notification** — all existing mutating endpoints (`POST/DELETE /api/containers/{id}/sessions`, `POST/DELETE /api/host/sessions`) call `s.events.broadcast("refresh")` and `s.notifyTUI(msg)`. New lifecycle endpoints follow this pattern exactly.

**Container manager methods** — `StartWithCompose`, `StopWithCompose`, `DestroyWithCompose` are existing methods on `container.Manager` (used by TUI). New endpoints call these directly.

**Worktree package functions** — `worktree.Create()` and `worktree.Destroy()` are existing package-level functions (used by TUI). The web backend calls them directly.

**ContainerResponse/SessionResponse types** — existing response types in `internal/web/api.go`. Reused in the new `WorktreeResponse` which embeds `ContainerResponse`.

### Frontend Patterns Followed

**Collapsible cards with localStorage persistence** — `ContainerCard` and `HostCard` both use expand/collapse with `devagent-expanded-cards` localStorage key. `ProjectCard` follows the same pattern.

**SSE-driven re-fetch** — `useServerEvents()` hook triggers `fetchContainers()` on `refresh` events. `ContainerTree` switches to `fetchProjects()` but same reactive pattern.

**api.ts fetch wrappers** — existing pattern of typed async functions (`fetchContainers`, `createSession`, etc.). New functions follow same pattern.

### New Pattern: Radio Selection + Dynamic Action Bar

This is a new interaction pattern not present in the current web UI. The TUI uses keyboard-driven selection; the web UI introduces radio-button selection with a dynamic action bar. This pattern is specific to `ProjectCard` and does not affect other components.

## Implementation Phases

<!-- START_PHASE_1 -->
### Phase 1: Backend API — Projects Endpoint and Plumbing

**Goal:** Expose project/worktree hierarchy to the frontend via a new `GET /api/projects` endpoint.

**Components:**
- `internal/web/server.go` — add `scanner` field to `Server` struct, update `New()` constructor
- `internal/web/api.go` — add `ProjectResponse`, `WorktreeResponse` types and `handleGetProjects` handler
- `internal/web/server.go` — register `GET /api/projects` route
- `main.go` — pass scanner function to `web.New()`

**Dependencies:** None (first phase)

**Done when:** `GET /api/projects` returns correctly structured JSON with projects, worktrees, and nested containers matched by path. Covers `web-lifecycle-ops.AC1.1`, `web-lifecycle-ops.AC1.2`, `web-lifecycle-ops.AC1.3`.
<!-- END_PHASE_1 -->

<!-- START_PHASE_2 -->
### Phase 2: Backend API — Container Lifecycle Endpoints

**Goal:** Expose start, stop, and destroy operations for containers via REST.

**Components:**
- `internal/web/api.go` — add `handleStartContainer`, `handleStopContainer`, `handleDestroyContainer` handlers
- `internal/web/server.go` — register routes: `POST /api/containers/{id}/start`, `POST /api/containers/{id}/stop`, `DELETE /api/containers/{id}`

**Dependencies:** Phase 1 (server plumbing)

**Done when:** Container start/stop/destroy work via API, trigger SSE refresh and TUI notifications, return appropriate errors for invalid states. Covers `web-lifecycle-ops.AC2.1` through `web-lifecycle-ops.AC2.5`.
<!-- END_PHASE_2 -->

<!-- START_PHASE_3 -->
### Phase 3: Backend API — Worktree Lifecycle Endpoints

**Goal:** Expose worktree creation (with auto-start) and deletion via REST.

**Components:**
- `internal/web/api.go` — add `handleCreateWorktree`, `handleDeleteWorktree` handlers
- `internal/web/server.go` — register routes: `POST /api/projects/{encodedPath}/worktrees`, `DELETE /api/projects/{encodedPath}/worktrees/{name}`

**Dependencies:** Phase 2 (container lifecycle for compound delete)

**Done when:** Worktree create returns new worktree with running container. Worktree delete performs compound stop+destroy+git-remove and returns errors for dirty/unmerged states. Covers `web-lifecycle-ops.AC3.1` through `web-lifecycle-ops.AC3.5`.
<!-- END_PHASE_3 -->

<!-- START_PHASE_4 -->
### Phase 4: Frontend — ProjectCard Component with Radio Selection

**Goal:** Replace flat container list with project-grouped collapsible cards using radio selection model.

**Components:**
- `internal/web/frontend/src/components/ProjectCard.tsx` — new component: collapsible card, worktree/session rows, radio buttons, action bar
- `internal/web/frontend/src/components/ContainerTree.tsx` — switch from `fetchContainers` to `fetchProjects`, render ProjectCards
- `internal/web/frontend/src/api.ts` — add `ProjectResponse`, `WorktreeResponse` types, `fetchProjects()` function

**Dependencies:** Phase 1 (projects endpoint)

**Done when:** Web UI shows projects as collapsible cards with worktrees and sessions nested inside, radio selection works, expand/collapse persists in localStorage. Covers `web-lifecycle-ops.AC1.4`, `web-lifecycle-ops.AC1.5`.
<!-- END_PHASE_4 -->

<!-- START_PHASE_5 -->
### Phase 5: Frontend — Container Lifecycle Actions

**Goal:** Wire start/stop/destroy buttons in the action bar to backend endpoints.

**Components:**
- `internal/web/frontend/src/components/ProjectCard.tsx` — action bar renders lifecycle buttons based on selection, inline confirmation for destructive actions
- `internal/web/frontend/src/api.ts` — add `startContainer()`, `stopContainer()`, `destroyContainer()` functions

**Dependencies:** Phase 2 (container lifecycle endpoints), Phase 4 (ProjectCard component)

**Done when:** Users can start, stop, and destroy containers from the web UI. Confirmations work for destructive actions. Error and loading states display correctly. Covers `web-lifecycle-ops.AC2.6`, `web-lifecycle-ops.AC2.7`, `web-lifecycle-ops.AC4.1`, `web-lifecycle-ops.AC4.2`.
<!-- END_PHASE_5 -->

<!-- START_PHASE_6 -->
### Phase 6: Frontend — Worktree Lifecycle Actions

**Goal:** Wire worktree create and delete in the action bar.

**Components:**
- `internal/web/frontend/src/components/ProjectCard.tsx` — "New worktree" selection shows branch name input + Create button; delete button for non-main worktrees with inline confirmation
- `internal/web/frontend/src/api.ts` — add `createWorktree()`, `deleteWorktree()` functions

**Dependencies:** Phase 3 (worktree endpoints), Phase 4 (ProjectCard component)

**Done when:** Users can create worktrees (container auto-starts) and delete worktrees from the web UI. Git safety errors display clearly. Covers `web-lifecycle-ops.AC3.6`, `web-lifecycle-ops.AC3.7`, `web-lifecycle-ops.AC4.3`.
<!-- END_PHASE_6 -->

<!-- START_PHASE_7 -->
### Phase 7: Frontend — Session Actions and "Other" Group

**Goal:** Wire session attach/destroy in action bar. Handle unmatched containers.

**Components:**
- `internal/web/frontend/src/components/ProjectCard.tsx` — session selection shows Attach + Destroy buttons; Create Session on running worktree selection
- `internal/web/frontend/src/components/ContainerTree.tsx` — render "Other" card for unmatched containers with lifecycle-only actions
- `internal/web/frontend/src/components/ContainerCard.tsx` — may be retired or retained only for "Other" group

**Dependencies:** Phase 5, Phase 6

**Done when:** Session attach opens terminal tab, session destroy works from action bar, Create Session works for running containers, unmatched containers appear under "Other" with start/stop/destroy. Covers `web-lifecycle-ops.AC5.1` through `web-lifecycle-ops.AC5.4`.
<!-- END_PHASE_7 -->

## Additional Considerations

**`GET /api/containers` retained:** The existing endpoint stays for backward compatibility (`devagent list` CLI). The frontend switches to `GET /api/projects` but the old endpoint is not removed.

**Scanner performance:** `GET /api/projects` calls the scanner on each request, which walks `ScanPaths` and runs `git worktree list`. For typical configurations (a few scan paths, dozens of projects), this is fast enough. If it becomes a bottleneck, caching with SSE invalidation can be added later.

**Path encoding:** Project paths are base64-encoded in URLs using Go's `base64.URLEncoding` (URL-safe alphabet, no padding issues). The frontend encodes with `btoa()` and the backend decodes in the handler.

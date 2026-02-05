# Code Quality Remediation Design

## Summary

This design implements fixes for 14 code quality issues identified in a full-project review of the devagent codebase. The primary focus is addressing race conditions in concurrent access to shared data structures, removing dead code that accumulated during the migration from direct Docker API usage to Docker Compose orchestration, and reducing hardcoded values that create maintenance friction. Two critical race conditions are fixed by adding `sync.RWMutex` protection: one in the logging package's `ChannelSink` (preventing send-on-closed-channel panics) and one in the container package's `Manager` (protecting concurrent map access from Bubbletea command goroutines). Dead code removal targets legacy lifecycle methods (`Start`, `Stop`, `Destroy`) that were replaced by compose-specific methods, unused constructors, and obsolete JSON generation fallbacks. Hardcoded values like proxy image names, ports, and user names are moved from Go source files into the existing template data system for easier configuration.

The remediation follows established patterns already present in the codebase: the mutex implementation mirrors the pattern used in `internal/logging/manager.go`, the template data changes extend the existing `TemplateData` struct in `compose.go`, and all container operations continue using the Bubbletea `tea.Cmd` pattern. Additional improvements include making the TUI's `validateForm()` function pure (following the Functional Core pattern documented in file headers), making side effects explicit in error handling, adding a `make test-race` build target, and including concurrency unit tests to prevent regression.

## Definition of Done

Fix all 14 issues identified in the full-project code quality review. This includes: fixing race conditions on shared maps with mutex protection (verified by `go test -race`), removing dead code (unused `NewManager()` constructor, legacy non-compose lifecycle methods, stale JSON fallback path), moving hardcoded Go values to template data where appropriate, cleaning up FCIS violations (`validateForm()` purity, `setError()` side effect documentation), adding a `make test-race` Makefile target, and adding simple concurrency unit tests. All existing tests must continue to pass.

Out of scope: pre-commit hooks, config.yaml overrides for template values, splitting `update.go` into smaller files, CI/CD pipeline setup.

## Glossary

- **Bubbletea**: A Go TUI (Text User Interface) framework using the Elm architecture pattern (model-update-view). Components return `tea.Cmd` closures for async operations.
- **`tea.Cmd`**: Bubbletea command pattern — functions that perform side effects (like container operations) and return results to the Update loop via messages.
- **FCIS (Functional Core, Imperative Shell)**: Architectural pattern separating pure business logic (functional core) from side effects (imperative shell). Used throughout the codebase.
- **Double-checked locking**: Concurrency pattern where a condition is checked once without a lock, then again inside a lock to minimize contention while ensuring correctness.
- **`sync.RWMutex`**: Go standard library reader/writer mutex allowing multiple concurrent readers or one exclusive writer. Used for protecting shared maps.
- **Race condition**: Concurrency bug where program behavior depends on the timing of operations (e.g., reading a map while another goroutine writes to it).
- **Docker Compose**: Tool for defining and running multi-container Docker applications using YAML configuration files. The project's preferred orchestration method.
- **Template data**: Struct containing values substituted into Go templates (`.tmpl` files) at runtime, replacing hardcoded values.
- **mitmproxy**: HTTP/HTTPS proxy used as a sidecar container for network isolation with domain allowlisting.
- **Sidecar container**: Supporting container deployed alongside a main container (e.g., proxy, logging agent).
- **`go test -race`**: Go testing flag that enables the race detector, which instruments code to detect concurrent access bugs at runtime.
- **Dead code**: Code that is no longer called or reachable, often left behind after refactoring (e.g., legacy lifecycle methods after compose migration).

## Architecture

This is a remediation plan, not a new feature. The architecture remains unchanged — changes are scoped to fixing concurrency bugs, removing dead code, reducing hardcoded fragility, and improving FCIS adherence within the existing package structure.

**Concurrency fixes** add `sync.RWMutex` protection to two locations:
- `ChannelSink` in `internal/logging/sink.go` — hold mutex through the entire Write operation to prevent send-on-closed-channel panics
- `Manager` in `internal/container/manager.go` — protect `containers` and `sidecars` maps accessed concurrently by Bubbletea Cmd goroutines

**Dead code removal** deletes three categories:
- Legacy non-compose lifecycle methods (`Start`, `Stop`, `Destroy`) and `IsComposeContainer()` in Manager, plus the corresponding TUI branching in `update.go`
- Unused `NewManager()` constructor (only `NewManagerWithConfigAndLogger()` is used in production)
- Stale struct-based JSON fallback in `WriteToProject()` (all flows now use template-driven output)

**Template hardcode reduction** adds fields to the existing `TemplateData` struct (`ProxyImage`, `ProxyPort`, `RemoteUser`) and updates templates and fallback code to use them instead of string literals.

**FCIS cleanups** make `validateForm()` pure (return error string instead of mutating state) and make the `setError()` → `setLogFilterFromContext()` cascade explicit at call sites.

## Existing Patterns

Investigation found established patterns that this design follows:

- **Mutex pattern**: `internal/logging/manager.go` already uses `sync.RWMutex` with double-checked locking for the logger cache. The Manager mutex follows this same pattern.
- **TemplateData struct**: `internal/container/compose.go` has an existing `TemplateData` struct with `buildTemplateData()` populating it. New fields follow this exact pattern.
- **Constants in types.go**: `internal/container/types.go` already defines label key constants. The hash truncation constant follows this convention.
- **Bubbletea Cmd pattern**: All container operations already return `tea.Cmd` closures. Removing the compose/non-compose branching simplifies these closures but doesn't change the pattern.

No new patterns are introduced.

## Implementation Phases

<!-- START_PHASE_1 -->
### Phase 1: Logging Package Fixes

**Goal:** Fix the race condition in `ChannelSink.Write()` and add a concurrency test.

**Components:**
- `internal/logging/sink.go` — Restructure `Write()` to hold the mutex through the closed-check AND channel-send operations. The `parseEntry()` call (pure, no shared state) stays outside the lock.
- `internal/logging/sink_test.go` — Add a concurrency test: one goroutine writes in a loop while another closes the sink after a delay. Verify no panic under `-race`.

**Dependencies:** None (first phase).

**Done when:** `go test -race ./internal/logging/...` passes with no race detected, and the new test exercises concurrent Write/Close.
<!-- END_PHASE_1 -->

<!-- START_PHASE_2 -->
### Phase 2: Container Package Fixes

**Goal:** Add mutex protection to Manager, remove dead code, reduce hardcoded values, and simplify the TUI's container operation dispatch.

**Components:**
- `internal/container/manager.go` — Add `sync.RWMutex` to Manager struct. Protect `containers` and `sidecars` map access: `RLock` for `List()`, `Get()`, `GetSidecarsForProject()`; `Lock` for `Refresh()`, state mutations in compose lifecycle methods, and map deletions in destroy. Remove legacy `Start()`, `Stop()`, `Destroy()` methods. Remove `IsComposeContainer()`. Remove unused `NewManager()` constructor.
- `internal/container/manager_test.go` — Add concurrency tests: concurrent `Refresh()` + `Get()`, concurrent `Refresh()` + `List()`. Remove any tests for deleted methods.
- `internal/container/devcontainer.go` — Remove the struct-based JSON generation fallback in `WriteToProject()` (lines 313-324).
- `internal/container/compose.go` — Add `ProxyImage`, `ProxyPort`, `RemoteUser` fields to `TemplateData` struct. Populate in `buildTemplateData()` with defaults (`mitmproxy/mitmproxy:latest`, `8080`, value from config or `DefaultRemoteUser`). Update fallback `Dockerfile.proxy` generation to use `ProxyImage` from template data.
- `internal/container/types.go` — Add `const HashTruncLen = 12`. Update usages in `devcontainer.go`, `proxy.go`, and `manager.go` to use the constant.
- `config/templates/basic/docker-compose.yml.tmpl` — Update proxy image reference and port to use `{{.ProxyImage}}` and `{{.ProxyPort}}` where applicable.
- `config/templates/basic/Dockerfile.proxy` — Update `FROM` line to use build arg or keep as-is if only the Go fallback path needs fixing.
- `config/templates/basic/devcontainer.json.tmpl` — Update `remoteUser` to use `{{.RemoteUser}}`.
- `config/templates/go-project/` — Same template updates as basic.
- `internal/tui/update.go` — Simplify `startContainer()`, `stopContainer()`, `destroyContainer()` to call compose methods directly, removing the `IsComposeContainer()` branching.
- `internal/container/manager.go` — Extract a progress reporting helper to reduce the repeated pattern in `CreateWithCompose()`.

**Dependencies:** Phase 1 (logging fixes should land first to establish the mutex pattern).

**Done when:** `go test -race ./internal/container/...` passes. `go test -race ./internal/tui/...` passes. Legacy methods and dead constructors are gone. Templates use `{{.ProxyImage}}`, `{{.ProxyPort}}`, `{{.RemoteUser}}`. Hash truncation uses the shared constant. Existing tests pass (updated for removed methods).
<!-- END_PHASE_2 -->

<!-- START_PHASE_3 -->
### Phase 3: TUI Package FCIS Cleanups

**Goal:** Improve FCIS adherence in the TUI package and fix a minor UX issue.

**Components:**
- `internal/tui/form.go` — Change `validateForm()` to return a `string` (error message, or empty for valid). Remove direct mutation of `m.formError` from the function.
- `internal/tui/update.go` — Update `handleFormKey()` to assign the return value of `validateForm()` to `m.formError`.
- `internal/tui/model.go` — Extract the `setLogFilterFromContext()` call from `setError()`. Move it to each call site in `Update()` that calls `setError()`, making the log filter side effect explicit.
- `internal/tui/view.go` — Fix the TODO at line 813: use `└─` for the last session in tree rendering instead of always `├─`.
- `internal/tui/form_test.go` — Update tests if `validateForm()` signature changes.

**Dependencies:** Phase 2 (TUI branching simplification should be done first).

**Done when:** `validateForm()` is a pure function returning a string. `setError()` no longer calls `setLogFilterFromContext()`. Tree rendering uses correct connector for last session. All TUI tests pass.
<!-- END_PHASE_3 -->

<!-- START_PHASE_4 -->
### Phase 4: Build Tooling

**Goal:** Add a `make test-race` target for ongoing race condition verification.

**Components:**
- `Makefile` — Add `test-race` target: `go test -race ./...`. Add to `.PHONY`.

**Dependencies:** Phases 1-3 (this is the final verification step).

**Done when:** `make test-race` runs successfully with zero race conditions detected across the entire codebase.
<!-- END_PHASE_4 -->

## Additional Considerations

**False positives dropped from original review:**
- `pendingOperations` race condition — investigation confirmed all access is within the single-threaded Bubbletea Update loop. No fix needed.
- `consumeLogEntries()` goroutine leak — investigation confirmed this is a standard Bubbletea Cmd pattern that returns after batched reads and is re-invoked by the Update handler. No leak.

**E2E test references to non-existent `Create()` method:** Investigation found that `internal/e2e/e2e_test.go` calls `runner.Model().Manager().Create()` which doesn't exist on the current Manager. These E2E tests would fail at compile time if run. This is a pre-existing issue outside the scope of this remediation but worth noting.

# Template File Consolidation Implementation Plan

**Goal:** Consolidate template configuration by embedding all container orchestration settings directly into template files, eliminating separate `devcontainer.json`, `allowlist.txt`, and the Go code that parses them.

**Architecture:** Template directories become the single source of truth. `filter.py.tmpl` replaces allowlist parsing. `devcontainer.json.tmpl` becomes self-contained per template. The `Template` struct is stripped to `Name` and `Path`. All isolation config types and parsing code are removed.

**Tech Stack:** Go 1.21+, text/template, mitmproxy Python addon API

**Scope:** 5 phases from original design (phases 1-5)

**Codebase verified:** 2026-02-05

**Testing conventions:** Standard Go `testing` package only (no testify/gomock). Table-driven tests with `t.Run()`. Manual test doubles. `t.TempDir()` for filesystem tests. `make test` runs `go test ./...`. Reference: `internal/config/templates_test.go`, `internal/container/compose_test.go`. See also `internal/config/CLAUDE.md` and `internal/container/CLAUDE.md` for domain contracts.

---

## Phase 5: Final cleanup and verification

**Goal:** Update documentation to reflect the completed consolidation and verify the entire implementation is correct across all phases.

**Note:** Phase 2 already hardcoded the proxy command in the real template files (replacing `{{.ProxyCommand}}`). Phase 5 focuses on documentation updates and comprehensive verification.

<!-- START_TASK_1 -->
### Task 1: Update container CLAUDE.md for consolidation changes

**Files:**
- Modify: `internal/container/CLAUDE.md`

**Step 1: Update the CLAUDE.md**

In the **Contracts/Exposes** section, remove `AllowlistConfig`. Verify `ComposeOptions` no longer mentions `Isolation`.

In the **Dependencies/Uses** section, remove `config.IsolationConfig` if still listed.

In the **Key Decisions** section:
- Update the "Network isolation via mitmproxy" entry to note that the proxy command is now static (no `--ignore-hosts` flag) and passthrough is handled by the filter script's `load()` hook using `ctx.options.ignore_hosts`.
- Update the "Compose file generation" entry to mention `filter.py.tmpl` processed by `processFilterTemplate()` instead of `GenerateFilterScript()`.

In the **Key Files** section:
- `compose.go` description: Replace "AllowlistConfig, buildProxyCommand" references with "processFilterTemplate(), CertInstallCommand in TemplateData"
- `proxy.go` description: Remove "GenerateFilterScript, filterScriptTemplate, GenerateIgnoreHostsPattern, WriteFilterScript". Keep "ReadAllowlistFromFilterScript, parseAllowlistFromScript".

In the **Gotchas** section: Add note that `filter.py.tmpl` is processed as a Go template but currently has no Go template placeholders (all config is hardcoded in the template).

**Step 2: Commit**

```bash
git add internal/container/CLAUDE.md
git commit -m "docs: update container CLAUDE.md for template consolidation"
```
<!-- END_TASK_1 -->

<!-- START_TASK_2 -->
### Task 2: Verify no remaining ProxyCommand references

**Files:** None (verification only)

**Step 1: Search for ProxyCommand references**

Run: `grep -rn 'ProxyCommand' internal/ config/ --include='*.go' --include='*.tmpl'`

Expected: No hits. All references should have been removed across Phases 2.

If any references remain (e.g., in comments), remove them.

**Step 2: Run full build and tests**

Run: `go build ./...`
Expected: Build succeeds

Run: `make test`
Expected: All tests pass

**Step 3: Run linter**

Run: `make lint`
Expected: No linter errors

If any failures, fix them before proceeding.
<!-- END_TASK_2 -->

<!-- START_TASK_3 -->
### Task 3: Final verification of entire consolidation

**Files:** None (verification only)

**Step 1: Verify all static files removed**

Run: `ls config/templates/basic/devcontainer.json config/templates/go-project/devcontainer.json config/templates/basic/allowlist.txt config/templates/go-project/allowlist.txt 2>&1`

Expected: All four files should report "No such file or directory".

**Step 2: Verify template directory structure**

Run: `find config/templates -type f | sort`

Expected output should show exactly these files per template:
```
config/templates/basic/Dockerfile
config/templates/basic/Dockerfile.proxy
config/templates/basic/devcontainer.json.tmpl
config/templates/basic/docker-compose.yml.tmpl
config/templates/basic/filter.py.tmpl
config/templates/go-project/Dockerfile
config/templates/go-project/Dockerfile.proxy
config/templates/go-project/devcontainer.json.tmpl
config/templates/go-project/docker-compose.yml.tmpl
config/templates/go-project/filter.py.tmpl
```

Plus any static files like `home/` directories if they exist.

**Step 3: Verify no remaining legacy references**

Run: `grep -rn 'IsolationConfig\|GetEffectiveIsolation\|AllowlistConfig\|LoadAllowlist\|GenerateFilterScript\|GenerateIgnoreHostsPattern\|buildProxyCommand\|ProxyCommand\|filterScriptTemplate\|WriteFilterScript' internal/ --include='*.go'`

Expected: No hits except `ReadAllowlistFromFilterScript` and `parseAllowlistFromScript` (preserved for runtime query).

**Step 4: Run full test suite**

Run: `make test`
Expected: All tests pass

Run: `make lint`
Expected: No linter errors

**Step 5: Verify Template struct is minimal**

Run: `grep -A 5 'type Template struct' internal/config/templates.go`

Expected:
```go
type Template struct {
	Name string
	Path string
}
```

**Step 6: Verify TemplateData has no legacy fields**

Run: `grep -A 12 'type TemplateData struct' internal/container/compose.go`

Expected fields: `ProjectHash`, `ProjectPath`, `ProjectName`, `WorkspaceFolder`, `ClaudeConfigDir`, `TemplateName`, `ContainerName`, `CertInstallCommand`. No `ProxyCommand`, no `PostCreateCommand`, no `Isolation`.
<!-- END_TASK_3 -->

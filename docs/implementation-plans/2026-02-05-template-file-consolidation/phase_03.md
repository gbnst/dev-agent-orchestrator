# Template File Consolidation Implementation Plan

**Goal:** Consolidate template configuration by embedding all container orchestration settings directly into template files, eliminating separate `devcontainer.json`, `allowlist.txt`, and the Go code that parses them.

**Architecture:** Template directories become the single source of truth. `filter.py.tmpl` replaces allowlist parsing. `devcontainer.json.tmpl` becomes self-contained per template. The `Template` struct is stripped to `Name`, `Path`, and `PostCreateCommand`. All isolation config types and parsing code are removed.

**Tech Stack:** Go 1.21+, text/template, mitmproxy Python addon API

**Scope:** 5 phases from original design (phases 1-5)

**Codebase verified:** 2026-02-05

**Testing conventions:** Standard Go `testing` package only (no testify/gomock). Table-driven tests with `t.Run()`. Manual test doubles. `t.TempDir()` for filesystem tests. `make test` runs `go test ./...`. Reference: `internal/config/templates_test.go`, `internal/container/compose_test.go`. See also `internal/config/CLAUDE.md` and `internal/container/CLAUDE.md` for domain contracts.

---

## Phase 3: Consolidate devcontainer.json.tmpl and remove devcontainer.json

**Goal:** Embed `postCreateCommand` (with cert install) directly in each template's `devcontainer.json.tmpl`. Remove static `devcontainer.json` files. Simplify `DevcontainerGenerator` to no longer chain cert install in Go.

<!-- START_SUBCOMPONENT_A (tasks 1-2) -->

<!-- START_TASK_1 -->
### Task 1: Update devcontainer.json.tmpl for basic template

**Files:**
- Modify: `config/templates/basic/devcontainer.json.tmpl`

**Step 1: Update the template**

Currently `config/templates/basic/devcontainer.json.tmpl` (line 7) uses `{{.PostCreateCommand}}` where Go code chains the cert install. Change it to use `{{.CertInstallCommand}}` directly since the basic template has no template-specific postCreateCommand.

Replace the entire file with:

```json
{
  "name": "{{.ContainerName}}",
  "dockerComposeFile": "docker-compose.yml",
  "service": "app",
  "workspaceFolder": "{{.WorkspaceFolder}}",
  "remoteUser": "vscode",
  "postCreateCommand": "{{.CertInstallCommand}}"
}
```

**Step 2: Commit**

```bash
git add config/templates/basic/devcontainer.json.tmpl
git commit -m "feat: basic devcontainer.json.tmpl uses CertInstallCommand directly"
```
<!-- END_TASK_1 -->

<!-- START_TASK_2 -->
### Task 2: Update devcontainer.json.tmpl for go-project template

**Files:**
- Modify: `config/templates/go-project/devcontainer.json.tmpl`

**Step 1: Update the template**

The go-project template has a template-specific command: `go mod download || true`. Hardcode it in the template and chain with `{{.CertInstallCommand}}`.

Replace the entire file with:

```json
{
  "name": "{{.ContainerName}}",
  "dockerComposeFile": "docker-compose.yml",
  "service": "app",
  "workspaceFolder": "{{.WorkspaceFolder}}",
  "remoteUser": "vscode",
  "postCreateCommand": "go mod download || true && {{.CertInstallCommand}}"
}
```

**Step 2: Commit**

```bash
git add config/templates/go-project/devcontainer.json.tmpl
git commit -m "feat: go-project devcontainer.json.tmpl embeds postCreateCommand"
```
<!-- END_TASK_2 -->

<!-- END_SUBCOMPONENT_A -->

<!-- START_SUBCOMPONENT_B (tasks 3-4) -->

<!-- START_TASK_3 -->
### Task 3: Simplify generateFromTemplate and remove PostCreateCommand from TemplateData

**Files:**
- Modify: `internal/container/devcontainer.go`
- Modify: `internal/container/compose.go`

**Step 1: Add certInstallCommand constant and remove PostCreateCommand from TemplateData in compose.go**

After Phase 2, `TemplateData` has `CertInstallCommand` already. Now remove `PostCreateCommand`, add a package-level constant for the cert install command (to avoid duplication across `buildTemplateData` and `generateFromTemplate`), and simplify the chaining logic.

First, add a package-level constant near the top of `internal/container/compose.go` (after the imports, before the struct definitions):

```go
// certInstallCommand is the shell command that waits for the mitmproxy CA cert
// to become available, then installs it into the system trust store.
const certInstallCommand = "timeout=30; while [ ! -f /tmp/mitmproxy-certs/mitmproxy-ca-cert.pem ] && [ $timeout -gt 0 ]; do sleep 1; timeout=$((timeout-1)); done && sudo cp /tmp/mitmproxy-certs/mitmproxy-ca-cert.pem /usr/local/share/ca-certificates/mitmproxy-ca-cert.crt && sudo update-ca-certificates"
```

Then update the `TemplateData` struct to remove the `PostCreateCommand` field:

Replace:
```go
type TemplateData struct {
	ProjectHash        string // 12-char SHA256 of project path
	ProjectPath        string // Absolute path to project
	ProjectName        string // Base name of project directory
	WorkspaceFolder    string // /workspaces/{{.ProjectName}}
	ClaudeConfigDir    string // Host path for persistent .claude directory
	TemplateName       string // Template name (e.g., "basic")
	ContainerName      string // Container name for devcontainer.json
	PostCreateCommand  string // From template's devcontainer.json (optional)
	CertInstallCommand string // Command to wait for, copy, and trust mitmproxy CA cert
}
```

With:
```go
type TemplateData struct {
	ProjectHash        string // 12-char SHA256 of project path
	ProjectPath        string // Absolute path to project
	ProjectName        string // Base name of project directory
	WorkspaceFolder    string // /workspaces/{{.ProjectName}}
	ClaudeConfigDir    string // Host path for persistent .claude directory
	TemplateName       string // Template name (e.g., "basic")
	ContainerName      string // Container name for devcontainer.json
	CertInstallCommand string // Command to wait for, copy, and trust mitmproxy CA cert
}
```

**Step 2: Simplify buildTemplateData in compose.go**

Remove the PostCreateCommand chaining logic from `buildTemplateData()`. Use the package-level `certInstallCommand` constant added in Step 1.

Replace the `buildTemplateData` method with:

```go
// buildTemplateData constructs TemplateData from options and template.
func (g *ComposeGenerator) buildTemplateData(opts ComposeOptions, tmpl *config.Template) TemplateData {
	hash := projectHash(opts.ProjectPath)
	projectName := filepath.Base(opts.ProjectPath)

	return TemplateData{
		ProjectHash:        hash,
		ProjectPath:        opts.ProjectPath,
		ProjectName:        projectName,
		WorkspaceFolder:    fmt.Sprintf("/workspaces/%s", projectName),
		ClaudeConfigDir:    getContainerClaudeDir(opts.ProjectPath),
		TemplateName:       tmpl.Name,
		ContainerName:      opts.Name,
		CertInstallCommand: certInstallCommand,
	}
}
```

**Step 3: Simplify generateFromTemplate in devcontainer.go**

In `internal/container/devcontainer.go`, simplify `generateFromTemplate()` (lines 198-232) to no longer chain cert install in Go. The template now handles it. Use the package-level `certInstallCommand` constant from `compose.go` (same package, so directly accessible).

Replace the function with:

```go
// generateFromTemplate processes devcontainer.json.tmpl and returns the result.
func (g *DevcontainerGenerator) generateFromTemplate(tmpl *config.Template, opts CreateOptions, copyDockerfile string) (*GenerateResult, error) {
	// Build template data (same data used for docker-compose.yml.tmpl)
	hash := projectHash(opts.ProjectPath)
	projectName := filepath.Base(opts.ProjectPath)

	data := TemplateData{
		ProjectHash:        hash,
		ProjectPath:        opts.ProjectPath,
		ProjectName:        projectName,
		WorkspaceFolder:    fmt.Sprintf("/workspaces/%s", projectName),
		ClaudeConfigDir:    getContainerClaudeDir(opts.ProjectPath),
		TemplateName:       tmpl.Name,
		ContainerName:      opts.Name,
		CertInstallCommand: certInstallCommand,
	}

	// Process the template
	content, err := ProcessDevcontainerTemplate(tmpl.Path, data)
	if err != nil {
		return nil, fmt.Errorf("failed to process devcontainer.json.tmpl: %w", err)
	}

	return &GenerateResult{
		Config:               nil, // No struct when using template
		TemplatePath:         tmpl.Path,
		CopyDockerfile:       copyDockerfile,
		DevcontainerTemplate: content, // New field for template output
	}, nil
}
```

**Step 4: Delete chainPostCreateCommand if unused**

Check if `chainPostCreateCommand` (lines 141-152 in `devcontainer.go`) is used elsewhere:

Run: `grep -n 'chainPostCreateCommand' internal/container/devcontainer.go`

If only used by the old `generateFromTemplate`, delete it.

**Step 5: Verify it compiles**

Run: `go build ./...`
Expected: Build succeeds

**Step 6: Commit**

```bash
git add internal/container/compose.go internal/container/devcontainer.go
git commit -m "refactor: remove PostCreateCommand from TemplateData, simplify generateFromTemplate"
```
<!-- END_TASK_3 -->

<!-- START_TASK_4 -->
### Task 4: Update devcontainer tests

**Files:**
- Modify: `internal/container/devcontainer_test.go`

**Step 1: Update TestGenerate_TemplateMode_UsesTemplateFiles**

This test (around line 408) creates a devcontainer.json.tmpl using `{{.PostCreateCommand}}`. Update it to use `{{.CertInstallCommand}}`:

Replace the template content string in the test:
```go
	tmplContent := `{
  "name": "test-{{.ProjectName}}",
  "dockerComposeFile": "docker-compose.yml",
  "service": "app",
  "workspaceFolder": "{{.WorkspaceFolder}}",
  "remoteUser": "vscode",
  "postCreateCommand": "{{.CertInstallCommand}}"
}`
```

The assertion at line 461 still works — it checks for `"update-ca-certificates"` which will be in `CertInstallCommand`.

**Step 2: Update TestGenerate_TemplateMode_SetsCopyDockerfile**

This test (around line 466) also creates a devcontainer.json.tmpl. Update the template content to not reference `PostCreateCommand`:

```go
	tmplContent := `{"name": "test", "dockerComposeFile": "docker-compose.yml", "service": "app", "postCreateCommand": "{{.CertInstallCommand}}"}`
```

**Step 3: Delete TestChainPostCreateCommand if chainPostCreateCommand was deleted**

If `chainPostCreateCommand` was deleted in Task 3 Step 4, also delete `TestChainPostCreateCommand` (around line 230).

**Step 4: Update test templates in compose_test.go createTestTemplateDir**

The docker-compose.yml.tmpl in the test helper's template should NOT have `{{.PostCreateCommand}}` anymore. The devcontainer.json.tmpl test content already uses `{{.CertInstallCommand}}`. Verify the test helper creates a devcontainer.json.tmpl. If it doesn't already, it doesn't need one (compose tests don't test devcontainer generation).

**Step 5: Run tests**

Run: `go test ./internal/container/... -v`
Expected: All tests pass

**Step 6: Commit**

```bash
git add internal/container/devcontainer_test.go
git commit -m "test: update devcontainer tests for CertInstallCommand"
```
<!-- END_TASK_4 -->

<!-- END_SUBCOMPONENT_B -->

<!-- START_TASK_5 -->
### Task 5: Delete static devcontainer.json files

**Files:**
- Delete: `config/templates/basic/devcontainer.json`
- Delete: `config/templates/go-project/devcontainer.json`

**Step 1: Delete the files**

```bash
git rm config/templates/basic/devcontainer.json config/templates/go-project/devcontainer.json
```

**Step 2: Verify no code references these files**

Run: `grep -rn 'devcontainer.json"' internal/ --include='*.go' | grep -v '.tmpl' | grep -v '_test.go'`

After Phase 1, `LoadTemplatesFrom` uses `docker-compose.yml.tmpl` as marker. No Go code should reference the static `devcontainer.json` anymore.

**Step 3: Run tests**

Run: `go test ./internal/config/... -v`
Expected: All tests pass (template loading uses new marker)

Run: `go test ./internal/container/... -v`
Expected: All tests pass

**Step 4: Commit**

```bash
git commit -m "chore: remove static devcontainer.json files (replaced by .tmpl)"
```
<!-- END_TASK_5 -->

<!-- START_TASK_6 -->
### Task 6: Remove PostCreateCommand from Template struct

**Files:**
- Modify: `internal/config/templates.go`
- Modify: `internal/config/templates_test.go`

**Step 1: Remove PostCreateCommand from Template struct**

After Phase 1, the Template struct was stripped to `Name`, `Path`, and `PostCreateCommand`. Now that templates embed their own postCreateCommand, remove it from the struct.

In `internal/config/templates.go`, update the `Template` struct:

Replace:
```go
type Template struct {
	Name              string
	Path              string
	PostCreateCommand string
}
```

With:
```go
type Template struct {
	Name string
	Path string
}
```

**Step 2: Update loadTemplate to remove PostCreateCommand extraction**

In `internal/config/templates.go`, the `loadTemplate` function (simplified in Phase 1 to extract `PostCreateCommand` from a sidecar or `extractPostCreateCommand`) should now just return `Name` and `Path`. Simplify `loadTemplate` to:

```go
func loadTemplate(name, path string) (Template, error) {
	return Template{
		Name: name,
		Path: path,
	}, nil
}
```

Delete `extractPostCreateCommand` if it exists.

**Step 3: Update tests**

In `internal/config/templates_test.go`, remove any assertions about `PostCreateCommand` field. After Phase 1's rewrite, there should be minimal references — update any that remain.

**Step 4: Update callers**

Search for `tmpl.PostCreateCommand` references:

Run: `grep -rn 'PostCreateCommand' internal/ --include='*.go'`

The main callers were in `compose.go:buildTemplateData` and `devcontainer.go:generateFromTemplate`, both of which were updated in Task 3 to no longer read this field. Verify no remaining references.

**Step 5: Run tests**

Run: `go test ./... -v`
Expected: All tests pass

**Step 6: Commit**

```bash
git add internal/config/templates.go internal/config/templates_test.go
git commit -m "refactor: remove PostCreateCommand from Template struct"
```
<!-- END_TASK_6 -->

<!-- START_TASK_7 -->
### Task 7: Verify Phase 3 compiles and all tests pass

**Files:** None (verification only)

**Step 1: Run full build**

Run: `go build ./...`
Expected: Build succeeds with exit code 0

**Step 2: Run all tests**

Run: `make test`
Expected: All tests pass

**Step 3: Run linter**

Run: `make lint`
Expected: No linter errors

If any failures, fix them before proceeding to Phase 4.
<!-- END_TASK_7 -->

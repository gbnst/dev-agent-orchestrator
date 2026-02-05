# Template File Consolidation Implementation Plan

**Goal:** Consolidate template configuration by embedding all container orchestration settings directly into template files, eliminating separate `devcontainer.json`, `allowlist.txt`, and the Go code that parses them.

**Architecture:** Template directories become the single source of truth. `filter.py.tmpl` replaces allowlist parsing. `devcontainer.json.tmpl` becomes self-contained per template. The `Template` struct is stripped to `Name`, `Path`, and `PostCreateCommand`. All isolation config types and parsing code are removed.

**Tech Stack:** Go 1.21+, text/template, mitmproxy Python addon API

**Scope:** 5 phases from original design (phases 1-5)

**Codebase verified:** 2026-02-05

**Testing conventions:** Standard Go `testing` package only (no testify/gomock). Table-driven tests with `t.Run()`. Manual test doubles. `t.TempDir()` for filesystem tests. `make test` runs `go test ./...`. Reference: `internal/config/templates_test.go`, `internal/container/compose_test.go`. See also `internal/config/CLAUDE.md` and `internal/container/CLAUDE.md` for domain contracts.

---

## Phase 4: Clean up manager.go and manager_test.go

**Goal:** Remove the `GetEffectiveIsolation()` call and dead isolation code from `manager.go`. Rewrite the `TestComposeGenerator_GeneratesAndWritesFiles` test in `manager_test.go` to use the new template structure (no `Image`, `Build`, `Isolation` fields; no `allowlist.txt`; uses `filter.py.tmpl` instead). Verify no remaining `config.IsolationConfig` references in the container package.

**Note:** Phase 2 Task 7 already removes the `Isolation` field from `ComposeOptions` and removes the `Isolation: effectiveIsolation` line from `manager.go`. This phase handles the remaining cleanup that Phase 2 does not cover: the `effectiveIsolation` variable declaration and `GetEffectiveIsolation()` call (lines 354-355 of `manager.go`), and the `manager_test.go` test that still creates old-style templates.

<!-- START_TASK_1 -->
### Task 1: Remove GetEffectiveIsolation call from manager.go

**Files:**
- Modify: `internal/container/manager.go`

**Step 1: Remove dead isolation code**

After Phase 1 strips `GetEffectiveIsolation()` from the `Template` struct and Phase 2 Task 7 removes the `Isolation:` field assignment from the `ComposeOptions` literal, the `effectiveIsolation` variable at lines 354-355 of `manager.go` becomes dead code that won't compile.

In `internal/container/manager.go`, delete lines 348-355 (the comment and the `effectiveIsolation` variable):

Replace:
```go
	// Find template to get isolation config
	tmpl := m.generator.GetTemplate(opts.Template)
	if tmpl == nil {
		return nil, fmt.Errorf("template not found: %s", opts.Template)
	}

	// Get effective isolation config (merged with defaults)
	effectiveIsolation := tmpl.GetEffectiveIsolation()
```

With:
```go
```

That is, delete those 7 lines entirely. The template lookup is no longer needed in `CreateWithCompose()` for isolation config — `ComposeGenerator.Generate()` handles template lookup internally via `getTemplate()`.

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add internal/container/manager.go
git commit -m "refactor: remove dead GetEffectiveIsolation call from CreateWithCompose"
```
<!-- END_TASK_1 -->

<!-- START_TASK_2 -->
### Task 2: Rewrite TestComposeGenerator_GeneratesAndWritesFiles in manager_test.go

**Files:**
- Modify: `internal/container/manager_test.go`

**Step 1: Rewrite the test**

The `TestComposeGenerator_GeneratesAndWritesFiles` test (line 402) creates templates with `Image`, `Build`, and `Isolation` fields, creates an `allowlist.txt` file, and passes `Isolation: testIsolationConfig()` to `ComposeOptions`. After Phases 1-3, none of these exist. Rewrite the test to use the simplified `Template` struct and `filter.py.tmpl`.

Replace the entire `TestComposeGenerator_GeneratesAndWritesFiles` function (lines 402-553) with:

```go
func TestComposeGenerator_GeneratesAndWritesFiles(t *testing.T) {
	projectDir := t.TempDir()

	cfg := &config.Config{
		Runtime: "docker",
	}

	// Create template files in the template directory
	templateDir := t.TempDir()

	templates := []config.Template{
		{
			Name: "default",
			Path: templateDir,
		},
	}

	dockerfileContent := "FROM ubuntu:22.04\n"
	if err := os.WriteFile(filepath.Join(templateDir, "Dockerfile"), []byte(dockerfileContent), 0644); err != nil {
		t.Fatalf("Failed to write Dockerfile: %v", err)
	}

	// Create docker-compose.yml.tmpl
	composeContent := `services:
  app:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: devagent-{{.ProjectHash}}-app
    depends_on:
      proxy:
        condition: service_started
    networks:
      - isolated
    volumes:
      - {{.ProjectPath}}:{{.WorkspaceFolder}}:cached
    labels:
      devagent.managed: "true"
      devagent.project_path: "{{.ProjectPath}}"
      devagent.template: "{{.TemplateName}}"
    command: sleep infinity

  proxy:
    build:
      context: .
      dockerfile: Dockerfile.proxy
    container_name: devagent-{{.ProjectHash}}-proxy
    networks:
      - isolated
    volumes:
      - proxy-certs:/home/mitmproxy/.mitmproxy
    command: ["mitmdump", "--listen-host", "0.0.0.0", "--listen-port", "8080", "-s", "/home/mitmproxy/filter.py"]
    labels:
      devagent.managed: "true"

networks:
  isolated:
    name: devagent-{{.ProjectHash}}-net

volumes:
  proxy-certs:
`
	if err := os.WriteFile(filepath.Join(templateDir, "docker-compose.yml.tmpl"), []byte(composeContent), 0644); err != nil {
		t.Fatalf("Failed to write docker-compose.yml.tmpl: %v", err)
	}

	// Create Dockerfile.proxy
	dockerfileProxyContent := "FROM mitmproxy/mitmproxy:latest\nCOPY filter.py /home/mitmproxy/filter.py\nEXPOSE 8080\n"
	if err := os.WriteFile(filepath.Join(templateDir, "Dockerfile.proxy"), []byte(dockerfileProxyContent), 0644); err != nil {
		t.Fatalf("Failed to write Dockerfile.proxy: %v", err)
	}

	// Create filter.py.tmpl (replaces allowlist.txt)
	filterContent := `"""mitmproxy filter script - test template"""
ALLOWED_DOMAINS = [
    "api.anthropic.com",
    "github.com",
]

BLOCK_GITHUB_PR_MERGE = False
`
	if err := os.WriteFile(filepath.Join(templateDir, "filter.py.tmpl"), []byte(filterContent), 0644); err != nil {
		t.Fatalf("Failed to write filter.py.tmpl: %v", err)
	}

	mock := &mockRuntime{
		containers: []Container{
			{
				ID:          "abc123def456",
				Name:        "test-compose-container",
				ProjectPath: projectDir,
				State:       StateRunning,
			},
		},
	}

	// Create a manager with all dependencies for testing compose generation.
	// Pass nil for devCLI since we're testing file generation, not container creation.
	mgr := NewManagerWithAllDeps(cfg, templates, mock, nil)

	// Manually set the containers map to simulate devCLI.Up() success
	// This avoids needing a mock CLI - we're testing file generation
	mgr.containers["abc123def456"] = &Container{
		ID:          "abc123def456",
		Name:        "test-compose-container",
		ProjectPath: projectDir,
		State:       StateRunning,
	}

	// Test the compose file generation directly via composeGenerator
	composeOpts := ComposeOptions{
		ProjectPath: projectDir,
		Template:    "default",
		Name:        "test-compose-container",
	}

	result, err := mgr.composeGenerator.Generate(composeOpts)
	if err != nil {
		t.Fatalf("ComposeGenerator.Generate failed: %v", err)
	}

	// Verify compose files would be created correctly
	if !strings.Contains(result.ComposeYAML, "services:") {
		t.Error("ComposeYAML missing services section")
	}
	if !strings.Contains(result.ComposeYAML, "app:") {
		t.Error("ComposeYAML missing app service")
	}
	if !strings.Contains(result.ComposeYAML, "proxy:") {
		t.Error("ComposeYAML missing proxy service")
	}

	// Test file writing via WriteComposeFiles
	err = mgr.generator.WriteComposeFiles(projectDir, result)
	if err != nil {
		t.Fatalf("WriteComposeFiles failed: %v", err)
	}

	// Verify compose files exist
	composeFile := filepath.Join(projectDir, ".devcontainer", "docker-compose.yml")
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		t.Error("docker-compose.yml was not created")
	}

	dockerfileProxy := filepath.Join(projectDir, ".devcontainer", "Dockerfile.proxy")
	if _, err := os.Stat(dockerfileProxy); os.IsNotExist(err) {
		t.Error("Dockerfile.proxy was not created")
	}

	filterPy := filepath.Join(projectDir, ".devcontainer", "filter.py")
	if _, err := os.Stat(filterPy); os.IsNotExist(err) {
		t.Error("filter.py was not created")
	}
}
```

**Step 2: Remove testIsolationConfig if still present**

After Phase 2 Task 5, `testIsolationConfig()` should already be removed from `compose_test.go`. Verify it is not duplicated in `manager_test.go`:

Run: `grep -n 'testIsolationConfig' internal/container/manager_test.go`

If still referenced, remove the function and all references.

**Step 3: Check if config import is still needed**

Run: `grep -n 'config\.' internal/container/manager_test.go`

The import `"devagent/internal/config"` is needed for `config.Config` (line 405) and `config.Template` (line 408). Keep it.

**Step 4: Run tests**

Run: `go test ./internal/container/... -v`
Expected: All tests pass

**Step 5: Commit**

```bash
git add internal/container/manager_test.go
git commit -m "test: rewrite manager compose test for template-driven generation"
```
<!-- END_TASK_2 -->

<!-- START_TASK_3 -->
### Task 3: Verify no remaining config.IsolationConfig references in container package

**Files:** None (verification only)

**Step 1: Search for IsolationConfig references**

Run: `grep -rn 'IsolationConfig\|GetEffectiveIsolation\|config\.Isolation' internal/container/ --include='*.go'`

Expected: No hits. All references should have been removed across Phases 1-4.

If any references remain, remove them. The only acceptable references are to `IsolationInfo` (the runtime query type in `types.go`) and `GetIsolationInfo` / `GetContainerIsolationInfo` (the runtime query methods).

**Step 2: Run full build and tests**

Run: `go build ./...`
Expected: Build succeeds

Run: `make test`
Expected: All tests pass

**Step 3: Run linter**

Run: `make lint`
Expected: No linter errors

If any failures, fix them before proceeding to Phase 5.
<!-- END_TASK_3 -->

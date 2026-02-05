# Template File Consolidation Implementation Plan

**Goal:** Consolidate template configuration by embedding all container orchestration settings directly into template files, eliminating separate `devcontainer.json`, `allowlist.txt`, and the Go code that parses them.

**Architecture:** Template directories become the single source of truth. `filter.py.tmpl` replaces allowlist parsing. `devcontainer.json.tmpl` becomes self-contained per template. The `Template` struct is stripped to `Name`, `Path`, and `PostCreateCommand`. All isolation config types and parsing code are removed.

**Tech Stack:** Go 1.21+, text/template, mitmproxy Python addon API

**Scope:** 5 phases from original design (phases 1-5)

**Codebase verified:** 2026-02-05

**Testing conventions:** Standard Go `testing` package only (no testify/gomock). Table-driven tests with `t.Run()`. Manual test doubles. `t.TempDir()` for filesystem tests. `make test` runs `go test ./...`. Reference: `internal/config/templates_test.go`, `internal/container/compose_test.go`. See also `internal/config/CLAUDE.md` and `internal/container/CLAUDE.md` for domain contracts.

---

## Phase 1: Template loading and struct cleanup

**Goal:** Change template discovery to use `docker-compose.yml.tmpl` as the marker file. Strip `Template` struct to minimal fields. Remove isolation config parsing.

<!-- START_SUBCOMPONENT_A (tasks 1-3) -->

<!-- START_TASK_1 -->
### Task 1: Strip Template struct and simplify loadTemplate

**Files:**
- Modify: `internal/config/templates.go`

**Step 1: Replace the Template struct**

Replace the current `Template` struct (lines 47-66) with the stripped version. Remove all JSON-parsed devcontainer fields. Keep only `Name`, `Path`, and `PostCreateCommand`.

Replace:

```go
// Template represents a loaded devcontainer template with devagent extensions.
// Templates are loaded from directories containing a devcontainer.json file.
type Template struct {
	// Standard devcontainer.json fields
	Name              string                            `json:"name"`
	Image             string                            `json:"image,omitempty"`
	Build             *BuildConfig                      `json:"build,omitempty"`
	Features          map[string]map[string]interface{} `json:"features,omitempty"`
	Customizations    map[string]interface{}            `json:"customizations,omitempty"`
	PostCreateCommand string                            `json:"postCreateCommand,omitempty"`
	RemoteUser        string                            `json:"remoteUser,omitempty"`

	// Devagent-specific fields extracted from customizations.devagent
	InjectCredentials []string         `json:"-"` // Populated from customizations.devagent.injectCredentials
	DefaultAgent      string           `json:"-"` // Populated from customizations.devagent.defaultAgent
	Isolation         *IsolationConfig `json:"-"` // Populated from customizations.devagent.isolation

	// Path to the template directory (for copying additional files like Dockerfile)
	Path string `json:"-"`
}
```

With:

```go
// Template represents a loaded devcontainer template.
// Templates are discovered by scanning directories for docker-compose.yml.tmpl marker files.
// All orchestration config (capabilities, resources, network allowlists) is hardcoded
// directly in template files (docker-compose.yml.tmpl, filter.py.tmpl, devcontainer.json.tmpl).
type Template struct {
	Name              string // Template name (from directory name)
	Path              string // Absolute path to template directory
	PostCreateCommand string // From devcontainer.json.tmpl (parsed at load time)
}
```

**Step 2: Remove BuildConfig type**

Delete the `BuildConfig` struct (lines 12-16) since it's no longer used:

```go
// BuildConfig represents the build section of a devcontainer.json.
type BuildConfig struct {
	Dockerfile string `json:"dockerfile,omitempty"`
	Context    string `json:"context,omitempty"`
}
```

**Step 3: Remove all isolation config types**

Delete these types from `templates.go` (lines 18-45):

- `IsolationConfig` (lines 18-24)
- `CapConfig` (lines 26-30)
- `ResourceConfig` (lines 32-37)
- `NetworkConfig` (lines 39-45)

**Step 4: Remove isolation parsing and helper functions**

Delete these functions from `templates.go`:

- `parseIsolationConfig()` (lines 68-145)
- `getEffectiveIsolation()` (lines 147-162)
- `copyIsolationConfig()` (lines 164-202)
- `GetEffectiveIsolation()` method on Template (lines 298-303)
- `IsIsolationEnabled()` method on Template (lines 305-313)

**Step 5: Simplify loadTemplate**

Replace the current `loadTemplate` function (lines 255-296) with a simpler version that extracts `PostCreateCommand` from `devcontainer.json.tmpl` using regex (since it's a Go template file, not valid JSON for unmarshaling):

```go
// loadTemplate loads a single template from a directory.
// The dirName is used as the template name.
// PostCreateCommand is extracted from devcontainer.json.tmpl if present.
func loadTemplate(templateDir string, dirName string) (Template, error) {
	tmpl := Template{
		Name: dirName,
		Path: templateDir,
	}

	// Try to extract PostCreateCommand from devcontainer.json.tmpl
	tmplPath := filepath.Join(templateDir, "devcontainer.json.tmpl")
	data, err := os.ReadFile(tmplPath)
	if err != nil {
		// devcontainer.json.tmpl is optional for PostCreateCommand extraction
		return tmpl, nil
	}

	// Parse the template file to extract postCreateCommand value.
	// The file is a Go template, so we look for the JSON field with potential template syntax.
	tmpl.PostCreateCommand = extractPostCreateCommand(string(data))

	return tmpl, nil
}

// extractPostCreateCommand extracts the postCreateCommand value from devcontainer.json.tmpl content.
// Returns empty string if not found. Handles both plain strings and Go template expressions.
func extractPostCreateCommand(content string) string {
	// Look for "postCreateCommand": "..." pattern
	// The value may contain Go template syntax like {{.CertInstallCommand}}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, `"postCreateCommand"`) {
			continue
		}
		// Extract the value after the colon
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		value := strings.TrimSpace(parts[1])
		// Remove surrounding quotes and trailing comma
		value = strings.Trim(value, `",`)
		value = strings.TrimSpace(value)
		// Remove Go template expressions (they'll be resolved at generation time)
		// We only want the static part of the command for the Template struct
		if strings.Contains(value, "{{") {
			// If the whole value is a template expression, return empty
			if strings.HasPrefix(value, "{{") {
				return ""
			}
			// Strip template expressions and trailing connectors
			idx := strings.Index(value, "{{")
			value = strings.TrimSpace(value[:idx])
			value = strings.TrimSuffix(value, "&&")
			value = strings.TrimSpace(value)
		}
		return value
	}
	return ""
}
```

**Step 6: Add `strings` import**

Add `"strings"` to the import block in templates.go since `extractPostCreateCommand` uses it.

**Step 7: Remove `encoding/json` and `log` imports**

These are no longer needed after removing JSON unmarshaling and the log.Printf warning.

**Step 8: Sanity check — identify remaining references**

Run: `cd /Users/josh/code/dev-agent-orchestrater/.worktrees/docker-compose-orchestration && go build ./internal/config/ 2>&1`
Expected: Config package compiles. Other packages may fail (expected — addressed in Phases 2-4).

This step identifies what later phases need to fix. It is NOT a pass/fail gate.

**Step 9: Commit template struct and function changes**

```bash
git add internal/config/templates.go
git commit -m "refactor: strip Template struct to Name/Path/PostCreateCommand, remove isolation types"
```

<!-- END_TASK_1 -->

<!-- START_TASK_2 -->
### Task 2: Update LoadTemplatesFrom to use docker-compose.yml.tmpl marker

**Files:**
- Modify: `internal/config/templates.go`

**Step 1: Update LoadTemplatesFrom**

Replace the current `LoadTemplatesFrom` function (lines 220-253) to use `docker-compose.yml.tmpl` as the marker file instead of `devcontainer.json`:

```go
// LoadTemplatesFrom loads all templates from the specified directory.
// Each subdirectory containing a docker-compose.yml.tmpl file is treated as a template.
// The directory name is used as the template name.
func LoadTemplatesFrom(dir string) ([]Template, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Template{}, nil
		}
		return nil, err
	}

	var templates []Template
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		templateDir := filepath.Join(dir, entry.Name())
		markerPath := filepath.Join(templateDir, "docker-compose.yml.tmpl")
		if _, err := os.Stat(markerPath); err != nil {
			if os.IsNotExist(err) {
				continue // Not a template directory
			}
			continue // Skip on stat errors
		}

		tmpl, err := loadTemplate(templateDir, entry.Name())
		if err != nil {
			continue
		}
		templates = append(templates, tmpl)
	}

	return templates, nil
}
```

**Step 2: Run build to verify**

Run: `cd /Users/josh/code/dev-agent-orchestrater/.worktrees/docker-compose-orchestration && go build ./...`
Expected: Compilation errors in other packages that reference removed types (expected — Phases 2-4 will address those)

**Step 3: Commit**

```bash
git add internal/config/templates.go
git commit -m "refactor: use docker-compose.yml.tmpl as template marker file"
```

<!-- END_TASK_2 -->

<!-- START_TASK_3 -->
### Task 3: Rewrite templates_test.go for new loading behavior

**Files:**
- Modify: `internal/config/templates_test.go`

**Step 1: Rewrite the entire test file**

The test file needs to be completely rewritten because:
- Tests currently create `devcontainer.json` as the marker file — must change to `docker-compose.yml.tmpl`
- Tests verify `Image`, `Customizations`, `Isolation`, and other removed fields
- Isolation-specific tests (`TestLoadTemplate_IsolationConfig`, `TestGetEffectiveIsolation`, etc.) must be deleted entirely

Replace the entire file with:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTemplates_EmptyDir(t *testing.T) {
	tempDir := t.TempDir()
	templates, err := LoadTemplatesFrom(tempDir)
	if err != nil {
		t.Fatalf("LoadTemplatesFrom failed: %v", err)
	}
	if len(templates) != 0 {
		t.Errorf("Expected empty slice, got %d templates", len(templates))
	}
}

func TestLoadTemplates_SingleTemplate(t *testing.T) {
	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "basic")
	if err := os.Mkdir(templateDir, 0755); err != nil {
		t.Fatalf("Failed to create template directory: %v", err)
	}

	// Create marker file (docker-compose.yml.tmpl)
	composeContent := "services:\n  app:\n    build:\n      context: .\n"
	if err := os.WriteFile(filepath.Join(templateDir, "docker-compose.yml.tmpl"), []byte(composeContent), 0644); err != nil {
		t.Fatalf("Failed to write docker-compose.yml.tmpl: %v", err)
	}

	templates, err := LoadTemplatesFrom(tempDir)
	if err != nil {
		t.Fatalf("LoadTemplatesFrom failed: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("Expected 1 template, got %d", len(templates))
	}

	tmpl := templates[0]
	if tmpl.Name != "basic" {
		t.Errorf("Name: got %q, want %q", tmpl.Name, "basic")
	}
	if tmpl.Path != templateDir {
		t.Errorf("Path: got %q, want %q", tmpl.Path, templateDir)
	}
}

func TestLoadTemplates_WithPostCreateCommand(t *testing.T) {
	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "go-project")
	if err := os.Mkdir(templateDir, 0755); err != nil {
		t.Fatalf("Failed to create template directory: %v", err)
	}

	// Create marker file
	if err := os.WriteFile(filepath.Join(templateDir, "docker-compose.yml.tmpl"), []byte("services:\n  app:\n"), 0644); err != nil {
		t.Fatalf("Failed to write docker-compose.yml.tmpl: %v", err)
	}

	// Create devcontainer.json.tmpl with postCreateCommand
	devcontainerContent := `{
  "name": "{{.ContainerName}}",
  "dockerComposeFile": "docker-compose.yml",
  "service": "app",
  "workspaceFolder": "{{.WorkspaceFolder}}",
  "remoteUser": "vscode",
  "postCreateCommand": "go mod download || true && {{.CertInstallCommand}}"
}`
	if err := os.WriteFile(filepath.Join(templateDir, "devcontainer.json.tmpl"), []byte(devcontainerContent), 0644); err != nil {
		t.Fatalf("Failed to write devcontainer.json.tmpl: %v", err)
	}

	templates, err := LoadTemplatesFrom(tempDir)
	if err != nil {
		t.Fatalf("LoadTemplatesFrom failed: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("Expected 1 template, got %d", len(templates))
	}

	tmpl := templates[0]
	if tmpl.PostCreateCommand != "go mod download || true" {
		t.Errorf("PostCreateCommand: got %q, want %q", tmpl.PostCreateCommand, "go mod download || true")
	}
}

func TestLoadTemplates_NoPostCreateCommand(t *testing.T) {
	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "basic")
	if err := os.Mkdir(templateDir, 0755); err != nil {
		t.Fatalf("Failed to create template directory: %v", err)
	}

	// Create marker file
	if err := os.WriteFile(filepath.Join(templateDir, "docker-compose.yml.tmpl"), []byte("services:\n  app:\n"), 0644); err != nil {
		t.Fatalf("Failed to write docker-compose.yml.tmpl: %v", err)
	}

	// Create devcontainer.json.tmpl without postCreateCommand
	devcontainerContent := `{
  "name": "{{.ContainerName}}",
  "dockerComposeFile": "docker-compose.yml",
  "service": "app",
  "workspaceFolder": "{{.WorkspaceFolder}}",
  "remoteUser": "vscode"
}`
	if err := os.WriteFile(filepath.Join(templateDir, "devcontainer.json.tmpl"), []byte(devcontainerContent), 0644); err != nil {
		t.Fatalf("Failed to write devcontainer.json.tmpl: %v", err)
	}

	templates, err := LoadTemplatesFrom(tempDir)
	if err != nil {
		t.Fatalf("LoadTemplatesFrom failed: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("Expected 1 template, got %d", len(templates))
	}
	if templates[0].PostCreateCommand != "" {
		t.Errorf("PostCreateCommand: got %q, want empty string", templates[0].PostCreateCommand)
	}
}

func TestLoadTemplates_MultipleTemplates(t *testing.T) {
	tempDir := t.TempDir()

	for _, name := range []string{"basic", "go-project"} {
		dir := filepath.Join(tempDir, name)
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatalf("Failed to create %s directory: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml.tmpl"), []byte("services:\n  app:\n"), 0644); err != nil {
			t.Fatalf("Failed to write docker-compose.yml.tmpl for %s: %v", name, err)
		}
	}

	templates, err := LoadTemplatesFrom(tempDir)
	if err != nil {
		t.Fatalf("LoadTemplatesFrom failed: %v", err)
	}
	if len(templates) != 2 {
		t.Errorf("Expected 2 templates, got %d", len(templates))
	}

	names := make(map[string]bool)
	for _, tmpl := range templates {
		names[tmpl.Name] = true
	}
	if !names["basic"] || !names["go-project"] {
		t.Errorf("Expected basic and go-project templates, got: %v", names)
	}
}

func TestLoadTemplates_IgnoresFilesAndDirsWithoutMarker(t *testing.T) {
	tempDir := t.TempDir()

	// Create valid template directory (has docker-compose.yml.tmpl)
	validDir := filepath.Join(tempDir, "valid")
	if err := os.MkdirAll(validDir, 0755); err != nil {
		t.Fatalf("Failed to create valid dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(validDir, "docker-compose.yml.tmpl"), []byte("services:\n  app:\n"), 0644); err != nil {
		t.Fatalf("Failed to write docker-compose.yml.tmpl: %v", err)
	}

	// Create directory without marker file
	emptyDir := filepath.Join(tempDir, "no-marker")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatalf("Failed to create empty dir: %v", err)
	}

	// Create a file at root level (should be ignored)
	if err := os.WriteFile(filepath.Join(tempDir, "readme.txt"), []byte("ignore me"), 0644); err != nil {
		t.Fatalf("Failed to write txt file: %v", err)
	}

	templates, err := LoadTemplatesFrom(tempDir)
	if err != nil {
		t.Fatalf("LoadTemplatesFrom failed: %v", err)
	}
	if len(templates) != 1 {
		t.Errorf("Expected 1 template (ignoring files and dirs without marker), got %d", len(templates))
	}
}

func TestLoadTemplates_DirNotExists(t *testing.T) {
	templates, err := LoadTemplatesFrom("/nonexistent/path/12345")
	if err != nil {
		t.Fatalf("LoadTemplatesFrom should not error for nonexistent dir: %v", err)
	}
	if len(templates) != 0 {
		t.Errorf("Expected empty slice for nonexistent dir, got %d", len(templates))
	}
}

func TestLoadTemplates_UsesDirectoryName(t *testing.T) {
	tempDir := t.TempDir()

	templateDir := filepath.Join(tempDir, "my-template")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatalf("Failed to create template dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templateDir, "docker-compose.yml.tmpl"), []byte("services:\n  app:\n"), 0644); err != nil {
		t.Fatalf("Failed to write docker-compose.yml.tmpl: %v", err)
	}

	templates, err := LoadTemplatesFrom(tempDir)
	if err != nil {
		t.Fatalf("LoadTemplatesFrom failed: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("Expected 1 template, got %d", len(templates))
	}
	if templates[0].Name != "my-template" {
		t.Errorf("Name should be directory name: got %q, want %q", templates[0].Name, "my-template")
	}
}

func TestExtractPostCreateCommand(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "no postCreateCommand",
			content: `{"name": "test", "workspaceFolder": "/workspace"}`,
			want:    "",
		},
		{
			name:    "static postCreateCommand",
			content: `  "postCreateCommand": "go mod download || true"`,
			want:    "go mod download || true",
		},
		{
			name:    "template-only postCreateCommand",
			content: `  "postCreateCommand": "{{.CertInstallCommand}}"`,
			want:    "",
		},
		{
			name:    "mixed static and template",
			content: `  "postCreateCommand": "go mod download || true && {{.CertInstallCommand}}"`,
			want:    "go mod download || true",
		},
		{
			name: "full devcontainer.json.tmpl",
			content: `{
  "name": "{{.ContainerName}}",
  "dockerComposeFile": "docker-compose.yml",
  "service": "app",
  "workspaceFolder": "{{.WorkspaceFolder}}",
  "remoteUser": "vscode",
  "postCreateCommand": "pip install -r requirements.txt && {{.CertInstallCommand}}"
}`,
			want: "pip install -r requirements.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPostCreateCommand(tt.content)
			if got != tt.want {
				t.Errorf("extractPostCreateCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

**Step 2: Run the template tests**

Run: `cd /Users/josh/code/dev-agent-orchestrater/.worktrees/docker-compose-orchestration && go test ./internal/config/ -v -run TestLoadTemplates -count=1`
Expected: All tests pass

Run: `cd /Users/josh/code/dev-agent-orchestrater/.worktrees/docker-compose-orchestration && go test ./internal/config/ -v -run TestExtract -count=1`
Expected: All tests pass

**Step 3: Commit**

```bash
git add internal/config/templates_test.go
git commit -m "test: rewrite template tests for docker-compose.yml.tmpl marker and stripped struct"
```

<!-- END_TASK_3 -->
<!-- END_SUBCOMPONENT_A -->

<!-- START_TASK_4 -->
### Task 4: Update e2e test helpers for new Template struct

**Files:**
- Modify: `internal/e2e/helpers.go`

The E2E test helper `TestTemplates()` (line 72-78) has a fallback that references `Template.Image` which no longer exists.

**Step 1: Update the fallback in TestTemplates**

In `internal/e2e/helpers.go`, replace the fallback at lines 72-78:

```go
	// Fallback: return minimal default template for backward compatibility
	return []config.Template{
		{
			Name:  "default",
			Image: "mcr.microsoft.com/devcontainers/base:ubuntu",
		},
	}
```

With:

```go
	// Fallback: return minimal default template
	return []config.Template{
		{
			Name: "default",
		},
	}
```

**Step 2: Verify compilation**

Run: `cd /Users/josh/code/dev-agent-orchestrater/.worktrees/docker-compose-orchestration && go build ./internal/e2e/...`

Note: This requires the `e2e` build tag. Use:
Run: `cd /Users/josh/code/dev-agent-orchestrater/.worktrees/docker-compose-orchestration && go build -tags=e2e ./internal/e2e/...`
Expected: Compilation succeeds (or fails on other packages' references to removed types — that's expected)

**Step 3: Commit**

```bash
git add internal/e2e/helpers.go
git commit -m "fix: update e2e test helper for stripped Template struct"
```

<!-- END_TASK_4 -->

<!-- START_TASK_5 -->
### Task 5: Fix tmpl.Build reference in devcontainer.go

**Files:**
- Modify: `internal/container/devcontainer.go`

After removing `BuildConfig` and `Build` from the `Template` struct in Task 1, the code at `internal/container/devcontainer.go:188-195` references `tmpl.Build` which no longer exists. This must be updated.

**Step 1: Replace the tmpl.Build reference**

In `internal/container/devcontainer.go`, replace the code block (lines 188-195):

```go
	// Track Dockerfile to copy separately - we must NOT set Build
	// on the config because devcontainer CLI ignores dockerComposeFile when build is present
	var copyDockerfile string
	if tmpl.Build != nil && tmpl.Build.Dockerfile != "" {
		copyDockerfile = tmpl.Build.Dockerfile
	}

	return g.generateFromTemplate(tmpl, opts, copyDockerfile)
```

With:

```go
	// All templates have a Dockerfile in their template directory.
	// It is copied to .devcontainer/ separately because devcontainer CLI
	// ignores dockerComposeFile when build is present in devcontainer.json.
	copyDockerfile := "Dockerfile"

	return g.generateFromTemplate(tmpl, opts, copyDockerfile)
```

**Step 2: Verify compilation**

Run: `go build ./internal/container/...`
Expected: May still fail on other removed types (IsolationConfig, etc.) — that's expected for Phase 1. But the `tmpl.Build` error is resolved.

**Step 3: Commit**

```bash
git add internal/container/devcontainer.go
git commit -m "fix: replace tmpl.Build reference with hardcoded Dockerfile name"
```
<!-- END_TASK_5 -->

<!-- START_TASK_6 -->
### Task 6: Update config CLAUDE.md for new template structure

**Files:**
- Modify: `internal/config/CLAUDE.md`

**Step 1: Update CLAUDE.md to reflect new template loading**

Replace the contents of `internal/config/CLAUDE.md` to document the new template structure. (Update the freshness date with today's date.)

```markdown
# Config Domain

Last verified: 2026-02-05

## Purpose
Loads and validates application configuration (config.yaml) and devcontainer templates.

## Contracts
- **Exposes**: `Config`, `Template`, `LoadTemplates`, `LoadTemplatesFrom`
- **Guarantees**: Templates loaded from `~/.config/devagent/templates/` (XDG-compliant). Templates discovered by presence of `docker-compose.yml.tmpl` marker file. Template struct contains only `Name`, `Path`, `PostCreateCommand`.
- **Expects**: Valid YAML in config files. Template directories contain `docker-compose.yml.tmpl`.

## Dependencies
- **Uses**: os, strings (stdlib only)
- **Used by**: container.Manager, container.ComposeGenerator, container.DevcontainerGenerator, TUI
- **Boundary**: Configuration loading only; no container operations

## Key Decisions
- Template discovery uses `docker-compose.yml.tmpl` as marker file (not `devcontainer.json`)
- `PostCreateCommand` extracted from `devcontainer.json.tmpl` at load time (static portion only)
- All orchestration config (caps, resources, network allowlists) is hardcoded in template files
- No `IsolationConfig` types — isolation is entirely template-driven

## Invariants
- Template.Name always equals the directory name
- Template.Path is the absolute path to the template directory
- Templates without `docker-compose.yml.tmpl` are ignored during discovery

## Key Files
- `config.go` - Config struct, loading, credential management
- `templates.go` - Template loading, discovery, PostCreateCommand extraction
```

**Step 2: Commit**

```bash
git add internal/config/CLAUDE.md
git commit -m "docs: update config CLAUDE.md for template-driven architecture"
```

<!-- END_TASK_6 -->

<!-- START_TASK_7 -->
### Task 7: Verify Phase 1 compiles (config package only)

**Step 1: Run config package tests**

Run: `cd /Users/josh/code/dev-agent-orchestrater/.worktrees/docker-compose-orchestration && go test ./internal/config/ -v -count=1`
Expected: All config tests pass

**Step 2: Run lint on config package**

Run: `cd /Users/josh/code/dev-agent-orchestrater/.worktrees/docker-compose-orchestration && go vet ./internal/config/`
Expected: No vet issues

**Note:** The full `go build ./...` will fail because other packages (container, e2e) still reference removed types like `IsolationConfig`, `Template.Isolation`, etc. Those are addressed in Phases 2-4. The `tmpl.Build` reference in `devcontainer.go` was already fixed in Task 5. Phase 1 is done when the config package itself compiles and passes tests.

<!-- END_TASK_7 -->

package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseWorktreeList(t *testing.T) {
	output := `worktree /home/user/project
HEAD abc123def456
branch refs/heads/main

worktree /home/user/project/.worktrees/feature-x
HEAD def456abc123
branch refs/heads/feature/new-model

worktree /home/user/project/.worktrees/fix-bug
HEAD 789abc123def
branch refs/heads/fix/bug-123

`
	worktrees := parseWorktreeList(output)

	if len(worktrees) != 2 {
		t.Fatalf("expected 2 worktrees (skipping main), got %d", len(worktrees))
	}

	if worktrees[0].Path != "/home/user/project/.worktrees/feature-x" {
		t.Errorf("expected feature-x path, got %s", worktrees[0].Path)
	}
	if worktrees[0].Branch != "feature/new-model" {
		t.Errorf("expected feature/new-model branch, got %s", worktrees[0].Branch)
	}
	if worktrees[0].Name != "feature-x" {
		t.Errorf("expected feature-x name, got %s", worktrees[0].Name)
	}

	if worktrees[1].Branch != "fix/bug-123" {
		t.Errorf("expected fix/bug-123 branch, got %s", worktrees[1].Branch)
	}
}

func TestParseWorktreeList_MainOnly(t *testing.T) {
	output := `worktree /home/user/project
HEAD abc123def456
branch refs/heads/main

`
	worktrees := parseWorktreeList(output)
	if len(worktrees) != 0 {
		t.Fatalf("expected 0 worktrees for main-only, got %d", len(worktrees))
	}
}

func TestParseWorktreeList_Empty(t *testing.T) {
	worktrees := parseWorktreeList("")
	if len(worktrees) != 0 {
		t.Fatalf("expected 0 worktrees for empty input, got %d", len(worktrees))
	}
}

func TestComposeHasManagedLabel(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	// Compose file with managed label
	content := []byte(`services:
  app:
    image: mcr.microsoft.com/devcontainers/go:1.21
    labels:
      devagent.managed: "true"
      devagent.project_path: /workspaces/myproject
  proxy:
    image: mitmproxy/mitmproxy:latest
    labels:
      devagent.sidecar_type: proxy
`)
	if err := os.WriteFile(composePath, content, 0644); err != nil {
		t.Fatal(err)
	}

	if !composeHasManagedLabel(composePath) {
		t.Error("expected compose file to have managed label")
	}
}

func TestComposeHasManagedLabel_NoLabel(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	content := []byte(`services:
  app:
    image: mcr.microsoft.com/devcontainers/go:1.21
`)
	if err := os.WriteFile(composePath, content, 0644); err != nil {
		t.Fatal(err)
	}

	if composeHasManagedLabel(composePath) {
		t.Error("expected compose file without managed label to return false")
	}
}

func TestComposeHasManagedLabel_MissingFile(t *testing.T) {
	if composeHasManagedLabel("/nonexistent/docker-compose.yml") {
		t.Error("expected missing file to return false")
	}
}

func TestScanAll_FindsProjects(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid project
	projectDir := filepath.Join(tmpDir, "myproject")
	devcontainerDir := filepath.Join(projectDir, ".devcontainer")
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatal(err)
	}
	composeContent := []byte(`services:
  app:
    labels:
      devagent.managed: "true"
`)
	if err := os.WriteFile(filepath.Join(devcontainerDir, "docker-compose.yml"), composeContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Create a non-project directory
	otherDir := filepath.Join(tmpDir, "other")
	if err := os.MkdirAll(otherDir, 0755); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner()
	projects := scanner.ScanAll([]string{tmpDir})

	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Name != "myproject" {
		t.Errorf("expected myproject, got %s", projects[0].Name)
	}
}

func TestScanAll_SkipsNonDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a regular file
	if err := os.WriteFile(filepath.Join(tmpDir, "notadir"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner()
	projects := scanner.ScanAll([]string{tmpDir})

	if len(projects) != 0 {
		t.Fatalf("expected 0 projects, got %d", len(projects))
	}
}

func TestScanAll_HandlesMissingDir(t *testing.T) {
	scanner := NewScanner()
	projects := scanner.ScanAll([]string{"/nonexistent/path"})

	if len(projects) != 0 {
		t.Fatalf("expected 0 projects for missing dir, got %d", len(projects))
	}
}

func TestScanAll_DetectsMakefile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create project with Makefile
	projectDir := filepath.Join(tmpDir, "with-makefile")
	devcontainerDir := filepath.Join(projectDir, ".devcontainer")
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatal(err)
	}
	composeContent := []byte(`services:
  app:
    labels:
      devagent.managed: "true"
`)
	if err := os.WriteFile(filepath.Join(devcontainerDir, "docker-compose.yml"), composeContent, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "Makefile"), []byte(".PHONY: worktree-prep\nworktree-prep:\n\t@echo done\n"), 0644); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner()
	projects := scanner.ScanAll([]string{tmpDir})

	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if !projects[0].HasMakefile {
		t.Error("expected HasMakefile to be true")
	}
}

func TestScanAll_DeduplicatesSymlinks(t *testing.T) {
	tmpDir := t.TempDir()

	// Create project
	projectDir := filepath.Join(tmpDir, "real-project")
	devcontainerDir := filepath.Join(projectDir, ".devcontainer")
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatal(err)
	}
	composeContent := []byte(`services:
  app:
    labels:
      devagent.managed: "true"
`)
	if err := os.WriteFile(filepath.Join(devcontainerDir, "docker-compose.yml"), composeContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Create a second scan dir with a symlink to the same project
	scanDir2 := filepath.Join(tmpDir, "scan2")
	if err := os.MkdirAll(scanDir2, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(projectDir, filepath.Join(scanDir2, "linked-project")); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner()
	projects := scanner.ScanAll([]string{tmpDir, scanDir2})

	if len(projects) != 1 {
		t.Fatalf("expected 1 project (deduplicated), got %d", len(projects))
	}
}

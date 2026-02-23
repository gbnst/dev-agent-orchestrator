package worktree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteComposeOverride(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "myproject")
	wtDir := filepath.Join(projectPath, ".worktrees", "feature-x")
	devcontainerDir := filepath.Join(wtDir, ".devcontainer")

	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write an original compose file to verify it's untouched
	originalContent := `services:
  app:
    image: mcr.microsoft.com/devcontainers/go:1.21
`
	composePath := filepath.Join(devcontainerDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Write override
	err := WriteComposeOverride(projectPath, wtDir, "feature-x")
	if err != nil {
		t.Fatal(err)
	}

	// Verify original compose file is untouched
	data, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != originalContent {
		t.Error("original docker-compose.yml was modified")
	}

	// Verify override file exists and has correct content
	overridePath := filepath.Join(devcontainerDir, "docker-compose.worktree.yml")
	overrideData, err := os.ReadFile(overridePath)
	if err != nil {
		t.Fatal(err)
	}
	override := string(overrideData)

	// Should have sanitized compose name
	if !strings.Contains(override, "name: myproject-feature-x") {
		t.Errorf("expected compose name 'myproject-feature-x', got:\n%s", override)
	}

	// Should have host-path .git volume mount
	expectedMount := projectPath + "/.git:" + projectPath + "/.git:cached"
	if !strings.Contains(override, expectedMount) {
		t.Errorf("expected volume mount %q in:\n%s", expectedMount, override)
	}
}

func TestWriteComposeOverride_SanitizesName(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "My Project")
	wtDir := filepath.Join(projectPath, ".worktrees", "Feature_Branch")
	devcontainerDir := filepath.Join(wtDir, ".devcontainer")

	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatal(err)
	}

	err := WriteComposeOverride(projectPath, wtDir, "Feature_Branch")
	if err != nil {
		t.Fatal(err)
	}

	overridePath := filepath.Join(devcontainerDir, "docker-compose.worktree.yml")
	data, err := os.ReadFile(overridePath)
	if err != nil {
		t.Fatal(err)
	}

	// Name should be sanitized: lowercase, underscores→hyphens, spaces→hyphens
	if !strings.Contains(string(data), "name: my-project-feature-branch") {
		t.Errorf("expected sanitized name, got:\n%s", string(data))
	}
}


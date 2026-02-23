package worktree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPatchComposeForWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "myproject")
	wtDir := filepath.Join(projectPath, ".worktrees", "feature-x")
	devcontainerDir := filepath.Join(wtDir, ".devcontainer")

	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a compose file similar to what devagent templates produce
	composeContent := `services:
  app:
    image: mcr.microsoft.com/devcontainers/go:1.21
    volumes:
      - ..:/workspaces/myproject:cached
    labels:
      devagent.managed: "true"
      devagent.project_path: /workspaces/myproject
  proxy:
    image: mitmproxy/mitmproxy:latest
    labels:
      devagent.sidecar_type: proxy
`
	composePath := filepath.Join(devcontainerDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Patch
	err := PatchComposeForWorktree(projectPath, wtDir, "feature-x")
	if err != nil {
		t.Fatal(err)
	}

	// Read back and verify
	data, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Should have the top-level name
	if !strings.Contains(content, "myproject-feature-x") {
		t.Error("expected compose name 'myproject-feature-x' in output")
	}

	// Should have host-path .git volume mount
	if !strings.Contains(content, ".git:cached") {
		t.Error("expected .git:cached volume mount")
	}

	// Parse to verify structure
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	root := doc.Content[0]

	// Verify name is set
	nameNode := findMapValue(root, "name")
	if nameNode == nil {
		t.Fatal("expected 'name' key in compose")
	}
	if nameNode.Value != "myproject-feature-x" {
		t.Errorf("expected name 'myproject-feature-x', got %q", nameNode.Value)
	}
}

func TestPatchComposeForWorktree_NoServices(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "myproject")
	wtDir := filepath.Join(projectPath, ".worktrees", "feature-x")
	devcontainerDir := filepath.Join(wtDir, ".devcontainer")

	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatal(err)
	}

	composeContent := `version: "3.8"
`
	composePath := filepath.Join(devcontainerDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
		t.Fatal(err)
	}

	err := PatchComposeForWorktree(projectPath, wtDir, "feature-x")
	if err == nil {
		t.Fatal("expected error for compose without services")
	}
}

func TestFindAppService(t *testing.T) {
	composeYAML := `services:
  app:
    labels:
      devagent.managed: "true"
      devagent.project_path: /workspaces/test
  proxy:
    labels:
      devagent.sidecar_type: proxy
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(composeYAML), &doc); err != nil {
		t.Fatal(err)
	}
	root := doc.Content[0]
	servicesNode := findMapValue(root, "services")

	appNode := findAppService(servicesNode)
	if appNode == nil {
		t.Fatal("expected to find app service")
	}

	// Verify it found the app service, not the proxy
	labelsNode := findMapValue(appNode, "labels")
	if labelsNode == nil {
		t.Fatal("expected labels on found service")
	}

	// Check that it has project_path but not sidecar_type
	hasProjectPath := false
	hasSidecarType := false
	for i := 0; i < len(labelsNode.Content)-1; i += 2 {
		if labelsNode.Content[i].Value == "devagent.project_path" {
			hasProjectPath = true
		}
		if labelsNode.Content[i].Value == "devagent.sidecar_type" {
			hasSidecarType = true
		}
	}
	if !hasProjectPath {
		t.Error("expected found service to have devagent.project_path")
	}
	if hasSidecarType {
		t.Error("expected found service to NOT have devagent.sidecar_type")
	}
}

func TestBuildVolumeMounts(t *testing.T) {
	mounts := buildVolumeMounts("/home/user/myproject")

	if len(mounts) != 1 {
		t.Fatalf("expected 1 mount, got %d", len(mounts))
	}

	// Host-path .git mount: root .git at its host path so worktree gitdir resolves
	expected := "/home/user/myproject/.git:/home/user/myproject/.git:cached"
	if mounts[0] != expected {
		t.Errorf("mount[0] = %q, want %q", mounts[0], expected)
	}
}

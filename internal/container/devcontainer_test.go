package container

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain sets up the test environment, including mocking claude setup-token
func TestMain(m *testing.M) {
	// Mock claude setup-token to prevent it from opening browser auth flows
	claudeSetupTokenFunc = func() (string, error) {
		return "", errors.New("claude CLI not available in tests")
	}
	os.Exit(m.Run())
}

func TestReadWorkspaceFolder_Default(t *testing.T) {
	// No devcontainer.json - should return default
	result := ReadWorkspaceFolder("/nonexistent/path")
	if result != "/workspaces" {
		t.Errorf("ReadWorkspaceFolder() = %q, want %q", result, "/workspaces")
	}
}

func TestReadWorkspaceFolder_EmptyPath(t *testing.T) {
	result := ReadWorkspaceFolder("")
	if result != "/workspaces" {
		t.Errorf("ReadWorkspaceFolder() = %q, want %q", result, "/workspaces")
	}
}

func TestReadWorkspaceFolder_CustomFolder(t *testing.T) {
	tmpDir := t.TempDir()
	devcontainerDir := filepath.Join(tmpDir, ".devcontainer")
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatalf("Failed to create .devcontainer dir: %v", err)
	}

	// Write devcontainer.json with custom workspaceFolder
	dcJSON := `{"name": "test", "workspaceFolder": "/home/vscode/project"}`
	if err := os.WriteFile(filepath.Join(devcontainerDir, "devcontainer.json"), []byte(dcJSON), 0644); err != nil {
		t.Fatalf("Failed to write devcontainer.json: %v", err)
	}

	result := ReadWorkspaceFolder(tmpDir)
	if result != "/home/vscode/project" {
		t.Errorf("ReadWorkspaceFolder() = %q, want %q", result, "/home/vscode/project")
	}
}

func TestReadWorkspaceFolder_NoWorkspaceFolderField(t *testing.T) {
	tmpDir := t.TempDir()
	devcontainerDir := filepath.Join(tmpDir, ".devcontainer")
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatalf("Failed to create .devcontainer dir: %v", err)
	}

	// Write devcontainer.json without workspaceFolder
	dcJSON := `{"name": "test", "image": "ubuntu:22.04"}`
	if err := os.WriteFile(filepath.Join(devcontainerDir, "devcontainer.json"), []byte(dcJSON), 0644); err != nil {
		t.Fatalf("Failed to write devcontainer.json: %v", err)
	}

	result := ReadWorkspaceFolder(tmpDir)
	if result != "/workspaces" {
		t.Errorf("ReadWorkspaceFolder() = %q, want %q", result, "/workspaces")
	}
}

func TestEnsureClaudeToken_EmptyPath(t *testing.T) {
	gotPath, gotToken := ensureClaudeToken("")
	if gotPath != "" || gotToken != "" {
		t.Errorf("ensureClaudeToken(\"\") = (%q, %q), want (\"\", \"\")", gotPath, gotToken)
	}
}

func TestEnsureClaudeToken_ExistingToken(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, ".devagent-claude-token")
	expectedToken := "test-oauth-token-12345"
	if err := os.WriteFile(tokenPath, []byte(expectedToken), 0600); err != nil {
		t.Fatalf("Failed to write token: %v", err)
	}

	gotPath, gotToken := ensureClaudeToken(tokenPath)

	if gotPath != tokenPath {
		t.Errorf("ensureClaudeToken() path = %q, want %q", gotPath, tokenPath)
	}
	if gotToken != expectedToken {
		t.Errorf("ensureClaudeToken() token = %q, want %q", gotToken, expectedToken)
	}
}

func TestEnsureClaudeToken_TokenWithWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, ".devagent-claude-token")
	if err := os.WriteFile(tokenPath, []byte("test-token\n"), 0600); err != nil {
		t.Fatalf("Failed to write token: %v", err)
	}

	gotPath, gotToken := ensureClaudeToken(tokenPath)

	if gotPath != tokenPath {
		t.Errorf("ensureClaudeToken() path = %q, want %q", gotPath, tokenPath)
	}
	if gotToken != "test-token" {
		t.Errorf("ensureClaudeToken() token = %q, want %q", gotToken, "test-token")
	}
}

func TestCopyTemplateDir_ProcessesTemplates(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create source directory with .tmpl file and regular file
	if err := os.WriteFile(filepath.Join(srcDir, "config.json.tmpl"), []byte(`{"name": "{{.ProjectName}}"}`), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "regular.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	data := TemplateData{
		ProjectName: "myproject",
	}

	if err := copyTemplateDir(srcDir, dstDir, data); err != nil {
		t.Fatalf("copyTemplateDir failed: %v", err)
	}

	// Verify .tmpl was processed and removed extension
	jsonPath := filepath.Join(dstDir, "config.json")
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Error("config.json was not created from .tmpl")
	}

	// Verify .tmpl file itself doesn't exist
	tmplPath := filepath.Join(dstDir, "config.json.tmpl")
	if _, err := os.Stat(tmplPath); !os.IsNotExist(err) {
		t.Error("config.json.tmpl should not exist")
	}

	// Verify regular file was copied
	txtPath := filepath.Join(dstDir, "regular.txt")
	if _, err := os.Stat(txtPath); os.IsNotExist(err) {
		t.Error("regular.txt was not copied")
	}

	// Verify content
	content, _ := os.ReadFile(jsonPath)
	if !strings.Contains(string(content), `"name": "myproject"`) {
		t.Error("Template substitution did not work")
	}
}

func TestEnsureGitHubToken_EmptyPath(t *testing.T) {
	gotPath, gotToken := ensureGitHubToken("")
	if gotPath != "" || gotToken != "" {
		t.Errorf("ensureGitHubToken(\"\") = (%q, %q), want (\"\", \"\")", gotPath, gotToken)
	}
}

func TestEnsureGitHubToken_ExistingToken(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token")
	expectedToken := "ghp_testtoken12345"
	if err := os.WriteFile(tokenPath, []byte(expectedToken), 0600); err != nil {
		t.Fatalf("Failed to write token: %v", err)
	}

	gotPath, gotToken := ensureGitHubToken(tokenPath)

	if gotPath != tokenPath {
		t.Errorf("ensureGitHubToken() path = %q, want %q", gotPath, tokenPath)
	}
	if gotToken != expectedToken {
		t.Errorf("ensureGitHubToken() token = %q, want %q", gotToken, expectedToken)
	}
}

func TestEnsureGitHubToken_MissingToken(t *testing.T) {
	gotPath, gotToken := ensureGitHubToken("/nonexistent/path/token")

	if gotPath != "" {
		t.Errorf("ensureGitHubToken() path = %q, want empty", gotPath)
	}
	if gotToken != "" {
		t.Errorf("ensureGitHubToken() token = %q, want empty", gotToken)
	}
}

func TestEnsureGitHubToken_TokenWithWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token")
	if err := os.WriteFile(tokenPath, []byte("ghp_testtoken\n"), 0600); err != nil {
		t.Fatalf("Failed to write token: %v", err)
	}

	gotPath, gotToken := ensureGitHubToken(tokenPath)

	if gotPath != tokenPath {
		t.Errorf("ensureGitHubToken() path = %q, want %q", gotPath, tokenPath)
	}
	if gotToken != "ghp_testtoken" {
		t.Errorf("ensureGitHubToken() token = %q, want %q", gotToken, "ghp_testtoken")
	}
}

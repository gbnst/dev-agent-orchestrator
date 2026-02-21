package container

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"devagent/internal/config"
)

// TestMain sets up the test environment, including mocking claude setup-token
func TestMain(m *testing.M) {
	// Mock claude setup-token to prevent it from opening browser auth flows
	claudeSetupTokenFunc = func() (string, error) {
		return "", errors.New("claude CLI not available in tests")
	}
	os.Exit(m.Run())
}

func TestGenerate_TemplateNotFound(t *testing.T) {
	cfg := &config.Config{}
	templates := []config.Template{}
	g := NewDevcontainerGenerator(cfg, templates)

	_, err := g.Generate(CreateOptions{Template: "unknown"})
	if err == nil {
		t.Error("Expected error for unknown template")
	}
}

func TestWriteToProject_CreatesDirectory(t *testing.T) {
	tempDir := t.TempDir()
	projectPath := filepath.Join(tempDir, "myproject")
	templateDir := filepath.Join(tempDir, "template")

	// Create project directory
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Create template directory with .devcontainer structure
	if err := os.MkdirAll(filepath.Join(templateDir, ".devcontainer"), 0755); err != nil {
		t.Fatalf("Failed to create template dir: %v", err)
	}

	// Create a minimal devcontainer.json.tmpl
	templateContent := `{"name": "{{.ProjectName}}", "image": "ubuntu:22.04"}`
	if err := os.WriteFile(filepath.Join(templateDir, ".devcontainer", "devcontainer.json.tmpl"), []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	g := NewDevcontainerGenerator(nil, nil)
	result := &GenerateResult{
		TemplatePath: templateDir,
	}
	data := TemplateData{
		ProjectPath: projectPath,
		ProjectName: "test-project",
	}

	err := g.WriteToProject(projectPath, result, data)
	if err != nil {
		t.Fatalf("WriteToProject failed: %v", err)
	}

	// Check .devcontainer directory was created
	devcontainerDir := filepath.Join(projectPath, ".devcontainer")
	info, err := os.Stat(devcontainerDir)
	if err != nil {
		t.Fatalf(".devcontainer dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error(".devcontainer is not a directory")
	}

	// Check devcontainer.json exists (processed from .tmpl)
	jsonPath := filepath.Join(devcontainerDir, "devcontainer.json")
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("devcontainer.json not created: %v", err)
	}
}

func TestWriteToProject_ValidJSON(t *testing.T) {
	tempDir := t.TempDir()
	projectPath := filepath.Join(tempDir, "myproject")
	templateDir := filepath.Join(tempDir, "template")

	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Create template directory with .devcontainer structure
	if err := os.MkdirAll(filepath.Join(templateDir, ".devcontainer"), 0755); err != nil {
		t.Fatalf("Failed to create template dir: %v", err)
	}

	// Create a devcontainer.json.tmpl with template substitution
	templateContent := `{
  "name": "test-{{.ProjectName}}",
  "image": "ubuntu:22.04",
  "containerEnv": {
    "FOO": "bar"
  }
}`
	if err := os.WriteFile(filepath.Join(templateDir, ".devcontainer", "devcontainer.json.tmpl"), []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	g := NewDevcontainerGenerator(nil, nil)
	result := &GenerateResult{
		TemplatePath: templateDir,
	}
	data := TemplateData{
		ProjectPath: projectPath,
		ProjectName: "container",
	}

	err := g.WriteToProject(projectPath, result, data)
	if err != nil {
		t.Fatalf("WriteToProject failed: %v", err)
	}

	// Read back and verify
	jsonPath := filepath.Join(projectPath, ".devcontainer", "devcontainer.json")
	readData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("Failed to read devcontainer.json: %v", err)
	}

	var readBack DevcontainerJSON
	if err := json.Unmarshal(readData, &readBack); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	if readBack.Name != "test-container" {
		t.Errorf("Name: got %q, want %q", readBack.Name, "test-container")
	}
	if readBack.Image != "ubuntu:22.04" {
		t.Errorf("Image: got %q", readBack.Image)
	}
	if readBack.ContainerEnv["FOO"] != "bar" {
		t.Errorf("ContainerEnv[FOO]: got %q", readBack.ContainerEnv["FOO"])
	}
}

func TestWriteToProject_EmptyTemplate(t *testing.T) {
	tempDir := t.TempDir()
	projectPath := filepath.Join(tempDir, "myproject")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	g := NewDevcontainerGenerator(nil, nil)
	result := &GenerateResult{
		TemplatePath: "",
	}
	data := TemplateData{}

	err := g.WriteToProject(projectPath, result, data)
	if err == nil {
		t.Fatal("Expected error when TemplatePath is empty")
	}

	if !strings.Contains(err.Error(), "no template path specified") {
		t.Errorf("Error message: got %q, want substring %q", err.Error(), "no template path specified")
	}
}

func TestDevcontainerCLI_Up_CallsCorrectCommand(t *testing.T) {
	var capturedName string
	var capturedArgs []string

	mockExec := func(ctx context.Context, name string, args ...string) (string, error) {
		capturedName = name
		capturedArgs = args
		return `{"containerId":"abc123"}`, nil
	}

	cli := NewDevcontainerCLIWithExecutor(mockExec)
	containerID, err := cli.Up(context.Background(), "/home/user/project")
	if err != nil {
		t.Fatalf("Up failed: %v", err)
	}

	if capturedName != "devcontainer" {
		t.Errorf("Expected devcontainer, got %q", capturedName)
	}

	// Check args contain expected values
	hasUp := false
	hasWorkspace := false
	for i, arg := range capturedArgs {
		if arg == "up" {
			hasUp = true
		}
		if arg == "--workspace-folder" && i+1 < len(capturedArgs) && capturedArgs[i+1] == "/home/user/project" {
			hasWorkspace = true
		}
	}

	if !hasUp {
		t.Error("Missing 'up' subcommand")
	}
	if !hasWorkspace {
		t.Error("Missing --workspace-folder argument")
	}
	// --container-name should NOT be passed (not a valid devcontainer CLI flag)
	for _, arg := range capturedArgs {
		if arg == "--container-name" {
			t.Error("--container-name should not be passed to devcontainer CLI")
		}
	}
	if containerID != "abc123" {
		t.Errorf("containerID: got %q, want %q", containerID, "abc123")
	}
}

func TestDevcontainerCLI_Up_WithDockerPath(t *testing.T) {
	var capturedArgs []string

	mockExec := func(ctx context.Context, name string, args ...string) (string, error) {
		capturedArgs = args
		return `{"containerId":"abc123"}`, nil
	}

	cli := NewDevcontainerCLIWithExecutorAndRuntime(mockExec, "podman")
	_, err := cli.Up(context.Background(), "/home/user/project")
	if err != nil {
		t.Fatalf("Up failed: %v", err)
	}

	// Check --docker-path is present with correct value
	hasDockerPath := false
	for i, arg := range capturedArgs {
		if arg == "--docker-path" && i+1 < len(capturedArgs) && capturedArgs[i+1] == "podman" {
			hasDockerPath = true
		}
	}

	if !hasDockerPath {
		t.Errorf("Missing --docker-path podman, got args: %v", capturedArgs)
	}
}

func TestDevcontainerCLI_Up_WithoutDockerPath(t *testing.T) {
	var capturedArgs []string

	mockExec := func(ctx context.Context, name string, args ...string) (string, error) {
		capturedArgs = args
		return `{"containerId":"abc123"}`, nil
	}

	cli := NewDevcontainerCLIWithExecutor(mockExec)
	_, err := cli.Up(context.Background(), "/home/user/project")
	if err != nil {
		t.Fatalf("Up failed: %v", err)
	}

	// Check --docker-path is NOT present
	for _, arg := range capturedArgs {
		if arg == "--docker-path" {
			t.Errorf("--docker-path should not be present when runtime not set, got args: %v", capturedArgs)
		}
	}
}

func TestNewDevcontainerCLIWithRuntime(t *testing.T) {
	cli := NewDevcontainerCLIWithRuntime("docker")
	if cli.dockerPath != "docker" {
		t.Errorf("dockerPath: got %q, want %q", cli.dockerPath, "docker")
	}
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

func TestGenerate_TemplateMode_UsesTemplateFiles(t *testing.T) {
	// Generate() now just returns TemplatePath for file copying.
	// Set up a minimal template directory.
	tmpDir := t.TempDir()
	devcontainerDir := filepath.Join(tmpDir, ".devcontainer")
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatalf("Failed to create devcontainer dir: %v", err)
	}

	tmplContent := `{
  "name": "test-{{.ProjectName}}",
  "dockerComposeFile": "docker-compose.yml",
  "service": "app",
  "workspaceFolder": "{{.WorkspaceFolder}}",
  "remoteUser": "vscode",
  "postCreateCommand": "bash {{.WorkspaceFolder}}/.devcontainer/post-create.sh"
}`
	if err := os.WriteFile(filepath.Join(devcontainerDir, "devcontainer.json.tmpl"), []byte(tmplContent), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	cfg := &config.Config{}
	templates := []config.Template{
		{
			Name: "basic",
			Path: tmpDir,
		},
	}

	gen := NewDevcontainerGenerator(cfg, templates)

	opts := CreateOptions{
		ProjectPath: "/home/user/myproject",
		Template:    "basic",
		Name:        "test-container",
	}

	result, err := gen.Generate(opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// TemplatePath should point to the template directory
	if result.TemplatePath != tmpDir {
		t.Errorf("Expected TemplatePath=%q, got %q", tmpDir, result.TemplatePath)
	}
}

func TestGenerate_ReturnsTemplatePath(t *testing.T) {
	tmpDir := t.TempDir()
	devcontainerDir := filepath.Join(tmpDir, ".devcontainer")
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatalf("Failed to create devcontainer dir: %v", err)
	}

	tmplContent := `{"name": "test", "dockerComposeFile": "docker-compose.yml", "service": "app", "postCreateCommand": "bash {{.WorkspaceFolder}}/.devcontainer/post-create.sh"}`
	if err := os.WriteFile(filepath.Join(devcontainerDir, "devcontainer.json.tmpl"), []byte(tmplContent), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	cfg := &config.Config{}
	templates := []config.Template{
		{
			Name: "with-dockerfile",
			Path: tmpDir,
		},
	}

	gen := NewDevcontainerGenerator(cfg, templates)

	opts := CreateOptions{
		ProjectPath: "/home/user/myproject",
		Template:    "with-dockerfile",
		Name:        "test-container",
	}

	result, err := gen.Generate(opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// TemplatePath should be set for WriteToProject to find the source
	if result.TemplatePath != tmpDir {
		t.Errorf("Expected TemplatePath=%q, got %q", tmpDir, result.TemplatePath)
	}
}

func TestWriteAll_WritesTemplateFiles(t *testing.T) {
	projectDir := t.TempDir()
	templateDir := filepath.Join(projectDir, "template")
	devcontainerDir := filepath.Join(templateDir, ".devcontainer")

	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatalf("Failed to create template dir: %v", err)
	}

	// Create minimal template files
	tmplContent := `{"name": "test-{{.ProjectName}}", "dockerComposeFile": "docker-compose.yml", "service": "app"}`
	if err := os.WriteFile(filepath.Join(devcontainerDir, "devcontainer.json.tmpl"), []byte(tmplContent), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	composeContent := "services:\n  app:\n    image: {{.ProxyImage}}"
	if err := os.WriteFile(filepath.Join(devcontainerDir, "docker-compose.yml.tmpl"), []byte(composeContent), 0644); err != nil {
		t.Fatalf("Failed to write compose template: %v", err)
	}

	cfg := &config.Config{}
	gen := NewDevcontainerGenerator(cfg, nil)

	devResult := &GenerateResult{
		TemplatePath: templateDir,
	}

	composeResult := &ComposeResult{
		TemplateData: TemplateData{
			ProjectPath: projectDir,
			ProjectName: "test",
			ProxyImage:  "mitmproxy/mitmproxy:latest",
		},
	}

	err := gen.WriteAll(projectDir, devResult, composeResult)
	if err != nil {
		t.Fatalf("WriteAll failed: %v", err)
	}

	// Verify devcontainer.json was created from .tmpl
	devcontainerPath := filepath.Join(projectDir, ".devcontainer", "devcontainer.json")
	if _, err := os.Stat(devcontainerPath); os.IsNotExist(err) {
		t.Error("devcontainer.json was not created")
	}

	// Verify compose files were created from .tmpl
	composePath := filepath.Join(projectDir, ".devcontainer", "docker-compose.yml")
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		t.Error("docker-compose.yml was not created")
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

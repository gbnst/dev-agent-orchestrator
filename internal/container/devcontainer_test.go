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

	// Project dir doesn't exist yet - should be created
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	g := NewDevcontainerGenerator(nil, nil)
	result := &GenerateResult{
		DevcontainerTemplate: `{"name": "test", "image": "ubuntu:22.04"}`,
	}

	err := g.WriteToProject(projectPath, result)
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

	// Check devcontainer.json exists
	jsonPath := filepath.Join(devcontainerDir, "devcontainer.json")
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("devcontainer.json not created: %v", err)
	}
}

func TestWriteToProject_ValidJSON(t *testing.T) {
	tempDir := t.TempDir()
	projectPath := filepath.Join(tempDir, "myproject")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	g := NewDevcontainerGenerator(nil, nil)
	templateContent := `{
  "name": "test-container",
  "image": "ubuntu:22.04",
  "containerEnv": {
    "FOO": "bar"
  }
}`
	result := &GenerateResult{
		DevcontainerTemplate: templateContent,
	}

	err := g.WriteToProject(projectPath, result)
	if err != nil {
		t.Fatalf("WriteToProject failed: %v", err)
	}

	// Read back and verify
	jsonPath := filepath.Join(projectPath, ".devcontainer", "devcontainer.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("Failed to read devcontainer.json: %v", err)
	}

	var readBack DevcontainerJSON
	if err := json.Unmarshal(data, &readBack); err != nil {
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
		DevcontainerTemplate: "",
	}

	err := g.WriteToProject(projectPath, result)
	if err == nil {
		t.Fatal("Expected error when DevcontainerTemplate is empty")
	}

	if !strings.Contains(err.Error(), "no devcontainer template content generated") {
		t.Errorf("Error message: got %q, want substring %q", err.Error(), "no devcontainer template content generated")
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

func TestGetClaudeConfigDir_Default(t *testing.T) {
	// Clear XDG_CONFIG_HOME to test default behavior
	t.Setenv("XDG_CONFIG_HOME", "")

	dir := getClaudeConfigDir()

	// Should return ~/.claude
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".claude")
	if dir != expected {
		t.Errorf("getClaudeConfigDir() = %q, want %q", dir, expected)
	}
}

func TestGetClaudeConfigDir_XDGOverride(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")

	dir := getClaudeConfigDir()

	expected := "/custom/config/claude"
	if dir != expected {
		t.Errorf("getClaudeConfigDir() = %q, want %q", dir, expected)
	}
}

func TestEnsureClaudeToken_ExistingToken(t *testing.T) {
	// Create a temp directory with a token file
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create claude dir and token file
	claudeDir := filepath.Join(tmpDir, "claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create claude dir: %v", err)
	}

	tokenPath := filepath.Join(claudeDir, ".devagent-claude-token")
	expectedToken := "test-oauth-token-12345"
	if err := os.WriteFile(tokenPath, []byte(expectedToken), 0600); err != nil {
		t.Fatalf("Failed to write token: %v", err)
	}

	// Call ensureClaudeToken - should read existing token
	gotPath, gotToken := ensureClaudeToken()

	if gotPath != tokenPath {
		t.Errorf("ensureClaudeToken() path = %q, want %q", gotPath, tokenPath)
	}
	if gotToken != expectedToken {
		t.Errorf("ensureClaudeToken() token = %q, want %q", gotToken, expectedToken)
	}
}

func TestEnsureClaudeToken_TokenWithWhitespace(t *testing.T) {
	// Test that token is trimmed of whitespace
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	claudeDir := filepath.Join(tmpDir, "claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create claude dir: %v", err)
	}

	tokenPath := filepath.Join(claudeDir, ".devagent-claude-token")
	// Write token with trailing newline
	if err := os.WriteFile(tokenPath, []byte("test-token\n"), 0600); err != nil {
		t.Fatalf("Failed to write token: %v", err)
	}

	gotPath, gotToken := ensureClaudeToken()

	if gotPath != tokenPath {
		t.Errorf("ensureClaudeToken() path = %q, want %q", gotPath, tokenPath)
	}
	// Token should be trimmed
	if gotToken != "test-token" {
		t.Errorf("ensureClaudeToken() token = %q, want %q", gotToken, "test-token")
	}
}


func TestGenerate_TemplateMode_UsesTemplateFiles(t *testing.T) {
	// Generate() now requires actual template files on disk.
	// Set up a minimal template directory with devcontainer.json.tmpl.
	tmpDir := t.TempDir()
	tmplContent := `{
  "name": "test-{{.ProjectName}}",
  "dockerComposeFile": "docker-compose.yml",
  "service": "app",
  "workspaceFolder": "{{.WorkspaceFolder}}",
  "remoteUser": "vscode",
  "postCreateCommand": "{{.CertInstallCommand}}"
}`
	if err := os.WriteFile(filepath.Join(tmpDir, "devcontainer.json.tmpl"), []byte(tmplContent), 0644); err != nil {
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

	// Config should be nil when using templates
	if result.Config != nil {
		t.Error("Config should be nil when using template mode")
	}

	// DevcontainerTemplate should contain processed template output
	if result.DevcontainerTemplate == "" {
		t.Fatal("DevcontainerTemplate should not be empty")
	}

	if !strings.Contains(result.DevcontainerTemplate, `"name": "test-myproject"`) {
		t.Errorf("Template should substitute ProjectName, got: %s", result.DevcontainerTemplate)
	}
	if !strings.Contains(result.DevcontainerTemplate, `"workspaceFolder": "/workspaces/myproject"`) {
		t.Errorf("Template should substitute WorkspaceFolder, got: %s", result.DevcontainerTemplate)
	}
	if !strings.Contains(result.DevcontainerTemplate, "update-ca-certificates") {
		t.Error("CertInstallCommand should include cert installation")
	}
}

func TestGenerate_TemplateMode_SetsCopyDockerfile(t *testing.T) {
	tmpDir := t.TempDir()
	tmplContent := `{"name": "test", "dockerComposeFile": "docker-compose.yml", "service": "app", "postCreateCommand": "{{.CertInstallCommand}}"}`
	if err := os.WriteFile(filepath.Join(tmpDir, "devcontainer.json.tmpl"), []byte(tmplContent), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}
	// Also create a Dockerfile so copy would work
	if err := os.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte("FROM ubuntu"), 0644); err != nil {
		t.Fatalf("Failed to write Dockerfile: %v", err)
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

	// CopyDockerfile should be set to track which Dockerfile to copy
	if result.CopyDockerfile != "Dockerfile" {
		t.Errorf("Expected CopyDockerfile='Dockerfile', got %q", result.CopyDockerfile)
	}

	// TemplatePath should be set for WriteToProject to find the source
	if result.TemplatePath != tmpDir {
		t.Errorf("Expected TemplatePath=%q, got %q", tmpDir, result.TemplatePath)
	}
}

func TestWriteComposeFiles_CreatesAllFiles(t *testing.T) {
	projectDir := t.TempDir()
	cfg := &config.Config{}
	gen := NewDevcontainerGenerator(cfg, nil)

	composeResult := &ComposeResult{
		ComposeYAML: `services:
  app:
    image: ubuntu
  proxy:
    image: mitmproxy/mitmproxy
`,
	}

	err := gen.WriteComposeFiles(projectDir, composeResult)
	if err != nil {
		t.Fatalf("WriteComposeFiles failed: %v", err)
	}

	// Verify docker-compose.yml
	composePath := filepath.Join(projectDir, ".devcontainer", "docker-compose.yml")
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		t.Error("docker-compose.yml was not created")
	}
	composeContent, _ := os.ReadFile(composePath)
	if !strings.Contains(string(composeContent), "services:") {
		t.Error("docker-compose.yml content is incorrect")
	}

	// Verify Dockerfile.proxy is NOT created (now copied by WriteToProject from template)
	dockerfilePath := filepath.Join(projectDir, ".devcontainer", "Dockerfile.proxy")
	if _, err := os.Stat(dockerfilePath); !os.IsNotExist(err) {
		t.Error("Dockerfile.proxy should not be created by WriteComposeFiles")
	}

	// Verify filter.py is NOT created (now copied by WriteToProject from template)
	filterPath := filepath.Join(projectDir, ".devcontainer", "filter.py")
	if _, err := os.Stat(filterPath); !os.IsNotExist(err) {
		t.Error("filter.py should not be created by WriteComposeFiles")
	}
}

func TestWriteComposeFiles_CreatesDevcontainerDir(t *testing.T) {
	projectDir := t.TempDir()
	cfg := &config.Config{}
	gen := NewDevcontainerGenerator(cfg, nil)

	composeResult := &ComposeResult{
		ComposeYAML: "services: {}",
	}

	err := gen.WriteComposeFiles(projectDir, composeResult)
	if err != nil {
		t.Fatalf("WriteComposeFiles failed: %v", err)
	}

	devcontainerDir := filepath.Join(projectDir, ".devcontainer")
	info, err := os.Stat(devcontainerDir)
	if os.IsNotExist(err) {
		t.Error(".devcontainer directory was not created")
	}
	if !info.IsDir() {
		t.Error(".devcontainer is not a directory")
	}
}

func TestWriteAll_WritesDevcontainerAndComposeFiles(t *testing.T) {
	projectDir := t.TempDir()
	cfg := &config.Config{}
	templates := []config.Template{
		{Name: "basic"},
	}
	gen := NewDevcontainerGenerator(cfg, templates)

	devcontainerResult := &GenerateResult{
		DevcontainerTemplate: `{
  "name": "test",
  "dockerComposeFile": "docker-compose.yml",
  "service": "app",
  "workspaceFolder": "/workspaces/test"
}`,
	}

	composeResult := &ComposeResult{
		ComposeYAML: "services:\n  app:\n    image: ubuntu",
	}

	err := gen.WriteAll(projectDir, devcontainerResult, composeResult)
	if err != nil {
		t.Fatalf("WriteAll failed: %v", err)
	}

	// Verify devcontainer.json
	devcontainerPath := filepath.Join(projectDir, ".devcontainer", "devcontainer.json")
	if _, err := os.Stat(devcontainerPath); os.IsNotExist(err) {
		t.Error("devcontainer.json was not created")
	}

	// Verify compose files
	composePath := filepath.Join(projectDir, ".devcontainer", "docker-compose.yml")
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		t.Error("docker-compose.yml was not created")
	}
}

func TestWriteAll_WithoutComposeResult(t *testing.T) {
	projectDir := t.TempDir()
	cfg := &config.Config{}
	gen := NewDevcontainerGenerator(cfg, nil)

	devcontainerResult := &GenerateResult{
		DevcontainerTemplate: `{
  "name": "test",
  "image": "ubuntu"
}`,
	}

	// composeResult is nil - should only write devcontainer.json
	err := gen.WriteAll(projectDir, devcontainerResult, nil)
	if err != nil {
		t.Fatalf("WriteAll failed: %v", err)
	}

	// Verify devcontainer.json exists
	devcontainerPath := filepath.Join(projectDir, ".devcontainer", "devcontainer.json")
	if _, err := os.Stat(devcontainerPath); os.IsNotExist(err) {
		t.Error("devcontainer.json was not created")
	}

	// Verify compose files do NOT exist
	composePath := filepath.Join(projectDir, ".devcontainer", "docker-compose.yml")
	if _, err := os.Stat(composePath); !os.IsNotExist(err) {
		t.Error("docker-compose.yml should not be created when composeResult is nil")
	}
}

func TestWriteComposeFiles_FileContentsMatch(t *testing.T) {
	projectDir := t.TempDir()
	cfg := &config.Config{}
	gen := NewDevcontainerGenerator(cfg, nil)

	expectedCompose := "services:\n  app:\n    build: .\n"

	composeResult := &ComposeResult{
		ComposeYAML: expectedCompose,
	}

	err := gen.WriteComposeFiles(projectDir, composeResult)
	if err != nil {
		t.Fatalf("WriteComposeFiles failed: %v", err)
	}

	// Verify exact content match
	composeContent, _ := os.ReadFile(filepath.Join(projectDir, ".devcontainer", "docker-compose.yml"))
	if string(composeContent) != expectedCompose {
		t.Errorf("docker-compose.yml content mismatch:\ngot: %q\nwant: %q", composeContent, expectedCompose)
	}
}

func TestWriteComposeFiles_CreatesProxyLogsDirectory(t *testing.T) {
	projectDir := t.TempDir()
	cfg := &config.Config{}
	gen := NewDevcontainerGenerator(cfg, nil)

	composeResult := &ComposeResult{
		ComposeYAML: "services: {}",
	}

	err := gen.WriteComposeFiles(projectDir, composeResult)
	if err != nil {
		t.Fatalf("WriteComposeFiles failed: %v", err)
	}

	// Verify proxy/logs directory was created
	proxyLogsDir := filepath.Join(projectDir, ".devcontainer", "proxy", "logs")
	info, err := os.Stat(proxyLogsDir)
	if os.IsNotExist(err) {
		t.Error("proxy/logs directory was not created")
	}
	if err == nil && !info.IsDir() {
		t.Error("proxy/logs is not a directory")
	}
}

func TestWriteComposeFiles_DoesNotCreateGitignore(t *testing.T) {
	projectDir := t.TempDir()
	cfg := &config.Config{}
	gen := NewDevcontainerGenerator(cfg, nil)

	composeResult := &ComposeResult{
		ComposeYAML: "services: {}",
	}

	err := gen.WriteComposeFiles(projectDir, composeResult)
	if err != nil {
		t.Fatalf("WriteComposeFiles failed: %v", err)
	}

	// Verify .gitignore is NOT created by WriteComposeFiles (now handled by WriteToProject)
	gitignorePath := filepath.Join(projectDir, ".devcontainer", ".gitignore")
	if _, err := os.Stat(gitignorePath); !os.IsNotExist(err) {
		t.Error(".gitignore should not be created by WriteComposeFiles")
	}
}

func TestEnsureGitHubToken_ExistingToken(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create github dir and token file
	githubDir := filepath.Join(tmpDir, "github")
	if err := os.MkdirAll(githubDir, 0755); err != nil {
		t.Fatalf("Failed to create github dir: %v", err)
	}

	tokenPath := filepath.Join(githubDir, "token")
	expectedToken := "ghp_testtoken12345"
	if err := os.WriteFile(tokenPath, []byte(expectedToken), 0600); err != nil {
		t.Fatalf("Failed to write token: %v", err)
	}

	gotPath, gotToken := ensureGitHubToken()

	if gotPath != tokenPath {
		t.Errorf("ensureGitHubToken() path = %q, want %q", gotPath, tokenPath)
	}
	if gotToken != expectedToken {
		t.Errorf("ensureGitHubToken() token = %q, want %q", gotToken, expectedToken)
	}
}

func TestEnsureGitHubToken_MissingToken(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Don't create the token file
	gotPath, gotToken := ensureGitHubToken()

	if gotPath != "" {
		t.Errorf("ensureGitHubToken() path = %q, want empty", gotPath)
	}
	if gotToken != "" {
		t.Errorf("ensureGitHubToken() token = %q, want empty", gotToken)
	}
}

func TestEnsureGitHubToken_TokenWithWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	githubDir := filepath.Join(tmpDir, "github")
	if err := os.MkdirAll(githubDir, 0755); err != nil {
		t.Fatalf("Failed to create github dir: %v", err)
	}

	tokenPath := filepath.Join(githubDir, "token")
	if err := os.WriteFile(tokenPath, []byte("ghp_testtoken\n"), 0600); err != nil {
		t.Fatalf("Failed to write token: %v", err)
	}

	gotPath, gotToken := ensureGitHubToken()

	if gotPath != tokenPath {
		t.Errorf("ensureGitHubToken() path = %q, want %q", gotPath, tokenPath)
	}
	if gotToken != "ghp_testtoken" {
		t.Errorf("ensureGitHubToken() token = %q, want %q", gotToken, "ghp_testtoken")
	}
}


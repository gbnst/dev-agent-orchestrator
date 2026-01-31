package container

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"devagent/internal/config"
)

func TestGenerate_TemplateNotFound(t *testing.T) {
	cfg := &config.Config{}
	templates := []config.Template{}
	g := NewDevcontainerGenerator(cfg, templates)

	_, err := g.Generate(CreateOptions{Template: "unknown"})
	if err == nil {
		t.Error("Expected error for unknown template")
	}
}


func TestGenerate_BasicTemplate(t *testing.T) {
	cfg := &config.Config{}
	templates := []config.Template{
		{
			Name:  "python",
			Image: "mcr.microsoft.com/devcontainers/python:3.11",
		},
	}
	g := NewDevcontainerGenerator(cfg, templates)

	result, err := g.Generate(CreateOptions{
		Template:    "python",
		ProjectPath: "/home/user/project",
		Name:        "my-container",
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if result.Config.Image != "mcr.microsoft.com/devcontainers/python:3.11" {
		t.Errorf("Image: got %q", result.Config.Image)
	}
	if result.Config.Name != "my-container" {
		t.Errorf("Name: got %q, want %q", result.Config.Name, "my-container")
	}
}

func TestGenerate_InjectsCredentials(t *testing.T) {
	cfg := &config.Config{
		Credentials: map[string]string{
			"OPENAI_API_KEY": "TEST_OPENAI_KEY",
		},
	}
	templates := []config.Template{
		{
			Name:              "default",
			Image:             "ubuntu:22.04",
			InjectCredentials: []string{"OPENAI_API_KEY"},
		},
	}

	// Set the env var
	t.Setenv("TEST_OPENAI_KEY", "sk-secret-key")

	g := NewDevcontainerGenerator(cfg, templates)
	result, err := g.Generate(CreateOptions{Template: "default"})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if result.Config.ContainerEnv["OPENAI_API_KEY"] != "sk-secret-key" {
		t.Errorf("ContainerEnv[OPENAI_API_KEY]: got %q, want %q", result.Config.ContainerEnv["OPENAI_API_KEY"], "sk-secret-key")
	}
}

func TestGenerate_SkipsMissingCredentials(t *testing.T) {
	cfg := &config.Config{
		Credentials: map[string]string{
			"MISSING_KEY": "UNSET_ENV_VAR_12345",
		},
	}
	templates := []config.Template{
		{
			Name:              "default",
			Image:             "ubuntu:22.04",
			InjectCredentials: []string{"MISSING_KEY"},
		},
	}

	g := NewDevcontainerGenerator(cfg, templates)
	result, err := g.Generate(CreateOptions{Template: "default"})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Should not include credentials that aren't set
	if _, ok := result.Config.ContainerEnv["MISSING_KEY"]; ok {
		t.Error("Should not include missing credentials")
	}
}

func TestGenerate_InjectsAgentOTELEnv(t *testing.T) {
	cfg := &config.Config{
		Agents: map[string]config.AgentConfig{
			"claude-code": {
				DisplayName: "Claude Code",
				OTELEnv: map[string]string{
					"OTEL_SERVICE_NAME": "claude-code",
					"OTEL_ENDPOINT":     "http://host.docker.internal:4317",
				},
			},
		},
	}
	templates := []config.Template{
		{
			Name:         "default",
			Image:        "ubuntu:22.04",
			DefaultAgent: "claude-code",
		},
	}

	g := NewDevcontainerGenerator(cfg, templates)
	result, err := g.Generate(CreateOptions{Template: "default"})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if result.Config.ContainerEnv["OTEL_SERVICE_NAME"] != "claude-code" {
		t.Errorf("ContainerEnv[OTEL_SERVICE_NAME]: got %q", result.Config.ContainerEnv["OTEL_SERVICE_NAME"])
	}
	if result.Config.ContainerEnv["OTEL_ENDPOINT"] != "http://host.docker.internal:4317" {
		t.Errorf("ContainerEnv[OTEL_ENDPOINT]: got %q", result.Config.ContainerEnv["OTEL_ENDPOINT"])
	}
}

func TestGenerate_AddsDevagentLabels(t *testing.T) {
	cfg := &config.Config{}
	templates := []config.Template{
		{
			Name:  "default",
			Image: "ubuntu:22.04",
		},
	}

	g := NewDevcontainerGenerator(cfg, templates)
	result, err := g.Generate(CreateOptions{
		Template:    "default",
		ProjectPath: "/home/user/project",
		Agent:       "claude-code",
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check RunArgs contains label flags
	hasManaged := false
	hasProjectPath := false
	hasTemplate := false
	hasAgent := false

	for i, arg := range result.Config.RunArgs {
		if arg == "--label" && i+1 < len(result.Config.RunArgs) {
			label := result.Config.RunArgs[i+1]
			switch {
			case label == "devagent.managed=true":
				hasManaged = true
			case label == "devagent.project_path=/home/user/project":
				hasProjectPath = true
			case label == "devagent.template=default":
				hasTemplate = true
			case label == "devagent.agent=claude-code":
				hasAgent = true
			}
		}
	}

	if !hasManaged {
		t.Error("Missing devagent.managed label")
	}
	if !hasProjectPath {
		t.Error("Missing devagent.project_path label")
	}
	if !hasTemplate {
		t.Error("Missing devagent.template label")
	}
	if !hasAgent {
		t.Error("Missing devagent.agent label")
	}
}

func TestGenerate_CopiesFeatures(t *testing.T) {
	cfg := &config.Config{}
	templates := []config.Template{
		{
			Name:  "default",
			Image: "ubuntu:22.04",
			Features: map[string]map[string]interface{}{
				"ghcr.io/devcontainers/features/python:1": {
					"version": "3.11",
				},
			},
			PostCreateCommand: "pip install -r requirements.txt",
		},
	}

	g := NewDevcontainerGenerator(cfg, templates)
	result, err := g.Generate(CreateOptions{Template: "default"})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if result.Config.Features == nil {
		t.Fatal("Features should not be nil")
	}
	feature, ok := result.Config.Features["ghcr.io/devcontainers/features/python:1"]
	if !ok {
		t.Error("Expected python feature")
	}
	if feature["version"] != "3.11" {
		t.Errorf("Feature version: got %v", feature["version"])
	}
	if result.Config.PostCreateCommand != "pip install -r requirements.txt" {
		t.Errorf("PostCreateCommand: got %q", result.Config.PostCreateCommand)
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
		Config: &DevcontainerJSON{
			Name:  "test",
			Image: "ubuntu:22.04",
		},
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
	result := &GenerateResult{
		Config: &DevcontainerJSON{
			Name:  "test-container",
			Image: "ubuntu:22.04",
			ContainerEnv: map[string]string{
				"FOO": "bar",
			},
		},
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

func TestGenerate_AddsDockerNameToRunArgs(t *testing.T) {
	cfg := &config.Config{}
	templates := []config.Template{
		{
			Name:  "default",
			Image: "ubuntu:22.04",
		},
	}

	g := NewDevcontainerGenerator(cfg, templates)
	result, err := g.Generate(CreateOptions{
		Template: "default",
		Name:     "my-container",
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check RunArgs contains --name my-container
	hasName := false
	for i, arg := range result.Config.RunArgs {
		if arg == "--name" && i+1 < len(result.Config.RunArgs) && result.Config.RunArgs[i+1] == "my-container" {
			hasName = true
		}
	}
	if !hasName {
		t.Errorf("Expected --name my-container in RunArgs, got: %v", result.Config.RunArgs)
	}
}

func TestGenerate_OmitsDockerNameWhenEmpty(t *testing.T) {
	cfg := &config.Config{}
	templates := []config.Template{
		{
			Name:  "default",
			Image: "ubuntu:22.04",
		},
	}

	g := NewDevcontainerGenerator(cfg, templates)
	result, err := g.Generate(CreateOptions{
		Template: "default",
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check RunArgs does NOT contain --name
	for _, arg := range result.Config.RunArgs {
		if arg == "--name" {
			t.Errorf("--name should not be in RunArgs when Name is empty, got: %v", result.Config.RunArgs)
		}
	}
}

func TestNewDevcontainerCLIWithRuntime(t *testing.T) {
	cli := NewDevcontainerCLIWithRuntime("docker")
	if cli.dockerPath != "docker" {
		t.Errorf("dockerPath: got %q, want %q", cli.dockerPath, "docker")
	}
}

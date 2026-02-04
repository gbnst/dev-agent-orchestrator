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

func TestGenerate_AddsClaudeMount(t *testing.T) {
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
		ProjectPath: "/home/user/myproject",
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Should have at least one mount for .claude directory
	if len(result.Config.Mounts) < 1 {
		t.Fatalf("Expected at least 1 mount, got %d", len(result.Config.Mounts))
	}

	// Find the .claude mount
	var claudeMount string
	for _, mount := range result.Config.Mounts {
		if strings.Contains(mount, "target=/home/vscode/.claude") {
			claudeMount = mount
			break
		}
	}
	if claudeMount == "" {
		t.Fatalf("Expected .claude mount not found in: %v", result.Config.Mounts)
	}
	// Mount should be a bind type
	if !strings.Contains(claudeMount, "type=bind") {
		t.Errorf("Mount should be type=bind, got: %s", claudeMount)
	}
	// Source should be in devagent data directory
	if !strings.Contains(claudeMount, "source=") {
		t.Errorf("Mount should have source, got: %s", claudeMount)
	}
}

func TestGenerate_ClaudeMountUsesRemoteUser(t *testing.T) {
	cfg := &config.Config{}
	templates := []config.Template{
		{
			Name:       "default",
			Image:      "ubuntu:22.04",
			RemoteUser: "developer",
		},
	}

	g := NewDevcontainerGenerator(cfg, templates)
	result, err := g.Generate(CreateOptions{
		Template:    "default",
		ProjectPath: "/home/user/myproject",
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Find the .claude mount with custom remoteUser
	var claudeMount string
	for _, mount := range result.Config.Mounts {
		if strings.Contains(mount, "target=/home/developer/.claude") {
			claudeMount = mount
			break
		}
	}
	if claudeMount == "" {
		t.Fatalf("Expected .claude mount for developer user not found in: %v", result.Config.Mounts)
	}
}

func TestGenerate_NoClaudeMountWithoutProjectPath(t *testing.T) {
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
		// No ProjectPath
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Should have no .claude mount when no project path (may have token mount)
	for _, mount := range result.Config.Mounts {
		if strings.Contains(mount, "target=/home/vscode/.claude") {
			t.Errorf("Should not have .claude mount without project path, got: %s", mount)
		}
	}
}

func TestBuildIsolationRunArgs(t *testing.T) {
	tests := []struct {
		name string
		iso  *config.IsolationConfig
		want []string
	}{
		{
			name: "nil config returns nil",
			iso:  nil,
			want: nil,
		},
		{
			name: "empty config returns nil",
			iso:  &config.IsolationConfig{},
			want: nil,
		},
		{
			name: "capability drops",
			iso: &config.IsolationConfig{
				Caps: &config.CapConfig{
					Drop: []string{"NET_RAW", "SYS_ADMIN"},
				},
			},
			want: []string{"--cap-drop", "NET_RAW", "--cap-drop", "SYS_ADMIN"},
		},
		{
			name: "memory limit",
			iso: &config.IsolationConfig{
				Resources: &config.ResourceConfig{
					Memory: "2g",
				},
			},
			want: []string{"--memory", "2g"},
		},
		{
			name: "cpu limit",
			iso: &config.IsolationConfig{
				Resources: &config.ResourceConfig{
					CPUs: "1.5",
				},
			},
			want: []string{"--cpus", "1.5"},
		},
		{
			name: "pids limit",
			iso: &config.IsolationConfig{
				Resources: &config.ResourceConfig{
					PidsLimit: 256,
				},
			},
			want: []string{"--pids-limit", "256"},
		},
		{
			name: "full isolation config",
			iso: &config.IsolationConfig{
				Caps: &config.CapConfig{
					Drop: []string{"NET_RAW"},
				},
				Resources: &config.ResourceConfig{
					Memory:    "4g",
					CPUs:      "2",
					PidsLimit: 512,
				},
			},
			want: []string{
				"--cap-drop", "NET_RAW",
				"--memory", "4g",
				"--cpus", "2",
				"--pids-limit", "512",
			},
		},
		{
			name: "zero pids limit omitted",
			iso: &config.IsolationConfig{
				Resources: &config.ResourceConfig{
					Memory:    "2g",
					PidsLimit: 0,
				},
			},
			want: []string{"--memory", "2g"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildIsolationRunArgs(tt.iso)
			if len(got) != len(tt.want) {
				t.Errorf("buildIsolationRunArgs() len = %d, want %d\ngot: %v\nwant: %v",
					len(got), len(tt.want), got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("buildIsolationRunArgs()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGenerate_AddsIsolationRunArgs(t *testing.T) {
	enabled := true
	templates := []config.Template{
		{
			Name:  "isolated-template",
			Image: "ubuntu:22.04",
			Isolation: &config.IsolationConfig{
				Enabled: &enabled,
				Caps: &config.CapConfig{
					Drop: []string{"NET_RAW", "SYS_ADMIN"},
					Add:  []string{"NET_BIND_SERVICE"},
				},
				Resources: &config.ResourceConfig{
					Memory:    "4g",
					CPUs:      "2",
					PidsLimit: 512,
				},
			},
		},
	}

	cfg := &config.Config{
		Runtime: "docker",
	}

	gen := NewDevcontainerGenerator(cfg, templates)
	result, err := gen.Generate(CreateOptions{
		Template: "isolated-template",
		Name:     "test-container",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify isolation runArgs are present
	runArgs := result.Config.RunArgs
	wantArgs := map[string]string{
		"--cap-drop":   "NET_RAW",
		"--memory":     "4g",
		"--cpus":       "2",
		"--pids-limit": "512",
	}

	for flag, expectedValue := range wantArgs {
		found := false
		for i, arg := range runArgs {
			if arg == flag && i+1 < len(runArgs) {
				if flag == "--cap-drop" {
					// --cap-drop can appear multiple times, check if any matches
					if runArgs[i+1] == expectedValue || runArgs[i+1] == "SYS_ADMIN" {
						found = true
						break
					}
				} else if runArgs[i+1] == expectedValue {
					found = true
					break
				}
			}
		}
		if !found {
			t.Errorf("runArgs missing %s %s, got: %v", flag, expectedValue, runArgs)
		}
	}

	// Verify capAdd native field
	if len(result.Config.CapAdd) != 1 {
		t.Errorf("CapAdd len = %d, want 1", len(result.Config.CapAdd))
	} else if result.Config.CapAdd[0] != "NET_BIND_SERVICE" {
		t.Errorf("CapAdd[0] = %q, want %q", result.Config.CapAdd[0], "NET_BIND_SERVICE")
	}
}

func TestGenerate_NoIsolationRunArgsWhenNil(t *testing.T) {
	// UPDATED: Phase 8 now applies default isolation even when template.Isolation is nil
	// This test is now superseded by TestGenerate_TemplateWithoutIsolationGetsDefaults
	// Keeping this test but changing behavior to verify defaults are applied
	enabled := false
	templates := []config.Template{
		{
			Name:      "basic-template",
			Image:     "ubuntu:22.04",
			Isolation: &config.IsolationConfig{Enabled: &enabled}, // Explicitly disable isolation
		},
	}

	cfg := &config.Config{
		Runtime: "docker",
	}

	gen := NewDevcontainerGenerator(cfg, templates)
	result, err := gen.Generate(CreateOptions{
		Template: "basic-template",
		Name:     "test-container",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify no isolation-specific runArgs when isolation is explicitly disabled
	for _, arg := range result.Config.RunArgs {
		if arg == "--cap-drop" || arg == "--memory" || arg == "--cpus" || arg == "--pids-limit" {
			t.Errorf("runArgs should not contain isolation args when Isolation is disabled, got: %v", result.Config.RunArgs)
			break
		}
	}

	// Verify CapAdd is empty
	if len(result.Config.CapAdd) != 0 {
		t.Errorf("CapAdd should be empty, got: %v", result.Config.CapAdd)
	}
}

func TestChainPostCreateCommand(t *testing.T) {
	tests := []struct {
		name       string
		existing   string
		additional string
		want       string
	}{
		{
			name:       "both empty",
			existing:   "",
			additional: "",
			want:       "",
		},
		{
			name:       "existing only",
			existing:   "pip install -r requirements.txt",
			additional: "",
			want:       "pip install -r requirements.txt",
		},
		{
			name:       "additional only",
			existing:   "",
			additional: "update-ca-certificates",
			want:       "update-ca-certificates",
		},
		{
			name:       "both present",
			existing:   "pip install -r requirements.txt",
			additional: "update-ca-certificates",
			want:       "pip install -r requirements.txt && update-ca-certificates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chainPostCreateCommand(tt.existing, tt.additional)
			if got != tt.want {
				t.Errorf("chainPostCreateCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerate_AddsCertMountWithProxy(t *testing.T) {
	// Redirect data directory to temp for this test to avoid polluting real user data
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	templates := []config.Template{
		{
			Name:  "test-template",
			Image: "ubuntu:22.04",
		},
	}

	cfg := &config.Config{
		Runtime: "docker",
	}

	certDir := t.TempDir()

	gen := NewDevcontainerGenerator(cfg, templates)
	result, err := gen.Generate(CreateOptions{
		Template:    "test-template",
		Name:        "test-container",
		ProjectPath: "/test/project",
		Proxy: &ProxyConfig{
			CertDir:     certDir,
			ProxyHost:   "proxy",
			ProxyPort:   "8080",
			NetworkName: "test-network",
		},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify cert mount was added
	foundCertMount := false
	for _, mount := range result.Config.Mounts {
		if strings.Contains(mount, "mitmproxy-certs") && strings.Contains(mount, certDir) {
			foundCertMount = true
			break
		}
	}
	if !foundCertMount {
		t.Errorf("cert mount not found in Mounts: %v", result.Config.Mounts)
	}

	// Verify postCreateCommand includes cert installation
	if !strings.Contains(result.Config.PostCreateCommand, "update-ca-certificates") {
		t.Errorf("postCreateCommand should include update-ca-certificates, got: %s",
			result.Config.PostCreateCommand)
	}
}

func TestGenerate_AddsProxyEnvironment(t *testing.T) {
	templates := []config.Template{
		{
			Name:  "test-template",
			Image: "ubuntu:22.04",
		},
	}

	cfg := &config.Config{
		Runtime: "docker",
	}

	gen := NewDevcontainerGenerator(cfg, templates)
	result, err := gen.Generate(CreateOptions{
		Template:    "test-template",
		Name:        "test-container",
		ProjectPath: "/test/project",
		Proxy: &ProxyConfig{
			CertDir:     "/tmp/certs",
			ProxyHost:   "proxy",
			ProxyPort:   "8080",
			NetworkName: "test-network",
		},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Check proxy environment variables
	wantEnv := map[string]string{
		"http_proxy":         "http://proxy:8080",
		"https_proxy":        "http://proxy:8080",
		"HTTP_PROXY":         "http://proxy:8080",
		"HTTPS_PROXY":        "http://proxy:8080",
		"no_proxy":           "localhost,127.0.0.1",
		"NO_PROXY":           "localhost,127.0.0.1",
		"REQUESTS_CA_BUNDLE": "/etc/ssl/certs/ca-certificates.crt",
		"NODE_EXTRA_CA_CERTS": "/etc/ssl/certs/ca-certificates.crt",
		"SSL_CERT_FILE":      "/etc/ssl/certs/ca-certificates.crt",
	}

	for key, want := range wantEnv {
		got, ok := result.Config.ContainerEnv[key]
		if !ok {
			t.Errorf("ContainerEnv missing %q", key)
			continue
		}
		if got != want {
			t.Errorf("ContainerEnv[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestGenerate_NoProxyEnvWithoutProxyConfig(t *testing.T) {
	templates := []config.Template{
		{
			Name:  "test-template",
			Image: "ubuntu:22.04",
		},
	}

	cfg := &config.Config{
		Runtime: "docker",
	}

	gen := NewDevcontainerGenerator(cfg, templates)
	result, err := gen.Generate(CreateOptions{
		Template:    "test-template",
		Name:        "test-container",
		ProjectPath: "/test/project",
		Proxy:       nil, // No proxy config
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Proxy environment variables should not be set
	proxyKeys := []string{"http_proxy", "https_proxy", "HTTP_PROXY", "HTTPS_PROXY"}
	for _, key := range proxyKeys {
		if _, ok := result.Config.ContainerEnv[key]; ok {
			t.Errorf("ContainerEnv should not have %q when no proxy config", key)
		}
	}
}

func TestGenerate_AddsNetworkToRunArgs(t *testing.T) {
	templates := []config.Template{
		{
			Name:  "test-template",
			Image: "ubuntu:22.04",
		},
	}

	cfg := &config.Config{
		Runtime: "docker",
	}

	gen := NewDevcontainerGenerator(cfg, templates)
	result, err := gen.Generate(CreateOptions{
		Template:    "test-template",
		Name:        "test-container",
		ProjectPath: "/test/project",
		Proxy: &ProxyConfig{
			CertDir:     "/tmp/certs",
			ProxyHost:   "proxy",
			ProxyPort:   "8080",
			NetworkName: "devagent-abc123-net",
		},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Check network is in runArgs
	foundNetwork := false
	for i, arg := range result.Config.RunArgs {
		if arg == "--network" && i+1 < len(result.Config.RunArgs) {
			if result.Config.RunArgs[i+1] == "devagent-abc123-net" {
				foundNetwork = true
				break
			}
		}
	}

	if !foundNetwork {
		t.Errorf("runArgs should contain --network devagent-abc123-net, got: %v", result.Config.RunArgs)
	}
}

func TestGenerate_NoNetworkWithoutProxyConfig(t *testing.T) {
	templates := []config.Template{
		{
			Name:  "test-template",
			Image: "ubuntu:22.04",
		},
	}

	cfg := &config.Config{
		Runtime: "docker",
	}

	gen := NewDevcontainerGenerator(cfg, templates)
	result, err := gen.Generate(CreateOptions{
		Template:    "test-template",
		Name:        "test-container",
		ProjectPath: "/test/project",
		Proxy:       nil,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// No --network flag should be present
	for _, arg := range result.Config.RunArgs {
		if arg == "--network" {
			t.Errorf("runArgs should not contain --network when no proxy config")
		}
	}
}

func TestGenerate_TemplateWithoutIsolationGetsDefaults(t *testing.T) {
	// Template with no isolation config should get DefaultIsolation applied
	templates := []config.Template{
		{
			Name:      "no-isolation-template",
			Image:     "ubuntu:22.04",
			Isolation: nil, // No isolation specified
		},
	}

	cfg := &config.Config{
		Runtime: "docker",
	}

	// Verify GetEffectiveIsolation returns defaults
	effective := templates[0].GetEffectiveIsolation()
	if effective == nil {
		t.Fatal("GetEffectiveIsolation() should return defaults, not nil")
	}

	// Verify it has default capabilities
	if effective.Caps == nil || len(effective.Caps.Drop) == 0 {
		t.Error("effective isolation should have default cap drops")
	}

	// Verify it has default resources
	if effective.Resources == nil || effective.Resources.Memory == "" {
		t.Error("effective isolation should have default resource limits")
	}

	// Verify it has default network allowlist
	if effective.Network == nil || len(effective.Network.Allowlist) == 0 {
		t.Error("effective isolation should have default allowlist")
	}

	// CRITICAL: Verify Generate() actually applies effective isolation to devcontainer.json
	gen := NewDevcontainerGenerator(cfg, templates)
	result, err := gen.Generate(CreateOptions{
		Template:    "no-isolation-template",
		Name:        "test-container",
		ProjectPath: "/test/project",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify runArgs contain isolation flags from defaults
	runArgs := result.Config.RunArgs

	// Check for default capability drops
	hasCapDrop := false
	for _, arg := range runArgs {
		if arg == "--cap-drop" {
			hasCapDrop = true
			break
		}
	}
	if !hasCapDrop {
		t.Errorf("Generate() should apply default cap drops, runArgs: %v", runArgs)
	}

	// Check for default memory limit
	hasMemory := false
	for i, arg := range runArgs {
		if arg == "--memory" && i+1 < len(runArgs) {
			hasMemory = true
			break
		}
	}
	if !hasMemory {
		t.Errorf("Generate() should apply default memory limit, runArgs: %v", runArgs)
	}
}

func TestGenerate_TemplateWithDisabledIsolation(t *testing.T) {
	enabled := false
	templates := []config.Template{
		{
			Name:  "disabled-isolation-template",
			Image: "ubuntu:22.04",
			Isolation: &config.IsolationConfig{
				Enabled: &enabled,
			},
		},
	}

	cfg := &config.Config{
		Runtime: "docker",
	}

	// Verify GetEffectiveIsolation returns nil
	effective := templates[0].GetEffectiveIsolation()
	if effective != nil {
		t.Errorf("GetEffectiveIsolation() should return nil when disabled, got %+v", effective)
	}

	// Verify IsIsolationEnabled returns false
	if templates[0].IsIsolationEnabled() {
		t.Error("IsIsolationEnabled() should return false")
	}

	// CRITICAL: Verify Generate() does NOT apply isolation when disabled
	gen := NewDevcontainerGenerator(cfg, templates)
	result, err := gen.Generate(CreateOptions{
		Template:    "disabled-isolation-template",
		Name:        "test-container",
		ProjectPath: "/test/project",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify runArgs do NOT contain isolation flags
	runArgs := result.Config.RunArgs
	for _, arg := range runArgs {
		if arg == "--cap-drop" || arg == "--memory" || arg == "--cpus" || arg == "--pids-limit" {
			t.Errorf("Generate() should not apply isolation when disabled, found %s in runArgs: %v", arg, runArgs)
			break
		}
	}
}

func TestGenerate_TemplateWithAllowlistExtend(t *testing.T) {
	templates := []config.Template{
		{
			Name:  "extended-allowlist-template",
			Image: "ubuntu:22.04",
			Isolation: &config.IsolationConfig{
				Network: &config.NetworkConfig{
					AllowlistExtend: []string{"custom.example.com", "internal.corp.net"},
				},
			},
		},
	}

	effective := templates[0].GetEffectiveIsolation()
	if effective == nil || effective.Network == nil {
		t.Fatal("effective isolation should not be nil")
	}

	// Should have default domains plus extended domains
	allowlist := effective.Network.Allowlist

	// Check for default domain
	hasDefault := false
	for _, domain := range allowlist {
		if domain == "github.com" || domain == "api.anthropic.com" {
			hasDefault = true
			break
		}
	}
	if !hasDefault {
		t.Error("allowlist should include default domains")
	}

	// Check for extended domain
	hasExtended := false
	for _, domain := range allowlist {
		if domain == "custom.example.com" {
			hasExtended = true
			break
		}
	}
	if !hasExtended {
		t.Error("allowlist should include extended domain custom.example.com")
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

func TestGenerate_AddsClaudeTokenMount(t *testing.T) {
	// Create temp directory with token
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("XDG_DATA_HOME", tmpDir) // For claude config dir

	// Create claude dir and token file
	claudeDir := filepath.Join(tmpDir, "claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create claude dir: %v", err)
	}
	tokenPath := filepath.Join(claudeDir, ".devagent-claude-token")
	if err := os.WriteFile(tokenPath, []byte("test-token"), 0600); err != nil {
		t.Fatalf("Failed to write token: %v", err)
	}

	templates := []config.Template{
		{
			Name:  "test-template",
			Image: "ubuntu:22.04",
		},
	}

	cfg := &config.Config{
		Runtime: "docker",
	}

	gen := NewDevcontainerGenerator(cfg, templates)
	result, err := gen.Generate(CreateOptions{
		Template:    "test-template",
		Name:        "test-container",
		ProjectPath: "/test/project",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify token mount was added
	foundTokenMount := false
	for _, mount := range result.Config.Mounts {
		if strings.Contains(mount, "/run/secrets/claude-token") && strings.Contains(mount, "readonly") {
			foundTokenMount = true
			break
		}
	}
	if !foundTokenMount {
		t.Errorf("token mount not found in Mounts: %v", result.Config.Mounts)
	}
}

func TestGenerate_NoClaudeTokenEnvInContainerEnv(t *testing.T) {
	// Verify CLAUDE_CODE_OAUTH_TOKEN is NOT in containerEnv anymore
	// (it's now injected via shell profile from mounted file)
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("XDG_DATA_HOME", tmpDir)

	// Create claude dir and token file
	claudeDir := filepath.Join(tmpDir, "claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create claude dir: %v", err)
	}
	tokenPath := filepath.Join(claudeDir, ".devagent-claude-token")
	if err := os.WriteFile(tokenPath, []byte("test-token"), 0600); err != nil {
		t.Fatalf("Failed to write token: %v", err)
	}

	templates := []config.Template{
		{
			Name:  "test-template",
			Image: "ubuntu:22.04",
		},
	}

	cfg := &config.Config{
		Runtime: "docker",
	}

	gen := NewDevcontainerGenerator(cfg, templates)
	result, err := gen.Generate(CreateOptions{
		Template:    "test-template",
		Name:        "test-container",
		ProjectPath: "/test/project",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify CLAUDE_CODE_OAUTH_TOKEN is NOT in containerEnv
	if _, ok := result.Config.ContainerEnv["CLAUDE_CODE_OAUTH_TOKEN"]; ok {
		t.Error("CLAUDE_CODE_OAUTH_TOKEN should not be in containerEnv (now uses mounted file)")
	}
}

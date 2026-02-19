package container

import (
	"context"
	"errors"
	"testing"
)

func TestParseContainerList_Empty(t *testing.T) {
	r := NewRuntimeWithExecutor("docker", nil)
	containers, err := r.parseContainerList("")
	if err != nil {
		t.Fatalf("parseContainerList failed: %v", err)
	}
	if len(containers) != 0 {
		t.Errorf("Expected empty slice, got %d containers", len(containers))
	}
}

func TestParseContainerList_SingleContainer(t *testing.T) {
	r := NewRuntimeWithExecutor("docker", nil)
	jsonOutput := `{"ID":"abc123","Names":"my-container","State":"running","Labels":"devagent.managed=true,devagent.project_path=/home/user/project,devagent.template=python,devagent.agent=claude-code","CreatedAt":"2024-01-15T10:30:00Z"}`

	containers, err := r.parseContainerList(jsonOutput)
	if err != nil {
		t.Fatalf("parseContainerList failed: %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("Expected 1 container, got %d", len(containers))
	}

	c := containers[0]
	if c.ID != "abc123" {
		t.Errorf("ID: got %q, want %q", c.ID, "abc123")
	}
	if c.Name != "my-container" {
		t.Errorf("Name: got %q, want %q", c.Name, "my-container")
	}
	if c.State != StateRunning {
		t.Errorf("State: got %q, want %q", c.State, StateRunning)
	}
	if c.ProjectPath != "/home/user/project" {
		t.Errorf("ProjectPath: got %q, want %q", c.ProjectPath, "/home/user/project")
	}
	if c.Template != "python" {
		t.Errorf("Template: got %q, want %q", c.Template, "python")
	}
	if c.Agent != "claude-code" {
		t.Errorf("Agent: got %q, want %q", c.Agent, "claude-code")
	}
}

func TestParseContainerList_MultipleContainers(t *testing.T) {
	r := NewRuntimeWithExecutor("docker", nil)
	jsonOutput := `{"ID":"abc123","Names":"container-1","State":"running","Labels":"devagent.managed=true"}
{"ID":"def456","Names":"container-2","State":"exited","Labels":"devagent.managed=true"}`

	containers, err := r.parseContainerList(jsonOutput)
	if err != nil {
		t.Fatalf("parseContainerList failed: %v", err)
	}
	if len(containers) != 2 {
		t.Errorf("Expected 2 containers, got %d", len(containers))
	}
}

func TestMapState_Running(t *testing.T) {
	got := mapState("running")
	if got != StateRunning {
		t.Errorf("mapState(running): got %q, want %q", got, StateRunning)
	}
}

func TestMapState_Stopped(t *testing.T) {
	got := mapState("exited")
	if got != StateStopped {
		t.Errorf("mapState(exited): got %q, want %q", got, StateStopped)
	}
}

func TestMapState_Created(t *testing.T) {
	got := mapState("created")
	if got != StateCreated {
		t.Errorf("mapState(created): got %q, want %q", got, StateCreated)
	}
}

func TestMapState_Paused(t *testing.T) {
	got := mapState("paused")
	if got != StateStopped {
		t.Errorf("mapState(paused): got %q, want %q", got, StateStopped)
	}
}

func TestMapState_Unknown(t *testing.T) {
	got := mapState("unknown-state")
	if got != StateStopped {
		t.Errorf("mapState(unknown): got %q, want %q", got, StateStopped)
	}
}

func TestParseLabels(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "empty",
			input: "",
			want:  map[string]string{},
		},
		{
			name:  "single label",
			input: "key1=val1",
			want:  map[string]string{"key1": "val1"},
		},
		{
			name:  "multiple labels",
			input: "key1=val1,key2=val2,key3=val3",
			want:  map[string]string{"key1": "val1", "key2": "val2", "key3": "val3"},
		},
		{
			name:  "label with empty value",
			input: "key1=,key2=val2",
			want:  map[string]string{"key1": "", "key2": "val2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLabels(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("parseLabels(%q) len = %d, want %d", tt.input, len(got), len(tt.want))
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("parseLabels(%q)[%s] = %q, want %q", tt.input, k, got[k], v)
				}
			}
		})
	}
}

func TestListContainers_CallsCorrectCommand(t *testing.T) {
	var capturedName string
	var capturedArgs []string

	mockExec := func(ctx context.Context, name string, args ...string) (string, error) {
		capturedName = name
		capturedArgs = args
		return "", nil
	}

	r := NewRuntimeWithExecutor("docker", mockExec)
	_, _ = r.ListContainers(context.Background())

	if capturedName != "docker" {
		t.Errorf("Expected docker, got %q", capturedName)
	}
	expectedArgs := []string{"ps", "-a", "--filter", "label=devagent.managed=true", "--format", "json"}
	if len(capturedArgs) != len(expectedArgs) {
		t.Fatalf("Args length: got %d, want %d", len(capturedArgs), len(expectedArgs))
	}
	for i, arg := range expectedArgs {
		if capturedArgs[i] != arg {
			t.Errorf("Arg[%d]: got %q, want %q", i, capturedArgs[i], arg)
		}
	}
}

func TestStartContainer_CallsCorrectCommand(t *testing.T) {
	var capturedArgs []string

	mockExec := func(ctx context.Context, name string, args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	}

	r := NewRuntimeWithExecutor("docker", mockExec)
	err := r.StartContainer(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("StartContainer failed: %v", err)
	}

	expectedArgs := []string{"start", "abc123"}
	if len(capturedArgs) != len(expectedArgs) {
		t.Fatalf("Args: got %v, want %v", capturedArgs, expectedArgs)
	}
	for i, arg := range expectedArgs {
		if capturedArgs[i] != arg {
			t.Errorf("Arg[%d]: got %q, want %q", i, capturedArgs[i], arg)
		}
	}
}

func TestStopContainer_CallsCorrectCommand(t *testing.T) {
	var capturedArgs []string

	mockExec := func(ctx context.Context, name string, args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	}

	r := NewRuntimeWithExecutor("docker", mockExec)
	err := r.StopContainer(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("StopContainer failed: %v", err)
	}

	expectedArgs := []string{"stop", "abc123"}
	if len(capturedArgs) != len(expectedArgs) {
		t.Fatalf("Args: got %v, want %v", capturedArgs, expectedArgs)
	}
}

func TestRemoveContainer_CallsCorrectCommand(t *testing.T) {
	var capturedArgs []string

	mockExec := func(ctx context.Context, name string, args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	}

	r := NewRuntimeWithExecutor("docker", mockExec)
	err := r.RemoveContainer(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("RemoveContainer failed: %v", err)
	}

	expectedArgs := []string{"rm", "abc123"}
	if len(capturedArgs) != len(expectedArgs) {
		t.Fatalf("Args: got %v, want %v", capturedArgs, expectedArgs)
	}
}

func TestExec_CallsCorrectCommand(t *testing.T) {
	var capturedArgs []string

	mockExec := func(ctx context.Context, name string, args ...string) (string, error) {
		capturedArgs = args
		return "output", nil
	}

	r := NewRuntimeWithExecutor("docker", mockExec)
	output, err := r.Exec(context.Background(), "abc123", []string{"echo", "hello"})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if output != "output" {
		t.Errorf("Output: got %q, want %q", output, "output")
	}

	expectedArgs := []string{"exec", "abc123", "echo", "hello"}
	if len(capturedArgs) != len(expectedArgs) {
		t.Fatalf("Args: got %v, want %v", capturedArgs, expectedArgs)
	}
}

func TestListContainers_ReturnsError(t *testing.T) {
	mockExec := func(ctx context.Context, name string, args ...string) (string, error) {
		return "", errors.New("docker not running")
	}

	r := NewRuntimeWithExecutor("docker", mockExec)
	_, err := r.ListContainers(context.Background())
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestNewRuntime(t *testing.T) {
	r := NewRuntime("podman")
	if r.executable != "podman" {
		t.Errorf("executable: got %q, want %q", r.executable, "podman")
	}
	if r.exec == nil {
		t.Error("exec should not be nil")
	}
}

func TestGetIsolationInfo(t *testing.T) {
	tests := []struct {
		name       string
		inspectOut string
		wantInfo   *IsolationInfo
		wantErr    bool
	}{
		{
			name: "parses full isolation config with proxy env vars",
			inspectOut: `[{
				"HostConfig": {
					"CapDrop": ["NET_RAW", "SYS_ADMIN"],
					"CapAdd": ["SYS_PTRACE"],
					"Memory": 4294967296,
					"NanoCpus": 2000000000,
					"PidsLimit": 512
				},
				"Config": {
					"Env": [
						"http_proxy=http://proxy:8080",
						"https_proxy=http://proxy:8080",
						"PATH=/usr/local/bin:/usr/bin"
					]
				},
				"NetworkSettings": {
					"Networks": {
						"myproject_isolated": {
							"IPAddress": "172.20.0.2",
							"Gateway": "172.20.0.1",
							"MacAddress": "02:42:ac:14:00:02"
						}
					}
				}
			}]`,
			wantInfo: &IsolationInfo{
				DroppedCaps:     []string{"NET_RAW", "SYS_ADMIN"},
				AddedCaps:       []string{"SYS_PTRACE"},
				MemoryLimit:     "4g",
				CPULimit:        "2",
				PidsLimit:       512,
				NetworkIsolated: true,
				NetworkName:     "myproject_isolated",
				ContainerIP:     "172.20.0.2",
				Gateway:         "172.20.0.1",
				ProxyAddress:    "http://proxy:8080",
			},
		},
		{
			name: "detects isolation via proxy env on non-isolated-named network",
			inspectOut: `[{
				"HostConfig": {
					"CapDrop": ["NET_RAW"],
					"CapAdd": null,
					"Memory": 4294967296,
					"NanoCpus": 2000000000,
					"PidsLimit": 512
				},
				"Config": {
					"Env": [
						"https_proxy=http://mitmproxy:8080",
						"PATH=/usr/bin"
					]
				},
				"NetworkSettings": {
					"Networks": {
						"myproject_internal": {
							"IPAddress": "10.0.0.5",
							"Gateway": "10.0.0.1",
							"MacAddress": ""
						}
					}
				}
			}]`,
			wantInfo: &IsolationInfo{
				DroppedCaps:     []string{"NET_RAW"},
				MemoryLimit:     "4g",
				CPULimit:        "2",
				PidsLimit:       512,
				NetworkIsolated: true,
				NetworkName:     "myproject_internal",
				ContainerIP:     "10.0.0.5",
				Gateway:         "10.0.0.1",
				ProxyAddress:    "http://mitmproxy:8080",
			},
		},
		{
			name: "not isolated without proxy env vars",
			inspectOut: `[{
				"HostConfig": {
					"CapDrop": ["NET_RAW"],
					"CapAdd": null,
					"Memory": 536870912,
					"NanoCpus": 500000000,
					"PidsLimit": 0
				},
				"Config": {
					"Env": [
						"PATH=/usr/local/bin:/usr/bin",
						"HOME=/home/vscode"
					]
				},
				"NetworkSettings": {
					"Networks": {
						"bridge": {
							"IPAddress": "172.17.0.2",
							"Gateway": "172.17.0.1",
							"MacAddress": ""
						}
					}
				}
			}]`,
			wantInfo: &IsolationInfo{
				DroppedCaps:     []string{"NET_RAW"},
				AddedCaps:       nil,
				MemoryLimit:     "512m",
				CPULimit:        "0.50",
				PidsLimit:       0,
				NetworkIsolated: false,
				NetworkName:     "",
				ContainerIP:     "",
				Gateway:         "",
			},
		},
		{
			name: "parses container with no limits and no env",
			inspectOut: `[{
				"HostConfig": {
					"CapDrop": null,
					"CapAdd": null,
					"Memory": 0,
					"NanoCpus": 0,
					"PidsLimit": 0
				},
				"NetworkSettings": {
					"Networks": {}
				}
			}]`,
			wantInfo: &IsolationInfo{
				DroppedCaps:     nil,
				AddedCaps:       nil,
				MemoryLimit:     "",
				CPULimit:        "",
				PidsLimit:       0,
				NetworkIsolated: false,
				NetworkName:     "",
			},
		},
		{
			name:       "returns error for empty output",
			inspectOut: `[]`,
			wantErr:    true,
		},
		{
			name:       "returns error for invalid JSON",
			inspectOut: `invalid json`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := func(ctx context.Context, name string, args ...string) (string, error) {
				return tt.inspectOut, nil
			}

			r := NewRuntimeWithExecutor("docker", mockExec)
			info, err := r.GetIsolationInfo(context.Background(), "test-container")

			if (err != nil) != tt.wantErr {
				t.Errorf("GetIsolationInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Compare results
			if len(info.DroppedCaps) != len(tt.wantInfo.DroppedCaps) {
				t.Errorf("DroppedCaps length: got %d, want %d", len(info.DroppedCaps), len(tt.wantInfo.DroppedCaps))
			}
			if len(info.AddedCaps) != len(tt.wantInfo.AddedCaps) {
				t.Errorf("AddedCaps length: got %d, want %d", len(info.AddedCaps), len(tt.wantInfo.AddedCaps))
			}
			if info.MemoryLimit != tt.wantInfo.MemoryLimit {
				t.Errorf("MemoryLimit: got %q, want %q", info.MemoryLimit, tt.wantInfo.MemoryLimit)
			}
			if info.CPULimit != tt.wantInfo.CPULimit {
				t.Errorf("CPULimit: got %q, want %q", info.CPULimit, tt.wantInfo.CPULimit)
			}
			if info.PidsLimit != tt.wantInfo.PidsLimit {
				t.Errorf("PidsLimit: got %d, want %d", info.PidsLimit, tt.wantInfo.PidsLimit)
			}
			if info.NetworkIsolated != tt.wantInfo.NetworkIsolated {
				t.Errorf("NetworkIsolated: got %v, want %v", info.NetworkIsolated, tt.wantInfo.NetworkIsolated)
			}
			if info.NetworkName != tt.wantInfo.NetworkName {
				t.Errorf("NetworkName: got %q, want %q", info.NetworkName, tt.wantInfo.NetworkName)
			}
			if info.ContainerIP != tt.wantInfo.ContainerIP {
				t.Errorf("ContainerIP: got %q, want %q", info.ContainerIP, tt.wantInfo.ContainerIP)
			}
			if info.Gateway != tt.wantInfo.Gateway {
				t.Errorf("Gateway: got %q, want %q", info.Gateway, tt.wantInfo.Gateway)
			}
			if info.ProxyAddress != tt.wantInfo.ProxyAddress {
				t.Errorf("ProxyAddress: got %q, want %q", info.ProxyAddress, tt.wantInfo.ProxyAddress)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0"},
		{1024 * 1024, "1m"},
		{512 * 1024 * 1024, "512m"},
		{1024 * 1024 * 1024, "1g"},
		{4 * 1024 * 1024 * 1024, "4g"},
		{1536 * 1024 * 1024, "1536m"},  // 1.5 GB shows as MB since not evenly divisible by GB
		{100 * 1024 * 1024, "100m"},    // 100 MB
		{1024*1024 + 512*1024, "1.5m"}, // 1.5 MB (non-round)
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatBytes(tt.bytes)
			if got != tt.want {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestComposeUp_Docker(t *testing.T) {
	var capturedCmd string
	var capturedArgs []string

	mockExec := func(ctx context.Context, name string, args ...string) (string, error) {
		capturedCmd = name
		capturedArgs = args
		return "", nil
	}

	r := NewRuntimeWithExecutor("docker", mockExec)
	err := r.ComposeUp(context.Background(), "/home/user/project", "myproject")

	if err != nil {
		t.Fatalf("ComposeUp failed: %v", err)
	}

	if capturedCmd != "docker" {
		t.Errorf("Expected command 'docker', got %q", capturedCmd)
	}

	expectedArgs := []string{
		"compose",
		"-f", "/home/user/project/.devcontainer/docker-compose.yml",
		"-p", "myproject",
		"up", "-d",
	}

	if len(capturedArgs) != len(expectedArgs) {
		t.Fatalf("Expected %d args, got %d: %v", len(expectedArgs), len(capturedArgs), capturedArgs)
	}

	for i, expected := range expectedArgs {
		if capturedArgs[i] != expected {
			t.Errorf("Arg %d: expected %q, got %q", i, expected, capturedArgs[i])
		}
	}
}

func TestComposeUp_Podman(t *testing.T) {
	var capturedCmd string
	var capturedArgs []string

	mockExec := func(ctx context.Context, name string, args ...string) (string, error) {
		capturedCmd = name
		capturedArgs = args
		return "", nil
	}

	r := NewRuntimeWithExecutor("podman", mockExec)
	err := r.ComposeUp(context.Background(), "/home/user/project", "myproject")

	if err != nil {
		t.Fatalf("ComposeUp failed: %v", err)
	}

	// Podman uses standalone podman-compose
	if capturedCmd != "podman-compose" {
		t.Errorf("Expected command 'podman-compose', got %q", capturedCmd)
	}

	expectedArgs := []string{
		"-f", "/home/user/project/.devcontainer/docker-compose.yml",
		"-p", "myproject",
		"up", "-d",
	}

	if len(capturedArgs) != len(expectedArgs) {
		t.Fatalf("Expected %d args, got %d: %v", len(expectedArgs), len(capturedArgs), capturedArgs)
	}

	for i, expected := range expectedArgs {
		if capturedArgs[i] != expected {
			t.Errorf("Arg %d: expected %q, got %q", i, expected, capturedArgs[i])
		}
	}
}

func TestComposeStart_Docker(t *testing.T) {
	var capturedArgs []string

	mockExec := func(ctx context.Context, name string, args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	}

	r := NewRuntimeWithExecutor("docker", mockExec)
	err := r.ComposeStart(context.Background(), "/test/project", "testproj")

	if err != nil {
		t.Fatalf("ComposeStart failed: %v", err)
	}

	// Verify 'start' command (not 'up -d')
	if capturedArgs[len(capturedArgs)-1] != "start" {
		t.Errorf("Expected 'start' command, got %q", capturedArgs[len(capturedArgs)-1])
	}
}

func TestComposeStop_Docker(t *testing.T) {
	var capturedArgs []string

	mockExec := func(ctx context.Context, name string, args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	}

	r := NewRuntimeWithExecutor("docker", mockExec)
	err := r.ComposeStop(context.Background(), "/test/project", "testproj")

	if err != nil {
		t.Fatalf("ComposeStop failed: %v", err)
	}

	// Verify 'stop' command
	if capturedArgs[len(capturedArgs)-1] != "stop" {
		t.Errorf("Expected 'stop' command, got %q", capturedArgs[len(capturedArgs)-1])
	}
}

func TestComposeDown_Docker(t *testing.T) {
	var capturedArgs []string

	mockExec := func(ctx context.Context, name string, args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	}

	r := NewRuntimeWithExecutor("docker", mockExec)
	err := r.ComposeDown(context.Background(), "/test/project", "testproj")

	if err != nil {
		t.Fatalf("ComposeDown failed: %v", err)
	}

	// Verify 'down' command
	if capturedArgs[len(capturedArgs)-1] != "down" {
		t.Errorf("Expected 'down' command, got %q", capturedArgs[len(capturedArgs)-1])
	}
}

func TestComposeUp_ReturnsError(t *testing.T) {
	expectedErr := errors.New("compose failed")

	mockExec := func(ctx context.Context, name string, args ...string) (string, error) {
		return "", expectedErr
	}

	r := NewRuntimeWithExecutor("docker", mockExec)
	err := r.ComposeUp(context.Background(), "/test", "proj")

	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

func TestComposeCommand_Docker(t *testing.T) {
	r := NewRuntimeWithExecutor("docker", nil)
	cmd, baseArgs := r.composeCommand()

	if cmd != "docker" {
		t.Errorf("Expected 'docker', got %q", cmd)
	}
	if len(baseArgs) != 1 || baseArgs[0] != "compose" {
		t.Errorf("Expected ['compose'], got %v", baseArgs)
	}
}

func TestComposeCommand_Podman(t *testing.T) {
	r := NewRuntimeWithExecutor("podman", nil)
	cmd, baseArgs := r.composeCommand()

	if cmd != "podman-compose" {
		t.Errorf("Expected 'podman-compose', got %q", cmd)
	}
	if len(baseArgs) != 0 {
		t.Errorf("Expected empty baseArgs for podman, got %v", baseArgs)
	}
}

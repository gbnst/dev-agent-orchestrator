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

func TestRuntime_CreateNetwork(t *testing.T) {
	tests := []struct {
		name       string
		network    string
		execOutput string
		execErr    error
		wantID     string
		wantErr    bool
	}{
		{
			name:       "creates network successfully",
			network:    "test-network",
			execOutput: "abc123def456\n",
			wantID:     "abc123def456",
		},
		{
			name:       "handles output without newline",
			network:    "test-network",
			execOutput: "abc123def456",
			wantID:     "abc123def456",
		},
		{
			name:    "returns error on failure",
			network: "test-network",
			execErr: errors.New("network already exists"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedArgs []string
			mockExec := func(ctx context.Context, name string, args ...string) (string, error) {
				capturedArgs = append([]string{name}, args...)
				return tt.execOutput, tt.execErr
			}

			r := NewRuntimeWithExecutor("docker", mockExec)
			id, err := r.CreateNetwork(context.Background(), tt.network)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreateNetwork() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if id != tt.wantID {
					t.Errorf("CreateNetwork() id = %q, want %q", id, tt.wantID)
				}

				// Verify command
				wantArgs := []string{"docker", "network", "create", tt.network}
				if len(capturedArgs) != len(wantArgs) {
					t.Errorf("CreateNetwork() args = %v, want %v", capturedArgs, wantArgs)
				}
			}
		})
	}
}

func TestRuntime_RemoveNetwork(t *testing.T) {
	tests := []struct {
		name    string
		network string
		execErr error
		wantErr bool
	}{
		{
			name:    "removes network successfully",
			network: "test-network",
		},
		{
			name:    "returns error on failure",
			network: "test-network",
			execErr: errors.New("network in use"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedArgs []string
			mockExec := func(ctx context.Context, name string, args ...string) (string, error) {
				capturedArgs = append([]string{name}, args...)
				return "", tt.execErr
			}

			r := NewRuntimeWithExecutor("docker", mockExec)
			err := r.RemoveNetwork(context.Background(), tt.network)

			if (err != nil) != tt.wantErr {
				t.Errorf("RemoveNetwork() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify command includes -f flag
				wantArgs := []string{"docker", "network", "rm", "-f", tt.network}
				if len(capturedArgs) != len(wantArgs) {
					t.Errorf("RemoveNetwork() args = %v, want %v", capturedArgs, wantArgs)
				}
			}
		})
	}
}

func TestRuntime_RunContainer(t *testing.T) {
	tests := []struct {
		name       string
		opts       RunContainerOptions
		execOutput string
		execErr    error
		wantID     string
		wantErr    bool
		wantArgs   []string
	}{
		{
			name: "minimal container run",
			opts: RunContainerOptions{
				Image:  "nginx:alpine",
				Detach: true,
			},
			execOutput: "container123\n",
			wantID:     "container123",
			wantArgs:   []string{"docker", "run", "-d", "nginx:alpine"},
		},
		{
			name: "full options",
			opts: RunContainerOptions{
				Image:      "mitmproxy/mitmproxy:latest",
				Name:       "proxy-sidecar",
				Network:    "devagent-net",
				Detach:     true,
				AutoRemove: true,
				Labels: map[string]string{
					"devagent.managed": "true",
					"devagent.sidecar": "proxy",
				},
				Env: map[string]string{
					"PROXY_PORT": "8080",
				},
				Volumes: []string{
					"/host/certs:/certs:ro",
					"/host/config:/config",
				},
				Command: []string{"mitmdump", "-p", "8080"},
			},
			execOutput: "abc123\n",
			wantID:     "abc123",
			wantArgs: []string{"docker", "run", "-d", "--rm", "--name", "proxy-sidecar", "--network", "devagent-net", "--label", "devagent.managed=true", "--label", "devagent.sidecar=proxy", "-e", "PROXY_PORT=8080", "-v", "/host/certs:/certs:ro", "-v", "/host/config:/config", "mitmproxy/mitmproxy:latest", "mitmdump", "-p", "8080"},
		},
		{
			name: "returns error on failure",
			opts: RunContainerOptions{
				Image:  "nonexistent:image",
				Detach: true,
			},
			execErr: errors.New("image not found"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedArgs []string
			mockExec := func(ctx context.Context, name string, args ...string) (string, error) {
				capturedArgs = append([]string{name}, args...)
				return tt.execOutput, tt.execErr
			}

			r := NewRuntimeWithExecutor("docker", mockExec)
			id, err := r.RunContainer(context.Background(), tt.opts)

			if (err != nil) != tt.wantErr {
				t.Errorf("RunContainer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if id != tt.wantID {
					t.Errorf("RunContainer() id = %q, want %q", id, tt.wantID)
				}

				// Verify basic structure
				if len(capturedArgs) < 3 {
					t.Errorf("RunContainer() args too short: %v", capturedArgs)
					return
				}
				if capturedArgs[0] != "docker" || capturedArgs[1] != "run" {
					t.Errorf("RunContainer() should start with 'docker run', got: %v", capturedArgs[:2])
				}

				// Verify specific expected args if provided
				if tt.wantArgs != nil {
					if len(capturedArgs) != len(tt.wantArgs) {
						t.Errorf("RunContainer() args = %v, want %v", capturedArgs, tt.wantArgs)
					}
					for i, want := range tt.wantArgs {
						if i < len(capturedArgs) && capturedArgs[i] != want {
							t.Errorf("RunContainer() args[%d] = %q, want %q", i, capturedArgs[i], want)
						}
					}
				}
			}
		})
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
			name: "parses full isolation config",
			inspectOut: `[{
				"HostConfig": {
					"CapDrop": ["NET_RAW", "SYS_ADMIN"],
					"CapAdd": ["SYS_PTRACE"],
					"Memory": 4294967296,
					"NanoCpus": 2000000000,
					"PidsLimit": 512
				},
				"NetworkSettings": {
					"Networks": {
						"devagent-abc123-net": {}
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
				NetworkName:     "devagent-abc123-net",
			},
		},
		{
			name: "parses container without network isolation",
			inspectOut: `[{
				"HostConfig": {
					"CapDrop": ["NET_RAW"],
					"CapAdd": null,
					"Memory": 536870912,
					"NanoCpus": 500000000,
					"PidsLimit": 0
				},
				"NetworkSettings": {
					"Networks": {
						"bridge": {}
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
			},
		},
		{
			name: "parses container with no limits",
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
		{1536 * 1024 * 1024, "1536m"},        // 1.5 GB shows as MB since not evenly divisible by GB
		{100 * 1024 * 1024, "100m"},          // 100 MB
		{1024*1024 + 512*1024, "1.5m"},       // 1.5 MB (non-round)
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

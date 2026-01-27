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

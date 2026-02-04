package tmux

import (
	"context"
	"errors"
	"testing"
)

// mockExecRecorder records all exec calls for verification.
type mockExecRecorder struct {
	calls   []execCall
	outputs map[string]string
	errors  map[string]error
}

type execCall struct {
	containerID string
	cmd         []string
}

func newMockExec() *mockExecRecorder {
	return &mockExecRecorder{
		outputs: make(map[string]string),
		errors:  make(map[string]error),
	}
}

func (m *mockExecRecorder) exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	m.calls = append(m.calls, execCall{containerID, cmd})
	key := containerID + ":" + cmd[0]
	if err, ok := m.errors[key]; ok {
		return "", err
	}
	if out, ok := m.outputs[key]; ok {
		return out, nil
	}
	return "", nil
}

func TestClient_ListSessions_ParsesOutput(t *testing.T) {
	mock := newMockExec()
	// tmux list-sessions format: name: windows (created date) (attached)
	mock.outputs["container1:tmux"] = `dev: 2 windows (created Mon Jan 20 10:00:00 2025)
main: 1 windows (created Mon Jan 20 09:00:00 2025) (attached)
`
	client := NewClientWithExecutor(mock.exec)

	sessions, err := client.ListSessions(context.Background(), "container1")
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("ListSessions() returned %d sessions, want 2", len(sessions))
	}

	// Check first session
	if sessions[0].Name != "dev" {
		t.Errorf("sessions[0].Name = %q, want %q", sessions[0].Name, "dev")
	}
	if sessions[0].Windows != 2 {
		t.Errorf("sessions[0].Windows = %d, want %d", sessions[0].Windows, 2)
	}
	if sessions[0].Attached {
		t.Error("sessions[0].Attached = true, want false")
	}
	if sessions[0].ContainerID != "container1" {
		t.Errorf("sessions[0].ContainerID = %q, want %q", sessions[0].ContainerID, "container1")
	}

	// Check second session
	if sessions[1].Name != "main" {
		t.Errorf("sessions[1].Name = %q, want %q", sessions[1].Name, "main")
	}
	if !sessions[1].Attached {
		t.Error("sessions[1].Attached = false, want true")
	}
}

func TestClient_ListSessions_EmptyOutput(t *testing.T) {
	mock := newMockExec()
	mock.outputs["container1:tmux"] = ""
	client := NewClientWithExecutor(mock.exec)

	sessions, err := client.ListSessions(context.Background(), "container1")
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("ListSessions() returned %d sessions, want 0", len(sessions))
	}
}

func TestClient_ListSessions_NoServer(t *testing.T) {
	mock := newMockExec()
	// tmux returns error when no server is running
	mock.errors["container1:tmux"] = errors.New("no server running on /tmp/tmux-1000/default")
	client := NewClientWithExecutor(mock.exec)

	sessions, err := client.ListSessions(context.Background(), "container1")
	// Should return empty list, not error (no server = no sessions)
	if err != nil {
		t.Fatalf("ListSessions() error = %v, want nil", err)
	}
	if len(sessions) != 0 {
		t.Errorf("ListSessions() returned %d sessions, want 0", len(sessions))
	}
}

func TestClient_CreateSession(t *testing.T) {
	mock := newMockExec()
	client := NewClientWithExecutor(mock.exec)

	err := client.CreateSession(context.Background(), "container1", "dev")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Verify correct command was called
	if len(mock.calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mock.calls))
	}

	call := mock.calls[0]
	if call.containerID != "container1" {
		t.Errorf("containerID = %q, want %q", call.containerID, "container1")
	}

	// Should call: tmux -u new-session -d -s dev
	expectedCmd := []string{"tmux", "-u", "new-session", "-d", "-s", "dev"}
	if len(call.cmd) != len(expectedCmd) {
		t.Fatalf("cmd = %v, want %v", call.cmd, expectedCmd)
	}
	for i, arg := range expectedCmd {
		if call.cmd[i] != arg {
			t.Errorf("cmd[%d] = %q, want %q", i, call.cmd[i], arg)
		}
	}
}

func TestClient_KillSession(t *testing.T) {
	mock := newMockExec()
	client := NewClientWithExecutor(mock.exec)

	err := client.KillSession(context.Background(), "container1", "dev")
	if err != nil {
		t.Fatalf("KillSession() error = %v", err)
	}

	if len(mock.calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mock.calls))
	}

	call := mock.calls[0]
	expectedCmd := []string{"tmux", "kill-session", "-t", "dev"}
	if len(call.cmd) != len(expectedCmd) {
		t.Fatalf("cmd = %v, want %v", call.cmd, expectedCmd)
	}
	for i, arg := range expectedCmd {
		if call.cmd[i] != arg {
			t.Errorf("cmd[%d] = %q, want %q", i, call.cmd[i], arg)
		}
	}
}

func TestClient_CapturePane(t *testing.T) {
	mock := newMockExec()
	mock.outputs["container1:tmux"] = "$ echo hello\nhello\n$ "
	client := NewClientWithExecutor(mock.exec)

	content, err := client.CapturePane(context.Background(), "container1", "dev")
	if err != nil {
		t.Fatalf("CapturePane() error = %v", err)
	}

	expected := "$ echo hello\nhello\n$ "
	if content != expected {
		t.Errorf("CapturePane() = %q, want %q", content, expected)
	}

	// Verify command
	if len(mock.calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mock.calls))
	}
	call := mock.calls[0]
	expectedCmd := []string{"tmux", "capture-pane", "-t", "dev", "-p"}
	for i, arg := range expectedCmd {
		if call.cmd[i] != arg {
			t.Errorf("cmd[%d] = %q, want %q", i, call.cmd[i], arg)
		}
	}
}

func TestClient_SendKeys(t *testing.T) {
	mock := newMockExec()
	client := NewClientWithExecutor(mock.exec)

	err := client.SendKeys(context.Background(), "container1", "dev", "echo hello")
	if err != nil {
		t.Fatalf("SendKeys() error = %v", err)
	}

	if len(mock.calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mock.calls))
	}

	call := mock.calls[0]
	// Should call: tmux send-keys -t dev "echo hello" Enter
	expectedCmd := []string{"tmux", "send-keys", "-t", "dev", "echo hello", "Enter"}
	if len(call.cmd) != len(expectedCmd) {
		t.Fatalf("cmd = %v, want %v", call.cmd, expectedCmd)
	}
	for i, arg := range expectedCmd {
		if call.cmd[i] != arg {
			t.Errorf("cmd[%d] = %q, want %q", i, call.cmd[i], arg)
		}
	}
}

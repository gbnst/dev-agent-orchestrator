package tmux

import (
	"context"
	"errors"
	"strings"
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
	// Try specific key first (containerID:cmd[0]:cmd[1]), then generic (containerID:cmd[0])
	if len(cmd) > 1 {
		specificKey := containerID + ":" + cmd[0] + ":" + cmd[1]
		if err, ok := m.errors[specificKey]; ok {
			return "", err
		}
		if out, ok := m.outputs[specificKey]; ok {
			return out, nil
		}
	}
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
	client := NewClient(mock.exec)

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
	client := NewClient(mock.exec)

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
	client := NewClient(mock.exec)

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
	client := NewClient(mock.exec)

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
	client := NewClient(mock.exec)

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
	client := NewClient(mock.exec)

	content, err := client.CapturePane(context.Background(), "container1", "dev", CaptureOpts{FromCursor: -1})
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

func TestClient_CapturePane_WithLines(t *testing.T) {
	mock := newMockExec()
	mock.outputs["container1:tmux"] = "line1\nline2\nline3\nline4\nline5\n"
	client := NewClient(mock.exec)

	content, err := client.CapturePane(context.Background(), "container1", "dev", CaptureOpts{Lines: 2, FromCursor: -1})
	if err != nil {
		t.Fatalf("CapturePane() error = %v", err)
	}

	// Should return only last 2 lines (after trimming trailing empty lines)
	expected := "line4\nline5"
	if content != expected {
		t.Errorf("CapturePane() = %q, want %q", content, expected)
	}

	// Verify command does NOT include -S flag (lines are trimmed in Go, not by tmux)
	if len(mock.calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mock.calls))
	}
	call := mock.calls[0]
	expectedCmd := []string{"tmux", "capture-pane", "-t", "dev", "-p"}
	if len(call.cmd) != len(expectedCmd) {
		t.Fatalf("cmd = %v, want %v", call.cmd, expectedCmd)
	}
	for i, arg := range expectedCmd {
		if call.cmd[i] != arg {
			t.Errorf("cmd[%d] = %q, want %q", i, call.cmd[i], arg)
		}
	}
}

func TestClient_CapturePane_WithLines_FewerThanRequested(t *testing.T) {
	mock := newMockExec()
	mock.outputs["container1:tmux"] = "line1\nline2\n"
	client := NewClient(mock.exec)

	// Request more lines than available
	content, err := client.CapturePane(context.Background(), "container1", "dev", CaptureOpts{Lines: 50, FromCursor: -1})
	if err != nil {
		t.Fatalf("CapturePane() error = %v", err)
	}

	// Should return all lines when fewer than requested
	expected := "line1\nline2"
	if content != expected {
		t.Errorf("CapturePane() = %q, want %q", content, expected)
	}
}

func TestClient_CapturePane_WithFromCursor(t *testing.T) {
	mock := newMockExec()
	// capture-pane returns content from scrollback
	mock.outputs["container1:tmux:capture-pane"] = "line12\nline13\n"
	// display-message returns current absolute position: history_size=100, cursor_y=15 → 115
	mock.outputs["container1:tmux:display-message"] = "100 15\n"
	client := NewClient(mock.exec)

	// FromCursor=112 means we want content from absolute position 112.
	// Current position is 115, so we need 3 lines back: -S -3
	content, err := client.CapturePane(context.Background(), "container1", "dev", CaptureOpts{FromCursor: 112})
	if err != nil {
		t.Fatalf("CapturePane() error = %v", err)
	}

	expected := "line12\nline13"
	if content != expected {
		t.Errorf("CapturePane() = %q, want %q", content, expected)
	}

	// Should have 2 calls: display-message (for position) + capture-pane
	if len(mock.calls) != 2 {
		t.Fatalf("Expected 2 calls, got %d: %v", len(mock.calls), mock.calls)
	}

	// First call: display-message to get current position
	call0 := mock.calls[0]
	if call0.cmd[1] != "display-message" {
		t.Errorf("first call should be display-message, got %v", call0.cmd)
	}

	// Second call: capture-pane with -S -3 (115 - 112 = 3 lines back)
	call1 := mock.calls[1]
	expectedCmd := []string{"tmux", "capture-pane", "-t", "dev", "-p", "-S", "-3"}
	if len(call1.cmd) != len(expectedCmd) {
		t.Fatalf("cmd = %v, want %v", call1.cmd, expectedCmd)
	}
	for i, arg := range expectedCmd {
		if call1.cmd[i] != arg {
			t.Errorf("cmd[%d] = %q, want %q", i, call1.cmd[i], arg)
		}
	}
}

func TestClient_CapturePane_WithFromCursor_NoNewOutput(t *testing.T) {
	mock := newMockExec()
	// Current position equals from_cursor — no new output expected
	mock.outputs["container1:tmux:capture-pane"] = "\n"
	mock.outputs["container1:tmux:display-message"] = "50 10\n"
	client := NewClient(mock.exec)

	// FromCursor=60 (same as current 50+10), so linesBack=0 — just capture visible pane
	content, err := client.CapturePane(context.Background(), "container1", "dev", CaptureOpts{FromCursor: 60})
	if err != nil {
		t.Fatalf("CapturePane() error = %v", err)
	}

	if content != "" {
		t.Errorf("expected empty content, got %q", content)
	}
}

func TestClient_CaptureLines(t *testing.T) {
	mock := newMockExec()
	mock.outputs["container1:tmux"] = "line18\nline19\nline20\n"
	client := NewClient(mock.exec)

	content, err := client.CaptureLines(context.Background(), "container1", "dev", 3)
	if err != nil {
		t.Fatalf("CaptureLines() error = %v", err)
	}

	expected := "line18\nline19\nline20"
	if content != expected {
		t.Errorf("CaptureLines() = %q, want %q", content, expected)
	}

	// Verify command: tmux capture-pane -t dev -p -S -3
	if len(mock.calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mock.calls))
	}
	call := mock.calls[0]
	expectedCmd := []string{"tmux", "capture-pane", "-t", "dev", "-p", "-S", "-3"}
	if len(call.cmd) != len(expectedCmd) {
		t.Fatalf("cmd = %v, want %v", call.cmd, expectedCmd)
	}
	for i, arg := range expectedCmd {
		if call.cmd[i] != arg {
			t.Errorf("cmd[%d] = %q, want %q", i, call.cmd[i], arg)
		}
	}
}

func TestClient_CaptureLines_Error(t *testing.T) {
	mock := newMockExec()
	mock.errors["container1:tmux"] = errors.New("exec failed")
	client := NewClient(mock.exec)

	_, err := client.CaptureLines(context.Background(), "container1", "dev", 20)
	if err == nil {
		t.Fatal("CaptureLines() expected error, got nil")
	}
}

func TestClient_CursorPosition_Success(t *testing.T) {
	mock := newMockExec()
	// history_size=100, cursor_y=15 → absolute position = 115
	mock.outputs["container1:tmux"] = "100 15\n"
	client := NewClient(mock.exec)

	pos, err := client.CursorPosition(context.Background(), "container1", "dev")
	if err != nil {
		t.Fatalf("CursorPosition() error = %v", err)
	}

	if pos != 115 {
		t.Errorf("CursorPosition() = %d, want 115", pos)
	}

	// Verify command
	if len(mock.calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mock.calls))
	}
	call := mock.calls[0]
	expectedCmd := []string{"tmux", "display-message", "-t", "dev", "-p", "#{history_size} #{cursor_y}"}
	if len(call.cmd) != len(expectedCmd) {
		t.Fatalf("cmd = %v, want %v", call.cmd, expectedCmd)
	}
	for i, arg := range expectedCmd {
		if call.cmd[i] != arg {
			t.Errorf("cmd[%d] = %q, want %q", i, call.cmd[i], arg)
		}
	}
}

func TestClient_CursorPosition_NoHistory(t *testing.T) {
	mock := newMockExec()
	// No scrollback yet: history_size=0, cursor_y=5
	mock.outputs["container1:tmux"] = "0 5\n"
	client := NewClient(mock.exec)

	pos, err := client.CursorPosition(context.Background(), "container1", "dev")
	if err != nil {
		t.Fatalf("CursorPosition() error = %v", err)
	}

	if pos != 5 {
		t.Errorf("CursorPosition() = %d, want 5", pos)
	}
}

func TestClient_CursorPosition_ParseError(t *testing.T) {
	mock := newMockExec()
	mock.outputs["container1:tmux"] = "invalid"
	client := NewClient(mock.exec)

	_, err := client.CursorPosition(context.Background(), "container1", "dev")
	if err == nil {
		t.Fatalf("CursorPosition() error = nil, want error")
	}

	if !strings.Contains(err.Error(), "unexpected cursor position output") {
		t.Errorf("error message = %q, want to contain 'unexpected cursor position output'", err.Error())
	}
}

func TestClient_SendKeys(t *testing.T) {
	mock := newMockExec()
	client := NewClient(mock.exec)

	err := client.SendKeys(context.Background(), "container1", "dev", "echo hello")
	if err != nil {
		t.Fatalf("SendKeys() error = %v", err)
	}

	if len(mock.calls) != 2 {
		t.Fatalf("Expected 2 calls, got %d", len(mock.calls))
	}

	// First call: send the text
	call := mock.calls[0]
	expectedCmd := []string{"tmux", "send-keys", "-t", "dev", "echo hello"}
	if len(call.cmd) != len(expectedCmd) {
		t.Fatalf("call[0] cmd = %v, want %v", call.cmd, expectedCmd)
	}
	for i, arg := range expectedCmd {
		if call.cmd[i] != arg {
			t.Errorf("call[0] cmd[%d] = %q, want %q", i, call.cmd[i], arg)
		}
	}

	// Second call: send Enter separately
	call = mock.calls[1]
	expectedCmd = []string{"tmux", "send-keys", "-t", "dev", "Enter"}
	if len(call.cmd) != len(expectedCmd) {
		t.Fatalf("call[1] cmd = %v, want %v", call.cmd, expectedCmd)
	}
	for i, arg := range expectedCmd {
		if call.cmd[i] != arg {
			t.Errorf("call[1] cmd[%d] = %q, want %q", i, call.cmd[i], arg)
		}
	}
}

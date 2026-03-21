package tui

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"devagent/internal/container"
)

func TestGenerateContainerActions_NilContainer(t *testing.T) {
	actions := GenerateContainerActions(nil, "/usr/bin/docker")
	if actions != nil {
		t.Errorf("expected nil for nil container, got %v", actions)
	}
}

func TestGenerateContainerActions_DefaultUser(t *testing.T) {
	c := &container.Container{
		Name:        "test-container",
		ProjectPath: "/home/user/project",
		RemoteUser:  "", // empty should use default
	}

	actions := GenerateContainerActions(c, "/usr/bin/docker")

	if len(actions) != 4 {
		t.Errorf("expected 4 actions, got %d", len(actions))
	}

	// Check that default user "vscode" is used
	for _, action := range actions {
		if strings.Contains(action.Command, "exec") && !strings.Contains(action.Command, "-u vscode") {
			t.Errorf("expected default user 'vscode' in exec command, got: %s", action.Command)
		}
	}
}

func TestGenerateContainerActions_CustomUser(t *testing.T) {
	c := &container.Container{
		Name:        "test-container",
		ProjectPath: "/home/user/project",
		RemoteUser:  "developer",
	}

	actions := GenerateContainerActions(c, "/usr/bin/docker")

	// Check that custom user is used
	for _, action := range actions {
		if strings.Contains(action.Command, "exec") && !strings.Contains(action.Command, "-u developer") {
			t.Errorf("expected custom user 'developer' in exec command, got: %s", action.Command)
		}
	}
}

func TestGenerateContainerActions_ContainsExpectedActions(t *testing.T) {
	c := &container.Container{
		Name:        "mycontainer",
		ProjectPath: "/projects/myapp",
		RemoteUser:  "vscode",
		ID:          "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
	}

	actions := GenerateContainerActions(c, "/usr/bin/docker")

	expectedLabels := []string{
		"Open in VS Code",
		"Create tmux session (named)",
		"Create tmux session (auto)",
		"Interactive shell",
	}

	if len(actions) != len(expectedLabels) {
		t.Fatalf("expected %d actions, got %d", len(expectedLabels), len(actions))
	}

	for i, expected := range expectedLabels {
		if actions[i].Label != expected {
			t.Errorf("action %d: expected label %q, got %q", i, expected, actions[i].Label)
		}
	}

	// Verify commands contain expected content
	if !strings.Contains(actions[0].Command, "code --folder-uri") {
		t.Errorf("VS Code command missing folder-uri: %s", actions[0].Command)
	}
	if !strings.Contains(actions[0].Command, "attached-container+") {
		t.Errorf("VS Code command should use attached-container+ scheme: %s", actions[0].Command)
	}
	if !strings.Contains(actions[1].Command, "tmux -u new-session -s mysession") {
		t.Errorf("named session command missing -u flag or session name: %s", actions[1].Command)
	}
	if !strings.Contains(actions[1].Command, "-e TERM=xterm-256color") {
		t.Errorf("named session command missing TERM env: %s", actions[1].Command)
	}
	if !strings.Contains(actions[1].Command, "-w /workspaces") {
		t.Errorf("named session command missing working directory: %s", actions[1].Command)
	}
	if !strings.Contains(actions[2].Command, "tmux -u new-session") || strings.Contains(actions[2].Command, "-s ") {
		t.Errorf("auto session command should have -u flag but not session name: %s", actions[2].Command)
	}
	if !strings.Contains(actions[2].Command, "-e COLORTERM=truecolor") {
		t.Errorf("auto session command missing COLORTERM env: %s", actions[2].Command)
	}
	if !strings.Contains(actions[2].Command, "-w /workspaces") {
		t.Errorf("auto session command missing working directory: %s", actions[2].Command)
	}
	if !strings.Contains(actions[3].Command, "/bin/bash") {
		t.Errorf("interactive shell command missing: %s", actions[3].Command)
	}
	if !strings.Contains(actions[3].Command, "-w /workspaces") {
		t.Errorf("interactive shell command missing working directory: %s", actions[3].Command)
	}
}

func TestGenerateVSCodeCommand(t *testing.T) {
	containerID := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	workspacePath := "/workspaces"
	cmd := GenerateVSCodeCommand(containerID, workspacePath)

	// Verify the command format
	if !strings.HasPrefix(cmd, "code --folder-uri vscode-remote://attached-container+") {
		t.Errorf("command should use attached-container+ scheme: %s", cmd)
	}

	// Verify the workspace path is appended
	if !strings.HasSuffix(cmd, workspacePath) {
		t.Errorf("command should end with workspace path: %s", cmd)
	}

	// Extract the hex portion from the URI and verify it decodes to correct JSON
	// URI format: vscode-remote://attached-container+<hexPayload><workspacePath>
	uriStart := strings.Index(cmd, "vscode-remote://attached-container+")
	if uriStart == -1 {
		t.Fatalf("command should contain vscode-remote URI: %s", cmd)
	}
	uriStart += len("vscode-remote://attached-container+")

	// Find where hex ends (before the workspace path)
	uriEnd := strings.LastIndex(cmd, workspacePath)
	if uriEnd == -1 {
		t.Fatalf("cannot find workspace path in command: %s", cmd)
	}

	hexPayload := cmd[uriStart:uriEnd]
	payload, err := hex.DecodeString(hexPayload)
	if err != nil {
		t.Fatalf("failed to decode hex payload: %v", err)
	}

	var decoded map[string]string
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("failed to unmarshal JSON payload: %v", err)
	}

	if decoded["containerName"] != containerID {
		t.Errorf("JSON payload should contain containerName=%q, got %v", containerID, decoded)
	}
}

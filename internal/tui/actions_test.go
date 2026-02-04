package tui

import (
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
	if !strings.Contains(actions[1].Command, "tmux -u new-session -s mysession") {
		t.Errorf("named session command missing -u flag or session name: %s", actions[1].Command)
	}
	if !strings.Contains(actions[2].Command, "tmux -u new-session") || strings.Contains(actions[2].Command, "-s ") {
		t.Errorf("auto session command should have -u flag but not session name: %s", actions[2].Command)
	}
	if !strings.Contains(actions[3].Command, "/bin/bash") {
		t.Errorf("interactive shell command missing: %s", actions[3].Command)
	}
}

func TestGenerateVSCodeCommand(t *testing.T) {
	cmd := GenerateVSCodeCommand("/home/user/project", "/workspaces")

	if !strings.HasPrefix(cmd, "code --folder-uri vscode-remote://dev-container+") {
		t.Errorf("unexpected command format: %s", cmd)
	}

	// The hex encoding of "/home/user/project" should be in the command
	// /home/user/project -> 2f686f6d652f757365722f70726f6a656374
	if !strings.Contains(cmd, "2f686f6d652f757365722f70726f6a656374") {
		t.Errorf("command should contain hex-encoded project path: %s", cmd)
	}

	if !strings.HasSuffix(cmd, "/workspaces") {
		t.Errorf("command should end with workspace path: %s", cmd)
	}
}

func TestGetVSCodePaletteInstructions(t *testing.T) {
	instructions := GetVSCodePaletteInstructions()

	if !strings.Contains(instructions, "Dev Containers: Attach to Running Container") {
		t.Errorf("instructions should mention attach command: %s", instructions)
	}

	if !strings.Contains(instructions, "Cmd+Shift+P") {
		t.Errorf("instructions should mention command palette shortcut: %s", instructions)
	}
}

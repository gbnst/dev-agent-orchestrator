package tmux

import (
	"testing"
)

func TestSession_AttachCommand(t *testing.T) {
	session := Session{
		Name:        "dev",
		ContainerID: "abc123def456",
	}

	got := session.AttachCommand("docker", "vscode")
	want := "docker exec -it -u vscode -e TERM=xterm-256color -e COLORTERM=truecolor abc123def456 tmux -u attach -t dev"

	if got != want {
		t.Errorf("AttachCommand() = %q, want %q", got, want)
	}
}

func TestSession_AttachCommand_Podman(t *testing.T) {
	session := Session{
		Name:        "main",
		ContainerID: "container123",
	}

	got := session.AttachCommand("podman", "vscode")
	want := "podman exec -it -u vscode -e TERM=xterm-256color -e COLORTERM=truecolor container123 tmux -u attach -t main"

	if got != want {
		t.Errorf("AttachCommand() = %q, want %q", got, want)
	}
}

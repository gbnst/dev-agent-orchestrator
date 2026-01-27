package tmux

import (
	"testing"
	"time"
)

func TestSession_AttachCommand(t *testing.T) {
	session := Session{
		Name:        "dev",
		ContainerID: "abc123def456",
	}

	got := session.AttachCommand("docker")
	want := "docker exec -it abc123def456 tmux attach -t dev"

	if got != want {
		t.Errorf("AttachCommand() = %q, want %q", got, want)
	}
}

func TestSession_AttachCommand_Podman(t *testing.T) {
	session := Session{
		Name:        "main",
		ContainerID: "container123",
	}

	got := session.AttachCommand("podman")
	want := "podman exec -it container123 tmux attach -t main"

	if got != want {
		t.Errorf("AttachCommand() = %q, want %q", got, want)
	}
}

func TestSession_IsActive(t *testing.T) {
	tests := []struct {
		name     string
		attached bool
		want     bool
	}{
		{"attached session is active", true, true},
		{"detached session is not active", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := Session{Attached: tt.attached}
			if got := session.IsActive(); got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSession_Age(t *testing.T) {
	session := Session{
		CreatedAt: time.Now().Add(-5 * time.Minute),
	}

	age := session.Age()
	if age < 4*time.Minute || age > 6*time.Minute {
		t.Errorf("Age() = %v, expected around 5 minutes", age)
	}
}

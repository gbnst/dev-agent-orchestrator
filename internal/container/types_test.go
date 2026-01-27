package container

import "testing"

func TestContainer_IsRunning(t *testing.T) {
	tests := []struct {
		name  string
		state ContainerState
		want  bool
	}{
		{"running", StateRunning, true},
		{"stopped", StateStopped, false},
		{"created", StateCreated, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Container{State: tt.state}
			if got := c.IsRunning(); got != tt.want {
				t.Errorf("IsRunning() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainer_IsStopped(t *testing.T) {
	tests := []struct {
		name  string
		state ContainerState
		want  bool
	}{
		{"stopped", StateStopped, true},
		{"running", StateRunning, false},
		{"created", StateCreated, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Container{State: tt.state}
			if got := c.IsStopped(); got != tt.want {
				t.Errorf("IsStopped() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainerState_Constants(t *testing.T) {
	// Verify state constants have expected string values
	if StateCreated != "created" {
		t.Errorf("StateCreated = %q, want %q", StateCreated, "created")
	}
	if StateRunning != "running" {
		t.Errorf("StateRunning = %q, want %q", StateRunning, "running")
	}
	if StateStopped != "stopped" {
		t.Errorf("StateStopped = %q, want %q", StateStopped, "stopped")
	}
}

func TestLabelConstants(t *testing.T) {
	// Verify label constants
	if LabelManagedBy != "devagent.managed" {
		t.Errorf("LabelManagedBy = %q", LabelManagedBy)
	}
	if LabelProjectPath != "devagent.project_path" {
		t.Errorf("LabelProjectPath = %q", LabelProjectPath)
	}
	if LabelTemplate != "devagent.template" {
		t.Errorf("LabelTemplate = %q", LabelTemplate)
	}
	if LabelAgent != "devagent.agent" {
		t.Errorf("LabelAgent = %q", LabelAgent)
	}
}

func TestContainer_HasSessions(t *testing.T) {
	tests := []struct {
		name     string
		sessions []Session
		want     bool
	}{
		{"no sessions", nil, false},
		{"empty sessions", []Session{}, false},
		{"one session", []Session{{Name: "dev"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Container{Sessions: tt.sessions}
			if got := c.HasSessions(); got != tt.want {
				t.Errorf("HasSessions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainer_SessionCount(t *testing.T) {
	tests := []struct {
		name     string
		sessions []Session
		want     int
	}{
		{"no sessions", nil, 0},
		{"one session", []Session{{Name: "dev"}}, 1},
		{"two sessions", []Session{{Name: "dev"}, {Name: "test"}}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Container{Sessions: tt.sessions}
			if got := c.SessionCount(); got != tt.want {
				t.Errorf("SessionCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

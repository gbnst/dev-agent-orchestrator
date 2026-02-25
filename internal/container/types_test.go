package container

import (
	"testing"
)

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

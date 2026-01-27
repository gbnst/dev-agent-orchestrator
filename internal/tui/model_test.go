package tui

import "testing"

func TestTabMode_String(t *testing.T) {
	tests := []struct {
		tab  TabMode
		want string
	}{
		{TabContainers, "Containers"},
		{TabSessions, "Sessions"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.tab.String(); got != tt.want {
				t.Errorf("TabMode.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestModel_HasTabs(t *testing.T) {
	// After this phase, Model should have currentTab field
	// This is a compile-time check more than runtime
	m := Model{
		currentTab:   TabContainers,
		logPanelOpen: false,
	}

	if m.currentTab != TabContainers {
		t.Errorf("currentTab = %v, want %v", m.currentTab, TabContainers)
	}
}

func TestStatusLevel_String(t *testing.T) {
	tests := []struct {
		level StatusLevel
		want  string
	}{
		{StatusInfo, "info"},
		{StatusSuccess, "success"},
		{StatusError, "error"},
		{StatusLoading, "loading"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("StatusLevel.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestModel_PendingOperations(t *testing.T) {
	m := newTestModel()

	// Initially empty
	if len(m.pendingOperations) != 0 {
		t.Error("pendingOperations should be empty initially")
	}

	// Can add pending operation
	m.setPending("abc123", "start")
	if op, ok := m.pendingOperations["abc123"]; !ok || op != "start" {
		t.Errorf("pendingOperations[abc123] = %q, want 'start'", op)
	}

	// Can check if pending
	if !m.isPending("abc123") {
		t.Error("abc123 should be pending")
	}
	if m.isPending("xyz789") {
		t.Error("xyz789 should not be pending")
	}

	// Can clear pending operation
	m.clearPending("abc123")
	if m.isPending("abc123") {
		t.Error("abc123 should not be pending after clear")
	}
}

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

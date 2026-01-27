package tui

import (
	"strings"
	"testing"
)

func TestRenderTabs(t *testing.T) {
	styles := NewStyles("mocha")

	tests := []struct {
		name       string
		currentTab TabMode
		width      int
		wantActive string
	}{
		{
			name:       "containers tab active",
			currentTab: TabContainers,
			width:      80,
			wantActive: "1 Containers",
		},
		{
			name:       "sessions tab active",
			currentTab: TabSessions,
			width:      80,
			wantActive: "2 Sessions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderTabs(tt.currentTab, tt.width, styles)

			if !strings.Contains(result, tt.wantActive) {
				t.Errorf("renderTabs() should contain %q, got %q", tt.wantActive, result)
			}
			if !strings.Contains(result, "1 Containers") {
				t.Errorf("renderTabs() should always contain '1 Containers'")
			}
			if !strings.Contains(result, "2 Sessions") {
				t.Errorf("renderTabs() should always contain '2 Sessions'")
			}
		})
	}
}

func TestRenderTabs_FillsWidth(t *testing.T) {
	styles := NewStyles("mocha")

	result := renderTabs(TabContainers, 80, styles)
	// Should contain gap fill characters
	if !strings.Contains(result, "─") {
		t.Error("renderTabs() should contain gap fill characters")
	}
}

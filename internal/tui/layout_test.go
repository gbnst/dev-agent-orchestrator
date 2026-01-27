package tui

import "testing"

func TestComputeLayout(t *testing.T) {
	tests := []struct {
		name          string
		width         int
		height        int
		logPanelOpen  bool
		wantHeader    Region
		wantContent   int // content height
		wantLogHeight int // 0 if closed
	}{
		{
			name:         "standard terminal no logs",
			width:        80,
			height:       24,
			logPanelOpen: false,
			wantHeader:   Region{X: 0, Y: 0, Width: 80, Height: 2},
			wantContent:  18, // 24 - 6 fixed lines
		},
		{
			name:         "large terminal no logs",
			width:        120,
			height:       40,
			logPanelOpen: false,
			wantHeader:   Region{X: 0, Y: 0, Width: 120, Height: 2},
			wantContent:  34, // 40 - 6 fixed lines
		},
		{
			name:          "standard terminal with logs",
			width:         80,
			height:        24,
			logPanelOpen:  true,
			wantContent:   7,  // 40% of (24-6) = 7.2 -> 7
			wantLogHeight: 11, // 60% of (24-6) = 10.8 -> 11
		},
		{
			name:         "minimum height",
			width:        80,
			height:       10,
			logPanelOpen: false,
			wantContent:  4, // 10 - 6 fixed lines
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := ComputeLayout(tt.width, tt.height, tt.logPanelOpen)

			if layout.Header.Width != tt.width {
				t.Errorf("Header.Width = %d, want %d", layout.Header.Width, tt.width)
			}
			if layout.Header.Height != 2 {
				t.Errorf("Header.Height = %d, want 2", layout.Header.Height)
			}
			if layout.Tabs.Height != 1 {
				t.Errorf("Tabs.Height = %d, want 1", layout.Tabs.Height)
			}
			if layout.Content.Height != tt.wantContent {
				t.Errorf("Content.Height = %d, want %d", layout.Content.Height, tt.wantContent)
			}
			if tt.logPanelOpen {
				if layout.Logs.Height != tt.wantLogHeight {
					t.Errorf("Logs.Height = %d, want %d", layout.Logs.Height, tt.wantLogHeight)
				}
			}
			if layout.StatusBar.Height != 1 {
				t.Errorf("StatusBar.Height = %d, want 1", layout.StatusBar.Height)
			}
		})
	}
}

func TestLayout_TotalHeight(t *testing.T) {
	tests := []struct {
		name         string
		width        int
		height       int
		logPanelOpen bool
	}{
		{"no logs", 80, 24, false},
		{"with logs", 80, 24, true},
		{"large", 120, 40, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := ComputeLayout(tt.width, tt.height, tt.logPanelOpen)

			// Total of all regions should equal terminal height
			total := layout.Header.Height + layout.Tabs.Height + layout.Content.Height
			if tt.logPanelOpen {
				total += layout.Logs.Height + 1 // +1 for separator
			}
			total += layout.StatusBar.Height

			// Allow 1-2 line variance for margins
			if total < tt.height-2 || total > tt.height {
				t.Errorf("total height = %d, want ~%d", total, tt.height)
			}
		})
	}
}

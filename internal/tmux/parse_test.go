package tmux

import (
	"testing"
)

func TestParseListSessions_BasicSessions(t *testing.T) {
	tests := []struct {
		name        string
		containerID string
		output      string
		wantCount   int
		wantFirst   Session
		wantSecond  Session
	}{
		{
			name:        "single session",
			containerID: "container1",
			output:      "dev: 2 windows (created Mon Jan 20 10:00:00 2025)",
			wantCount:   1,
			wantFirst: Session{
				Name:        "dev",
				Windows:     2,
				Attached:    false,
				ContainerID: "container1",
			},
		},
		{
			name:        "multiple sessions",
			containerID: "container1",
			output: `dev: 2 windows (created Mon Jan 20 10:00:00 2025)
main: 1 windows (created Mon Jan 20 09:00:00 2025) (attached)`,
			wantCount: 2,
			wantFirst: Session{
				Name:        "dev",
				Windows:     2,
				Attached:    false,
				ContainerID: "container1",
			},
			wantSecond: Session{
				Name:        "main",
				Windows:     1,
				Attached:    true,
				ContainerID: "container1",
			},
		},
		{
			name:        "session with attached flag",
			containerID: "container1",
			output:      "work: 3 windows (created Mon Jan 20 11:00:00 2025) (attached)",
			wantCount:   1,
			wantFirst: Session{
				Name:        "work",
				Windows:     3,
				Attached:    true,
				ContainerID: "container1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseListSessions(tt.containerID, tt.output)
			if len(got) != tt.wantCount {
				t.Fatalf("got %d sessions, want %d", len(got), tt.wantCount)
			}

			if len(got) > 0 {
				if got[0].Name != tt.wantFirst.Name {
					t.Errorf("session[0].Name = %q, want %q", got[0].Name, tt.wantFirst.Name)
				}
				if got[0].Windows != tt.wantFirst.Windows {
					t.Errorf("session[0].Windows = %d, want %d", got[0].Windows, tt.wantFirst.Windows)
				}
				if got[0].Attached != tt.wantFirst.Attached {
					t.Errorf("session[0].Attached = %v, want %v", got[0].Attached, tt.wantFirst.Attached)
				}
				if got[0].ContainerID != tt.wantFirst.ContainerID {
					t.Errorf("session[0].ContainerID = %q, want %q", got[0].ContainerID, tt.wantFirst.ContainerID)
				}
			}

			if len(got) > 1 {
				if got[1].Name != tt.wantSecond.Name {
					t.Errorf("session[1].Name = %q, want %q", got[1].Name, tt.wantSecond.Name)
				}
				if got[1].Windows != tt.wantSecond.Windows {
					t.Errorf("session[1].Windows = %d, want %d", got[1].Windows, tt.wantSecond.Windows)
				}
				if got[1].Attached != tt.wantSecond.Attached {
					t.Errorf("session[1].Attached = %v, want %v", got[1].Attached, tt.wantSecond.Attached)
				}
				if got[1].ContainerID != tt.wantSecond.ContainerID {
					t.Errorf("session[1].ContainerID = %q, want %q", got[1].ContainerID, tt.wantSecond.ContainerID)
				}
			}
		})
	}
}

func TestParseListSessions_EmptyInput(t *testing.T) {
	tests := []struct {
		name        string
		containerID string
		output      string
	}{
		{
			name:        "empty string",
			containerID: "container1",
			output:      "",
		},
		{
			name:        "only whitespace",
			containerID: "container1",
			output:      "   \n  \n",
		},
		{
			name:        "only blank lines",
			containerID: "container1",
			output:      "\n\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseListSessions(tt.containerID, tt.output)
			if len(got) != 0 {
				t.Fatalf("got %d sessions, want 0", len(got))
			}
		})
	}
}

func TestParseListSessions_MalformedLines(t *testing.T) {
	tests := []struct {
		name        string
		containerID string
		output      string
		wantCount   int
		description string
	}{
		{
			name:        "missing colon separator",
			containerID: "container1",
			output:      "dev 2 windows (created Mon Jan 20 10:00:00 2025)",
			wantCount:   0,
			description: "line without ': ' separator is skipped",
		},
		{
			name:        "malformed line without window count",
			containerID: "container1",
			output:      "dev: (created Mon Jan 20 10:00:00 2025)",
			wantCount:   1,
			description: "line has valid session name 'dev' even without proper 'N windows' syntax",
		},
		{
			name:        "mixed valid and invalid",
			containerID: "container1",
			output: `dev: 2 windows (created Mon Jan 20 10:00:00 2025)
invalid-line-without-colon
main: 1 windows (created Mon Jan 20 09:00:00 2025)`,
			wantCount:   2,
			description: "valid lines are parsed, invalid lines skipped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseListSessions(tt.containerID, tt.output)
			if len(got) != tt.wantCount {
				t.Fatalf("got %d sessions (want %d): %s", len(got), tt.wantCount, tt.description)
			}
		})
	}
}

func TestParseListSessions_SessionNamesWithColons(t *testing.T) {
	// tmux session names can contain colons (except as the final separator before windows count)
	output := `my:session: 2 windows (created Mon Jan 20 10:00:00 2025)
work:dev: 1 windows (created Mon Jan 20 09:00:00 2025)`

	got := ParseListSessions("container1", output)
	if len(got) != 2 {
		t.Fatalf("got %d sessions, want 2", len(got))
	}

	if got[0].Name != "my:session" {
		t.Errorf("session[0].Name = %q, want %q", got[0].Name, "my:session")
	}
	if got[1].Name != "work:dev" {
		t.Errorf("session[1].Name = %q, want %q", got[1].Name, "work:dev")
	}
}

func TestParseListSessions_TrailingWhitespace(t *testing.T) {
	output := `  dev: 2 windows (created Mon Jan 20 10:00:00 2025)
  main: 1 windows (created Mon Jan 20 09:00:00 2025)  `

	got := ParseListSessions("container1", output)
	if len(got) != 2 {
		t.Fatalf("got %d sessions, want 2", len(got))
	}

	if got[0].Name != "dev" {
		t.Errorf("session[0].Name = %q, want %q", got[0].Name, "dev")
	}
	if got[1].Name != "main" {
		t.Errorf("session[1].Name = %q, want %q", got[1].Name, "main")
	}
}

func TestParseListSessions_WindowsCountVariations(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		wantWindowsIdx int
		wantWindows    int
	}{
		{
			name:           "single window",
			output:         "dev: 1 window (created Mon Jan 20 10:00:00 2025)",
			wantWindowsIdx: 0,
			wantWindows:    1,
		},
		{
			name:           "multiple windows",
			output:         "dev: 5 windows (created Mon Jan 20 10:00:00 2025)",
			wantWindowsIdx: 0,
			wantWindows:    5,
		},
		{
			name:           "zero windows edge case",
			output:         "dev: 0 windows (created Mon Jan 20 10:00:00 2025)",
			wantWindowsIdx: 0,
			wantWindows:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseListSessions("container1", tt.output)
			if len(got) == 0 {
				t.Fatalf("got 0 sessions, want at least 1")
			}
			if got[tt.wantWindowsIdx].Windows != tt.wantWindows {
				t.Errorf("Windows = %d, want %d", got[tt.wantWindowsIdx].Windows, tt.wantWindows)
			}
		})
	}
}

func TestParseListSessions_NoContainerID(t *testing.T) {
	// Test that empty containerID is handled correctly (for host sessions)
	output := `dev: 2 windows (created Mon Jan 20 10:00:00 2025)
main: 1 windows (created Mon Jan 20 09:00:00 2025)`

	got := ParseListSessions("", output)
	if len(got) != 2 {
		t.Fatalf("got %d sessions, want 2", len(got))
	}

	for i, session := range got {
		if session.ContainerID != "" {
			t.Errorf("session[%d].ContainerID = %q, want empty string", i, session.ContainerID)
		}
	}
}

func TestParseListSessions_RealWorldOutput(t *testing.T) {
	// Real tmux list-sessions output with various formats
	output := `dev: 2 windows (created Mon Feb 24 10:00:00 2026) (attached)
main: 1 window (created Mon Feb 24 09:00:00 2026)
work:project: 3 windows (created Tue Feb 25 08:30:00 2026)
test_env: 5 windows (created Tue Feb 25 11:15:00 2026) (attached)`

	got := ParseListSessions("container1", output)
	if len(got) != 4 {
		t.Fatalf("got %d sessions, want 4", len(got))
	}

	expectedNames := []string{"dev", "main", "work:project", "test_env"}
	expectedAttached := []bool{true, false, false, true}
	expectedWindows := []int{2, 1, 3, 5}

	for i := 0; i < len(got); i++ {
		if got[i].Name != expectedNames[i] {
			t.Errorf("session[%d].Name = %q, want %q", i, got[i].Name, expectedNames[i])
		}
		if got[i].Attached != expectedAttached[i] {
			t.Errorf("session[%d].Attached = %v, want %v", i, got[i].Attached, expectedAttached[i])
		}
		if got[i].Windows != expectedWindows[i] {
			t.Errorf("session[%d].Windows = %d, want %d", i, got[i].Windows, expectedWindows[i])
		}
		if got[i].ContainerID != "container1" {
			t.Errorf("session[%d].ContainerID = %q, want %q", i, got[i].ContainerID, "container1")
		}
	}
}

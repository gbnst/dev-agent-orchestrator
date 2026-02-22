package web

import (
	"testing"
)

func TestParseHostSessions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []SessionResponse
	}{
		{
			name:  "empty output",
			input: "",
			want:  nil,
		},
		{
			name:  "single session",
			input: "main: 1 window (created Sat Feb 22 10:00:00 2026)\n",
			want: []SessionResponse{
				{Name: "main", Windows: 1, Attached: false},
			},
		},
		{
			name:  "multiple sessions with attached",
			input: "dev: 3 windows (created Sat Feb 22 10:00:00 2026) (attached)\nwork: 1 window (created Sat Feb 22 11:00:00 2026)\n",
			want: []SessionResponse{
				{Name: "dev", Windows: 3, Attached: true},
				{Name: "work", Windows: 1, Attached: false},
			},
		},
		{
			name:  "session name with colons",
			input: "my:session: 2 windows (created Sat Feb 22 10:00:00 2026)\n",
			want: []SessionResponse{
				{Name: "my:session", Windows: 2, Attached: false},
			},
		},
		{
			name:  "trailing whitespace and blank lines",
			input: "\n  main: 1 window (created Sat Feb 22 10:00:00 2026)  \n\n",
			want: []SessionResponse{
				{Name: "main", Windows: 1, Attached: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseHostSessions(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d sessions, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].Name != tt.want[i].Name {
					t.Errorf("session[%d].Name = %q, want %q", i, got[i].Name, tt.want[i].Name)
				}
				if got[i].Windows != tt.want[i].Windows {
					t.Errorf("session[%d].Windows = %d, want %d", i, got[i].Windows, tt.want[i].Windows)
				}
				if got[i].Attached != tt.want[i].Attached {
					t.Errorf("session[%d].Attached = %v, want %v", i, got[i].Attached, tt.want[i].Attached)
				}
			}
		})
	}
}

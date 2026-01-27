package logging

import (
	"testing"
	"time"
)

func TestLogEntry_String(t *testing.T) {
	tests := []struct {
		name     string
		entry    LogEntry
		contains []string
	}{
		{
			name: "basic entry",
			entry: LogEntry{
				Timestamp: time.Date(2025, 1, 27, 10, 30, 0, 0, time.UTC),
				Level:     "INFO",
				Scope:     "app",
				Message:   "application started",
			},
			contains: []string{"10:30:00", "INFO", "app", "application started"},
		},
		{
			name: "entry with fields",
			entry: LogEntry{
				Timestamp: time.Date(2025, 1, 27, 10, 30, 0, 0, time.UTC),
				Level:     "ERROR",
				Scope:     "container.abc123",
				Message:   "operation failed",
				Fields:    map[string]any{"error": "connection refused"},
			},
			contains: []string{"ERROR", "container.abc123", "operation failed", "connection refused"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.entry.String()
			for _, want := range tt.contains {
				if !containsString(got, want) {
					t.Errorf("String() = %q, should contain %q", got, want)
				}
			}
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"debug", "DEBUG"},
		{"DEBUG", "DEBUG"},
		{"info", "INFO"},
		{"INFO", "INFO"},
		{"warn", "WARN"},
		{"warning", "WARN"},
		{"error", "ERROR"},
		{"ERROR", "ERROR"},
		{"unknown", "INFO"},
		{"", "INFO"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseLevel(tt.input)
			if got != tt.want {
				t.Errorf("ParseLevel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

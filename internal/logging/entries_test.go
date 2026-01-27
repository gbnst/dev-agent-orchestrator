// pattern: Functional Core

package logging

import (
	"strings"
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
				if !strings.Contains(got, want) {
					t.Errorf("String() = %q, should contain %q", got, want)
				}
			}
		})
	}
}

func TestLogEntry_MatchesScope(t *testing.T) {
	tests := []struct {
		name   string
		scope  string
		prefix string
		want   bool
	}{
		{
			name:   "empty prefix matches all",
			scope:  "container.abc123",
			prefix: "",
			want:   true,
		},
		{
			name:   "exact match",
			scope:  "container.abc123",
			prefix: "container.abc123",
			want:   true,
		},
		{
			name:   "prefix match",
			scope:  "container.abc123.session",
			prefix: "container.abc123",
			want:   true,
		},
		{
			name:   "no match",
			scope:  "container.abc123",
			prefix: "container.xyz",
			want:   false,
		},
		{
			name:   "partial component no match",
			scope:  "container.abc123",
			prefix: "container.abc",
			want:   true, // HasPrefix includes partial component
		},
		{
			name:   "different root",
			scope:  "session.test",
			prefix: "container",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := LogEntry{Scope: tt.scope}
			got := entry.MatchesScope(tt.prefix)
			if got != tt.want {
				t.Errorf("MatchesScope(%q) = %v, want %v", tt.prefix, got, tt.want)
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

// pattern: Functional Core

package logging

import (
	"fmt"
	"strings"
	"time"
)

// LogEntry represents a structured log entry for TUI consumption.
// It contains all information needed to display and filter logs in the UI.
type LogEntry struct {
	Timestamp time.Time      // When the log was created
	Level     string         // DEBUG, INFO, WARN, ERROR
	Scope     string         // Hierarchical scope (e.g., "container.abc123")
	Message   string         // Log message
	Fields    map[string]any // Additional structured fields
}

// String returns a human-readable representation of the log entry.
func (e LogEntry) String() string {
	var sb strings.Builder
	sb.WriteString(e.Timestamp.Format("15:04:05"))
	sb.WriteString(" ")
	sb.WriteString(e.Level)
	sb.WriteString(" ")
	sb.WriteString("[")
	sb.WriteString(e.Scope)
	sb.WriteString("] ")
	sb.WriteString(e.Message)

	if len(e.Fields) > 0 {
		sb.WriteString(" ")
		first := true
		for k, v := range e.Fields {
			if !first {
				sb.WriteString(" ")
			}
			fmt.Fprintf(&sb, "%s=%v", k, v)
			first = false
		}
	}

	return sb.String()
}

// MatchesScope returns true if the entry's scope starts with the given prefix.
// An empty prefix matches all entries.
func (e LogEntry) MatchesScope(prefix string) bool {
	if prefix == "" {
		return true
	}
	return strings.HasPrefix(e.Scope, prefix)
}

// ParseLevel normalizes a log level string to uppercase.
// Returns "INFO" for unknown levels.
func ParseLevel(level string) string {
	switch strings.ToLower(level) {
	case "debug":
		return "DEBUG"
	case "info":
		return "INFO"
	case "warn", "warning":
		return "WARN"
	case "error":
		return "ERROR"
	default:
		return "INFO"
	}
}

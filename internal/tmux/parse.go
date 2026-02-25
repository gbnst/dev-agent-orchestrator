// pattern: Functional Core

package tmux

import (
	"bufio"
	"strconv"
	"strings"
)

// ParseListSessions parses tmux list-sessions output into a slice of Session objects.
// The output format is: "name: N windows (created DATE) [(attached)]"
// The containerID parameter is populated into each returned Session.
// Empty lines and malformed lines are skipped gracefully.
func ParseListSessions(containerID, output string) []Session {
	var sessions []Session

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		session := parseSessionLine(containerID, line)
		if session.Name != "" {
			sessions = append(sessions, session)
		}
	}

	return sessions
}

// parseSessionLine parses a single line from tmux list-sessions output.
func parseSessionLine(containerID, line string) Session {
	session := Session{ContainerID: containerID}

	// Split on ": " to get name
	parts := strings.SplitN(line, ": ", 2)
	if len(parts) < 2 {
		return session
	}
	session.Name = parts[0]

	// Parse windows count
	// tmux uses "1 window" (singular) or "N windows" (plural)
	rest := parts[1]
	// Try to find "window" or "windows" to extract the count before it
	windowIdx := strings.Index(rest, " window")
	if windowIdx > 0 {
		if n, err := strconv.Atoi(rest[:windowIdx]); err == nil {
			session.Windows = n
		}
	}

	// Check if attached
	session.Attached = strings.Contains(line, "(attached)")

	return session
}

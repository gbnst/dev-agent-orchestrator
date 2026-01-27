package tmux

import (
	"bufio"
	"context"
	"strconv"
	"strings"
)

// ContainerExecutor executes commands inside a container.
type ContainerExecutor func(ctx context.Context, containerID string, cmd []string) (string, error)

// Client wraps tmux commands executed inside containers.
type Client struct {
	exec ContainerExecutor
}

// NewClient creates a new tmux Client.
func NewClient(exec ContainerExecutor) *Client {
	return &Client{exec: exec}
}

// NewClientWithExecutor creates a new Client with the given executor (for testing).
func NewClientWithExecutor(exec ContainerExecutor) *Client {
	return &Client{exec: exec}
}

// ListSessions returns all tmux sessions in the container.
func (c *Client) ListSessions(ctx context.Context, containerID string) ([]Session, error) {
	output, err := c.exec(ctx, containerID, []string{"tmux", "list-sessions"})
	if err != nil {
		// No server running = no sessions (not an error)
		return []Session{}, nil
	}

	return c.parseSessions(containerID, output), nil
}

// parseSessions parses tmux list-sessions output.
// Format: "name: N windows (created DATE) [(attached)]"
func (c *Client) parseSessions(containerID, output string) []Session {
	var sessions []Session

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		session := c.parseSessionLine(containerID, line)
		if session.Name != "" {
			sessions = append(sessions, session)
		}
	}

	return sessions
}

// parseSessionLine parses a single line from list-sessions.
func (c *Client) parseSessionLine(containerID, line string) Session {
	session := Session{ContainerID: containerID}

	// Split on ": " to get name
	parts := strings.SplitN(line, ": ", 2)
	if len(parts) < 2 {
		return session
	}
	session.Name = parts[0]

	// Parse windows count
	rest := parts[1]
	if idx := strings.Index(rest, " windows"); idx > 0 {
		if n, err := strconv.Atoi(rest[:idx]); err == nil {
			session.Windows = n
		}
	}

	// Check if attached
	session.Attached = strings.Contains(line, "(attached)")

	return session
}

// CreateSession creates a new detached tmux session.
func (c *Client) CreateSession(ctx context.Context, containerID, name string) error {
	_, err := c.exec(ctx, containerID, []string{"tmux", "new-session", "-d", "-s", name})
	return err
}

// KillSession destroys a tmux session.
func (c *Client) KillSession(ctx context.Context, containerID, name string) error {
	_, err := c.exec(ctx, containerID, []string{"tmux", "kill-session", "-t", name})
	return err
}

// CapturePane captures the content of a session's active pane.
func (c *Client) CapturePane(ctx context.Context, containerID, session string) (string, error) {
	return c.exec(ctx, containerID, []string{"tmux", "capture-pane", "-t", session, "-p"})
}

// SendKeys sends keys to a tmux session, followed by Enter.
func (c *Client) SendKeys(ctx context.Context, containerID, session, keys string) error {
	_, err := c.exec(ctx, containerID, []string{"tmux", "send-keys", "-t", session, keys, "Enter"})
	return err
}

// pattern: Imperative Shell

package tmux

import (
	"bufio"
	"context"
	"strconv"
	"strings"

	"devagent/internal/logging"
)

// ContainerExecutor executes commands inside a container.
type ContainerExecutor func(ctx context.Context, containerID string, cmd []string) (string, error)

// Client wraps tmux commands executed inside containers.
type Client struct {
	exec   ContainerExecutor
	logger *logging.ScopedLogger
}

// NewClient creates a new tmux Client.
func NewClient(exec ContainerExecutor) *Client {
	return &Client{
		exec:   exec,
		logger: logging.NopLogger(),
	}
}

// NewClientWithExecutor creates a new Client with the given executor (for testing).
func NewClientWithExecutor(exec ContainerExecutor) *Client {
	return &Client{
		exec:   exec,
		logger: logging.NopLogger(),
	}
}

// NewClientWithLogger creates a new tmux Client with logging support.
func NewClientWithLogger(exec ContainerExecutor, logManager logging.LoggerProvider) *Client {
	logger := logManager.For("tmux")
	logger.Debug("tmux client initialized")

	return &Client{
		exec:   exec,
		logger: logger,
	}
}

// ListSessions returns all tmux sessions in the container.
func (c *Client) ListSessions(ctx context.Context, containerID string) ([]Session, error) {
	c.logger.Debug("listing tmux sessions", "containerID", containerID)

	output, err := c.exec(ctx, containerID, []string{"tmux", "list-sessions"})
	if err != nil {
		// No server running = no sessions (not an error)
		c.logger.Debug("no tmux server running", "containerID", containerID, "error", err)
		return []Session{}, nil
	}

	sessions := c.parseSessions(containerID, output)
	c.logger.Debug("sessions listed", "containerID", containerID, "count", len(sessions))
	return sessions, nil
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
	c.logger.Info("creating tmux session", "containerID", containerID, "session", name)

	_, err := c.exec(ctx, containerID, []string{"tmux", "new-session", "-d", "-s", name})
	if err != nil {
		c.logger.Error("failed to create session", "containerID", containerID, "session", name, "error", err)
		return err
	}

	c.logger.Info("session created", "containerID", containerID, "session", name)
	return nil
}

// KillSession destroys a tmux session.
func (c *Client) KillSession(ctx context.Context, containerID, name string) error {
	c.logger.Info("killing tmux session", "containerID", containerID, "session", name)

	_, err := c.exec(ctx, containerID, []string{"tmux", "kill-session", "-t", name})
	if err != nil {
		c.logger.Error("failed to kill session", "containerID", containerID, "session", name, "error", err)
		return err
	}

	c.logger.Info("session killed", "containerID", containerID, "session", name)
	return nil
}

// CapturePane captures the content of a session's active pane.
func (c *Client) CapturePane(ctx context.Context, containerID, session string) (string, error) {
	c.logger.Debug("capturing pane", "containerID", containerID, "session", session)

	output, err := c.exec(ctx, containerID, []string{"tmux", "capture-pane", "-t", session, "-p"})
	if err != nil {
		c.logger.Error("failed to capture pane", "containerID", containerID, "session", session, "error", err)
		return "", err
	}

	return output, nil
}

// SendKeys sends keys to a tmux session, followed by Enter.
func (c *Client) SendKeys(ctx context.Context, containerID, session, keys string) error {
	c.logger.Debug("sending keys", "containerID", containerID, "session", session)

	_, err := c.exec(ctx, containerID, []string{"tmux", "send-keys", "-t", session, keys, "Enter"})
	if err != nil {
		c.logger.Error("failed to send keys", "containerID", containerID, "session", session, "error", err)
		return err
	}

	return nil
}

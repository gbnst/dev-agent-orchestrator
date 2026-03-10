// pattern: Imperative Shell

package tmux

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"devagent/internal/logging"
)

// ContainerExecutor executes commands inside a container.
type ContainerExecutor func(ctx context.Context, containerID string, cmd []string) (string, error)

// CaptureOpts configures how CapturePane captures pane content.
type CaptureOpts struct {
	Lines      int // Limit output to last N lines (applied in Go after capture). 0 = no limit.
	FromCursor int // Absolute cursor position to capture from (history_size + cursor_y). -1 = disabled.
}

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

	sessions := ParseListSessions(containerID, output)
	c.logger.Debug("sessions listed", "containerID", containerID, "count", len(sessions))
	return sessions, nil
}

// CreateSession creates a new detached tmux session.
func (c *Client) CreateSession(ctx context.Context, containerID, name string) error {
	c.logger.Info("creating tmux session", "containerID", containerID, "session", name)

	_, err := c.exec(ctx, containerID, []string{"tmux", "-u", "new-session", "-d", "-s", name})
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
// When FromCursor is set (>= 0), it captures output since that absolute position
// by reaching into scrollback history as needed.
func (c *Client) CapturePane(ctx context.Context, containerID, session string, opts CaptureOpts) (string, error) {
	c.logger.Debug("capturing pane", "containerID", containerID, "session", session, "opts", opts)

	cmd := []string{"tmux", "capture-pane", "-t", session, "-p"}
	if opts.Lines <= 0 && opts.FromCursor >= 0 {
		// Get current absolute position to compute scrollback offset
		currentPos, err := c.CursorPosition(ctx, containerID, session)
		if err != nil {
			c.logger.Error("failed to get cursor position for from_cursor capture", "error", err)
			return "", err
		}
		// Reach back into scrollback: negative value means lines above visible pane top
		linesBack := currentPos - opts.FromCursor
		if linesBack > 0 {
			cmd = append(cmd, "-S", "-"+strconv.Itoa(linesBack))
		}
		// If linesBack <= 0, no new output since last cursor — capture visible pane
	}

	output, err := c.exec(ctx, containerID, cmd)
	if err != nil {
		c.logger.Error("failed to capture pane", "containerID", containerID, "session", session, "error", err)
		return "", err
	}

	// tmux capture-pane returns the entire visible pane height, padding with
	// blank lines below the cursor. Trim those trailing empty lines.
	output = trimTrailingEmptyLines(output)

	// Limit to last N lines if requested
	if opts.Lines > 0 {
		output = lastNLines(output, opts.Lines)
	}

	return output, nil
}

// trimTrailingEmptyLines removes trailing empty/whitespace-only lines from
// tmux capture-pane output. Tmux pads capture-pane output with blank lines
// to fill the full visible pane height; this trims those padding lines.
func trimTrailingEmptyLines(s string) string {
	lines := strings.Split(s, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

// lastNLines returns the last n lines from s. If s has n or fewer lines, returns s unchanged.
func lastNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// CaptureLines captures the last N lines from a session's scrollback history.
// Unlike CapturePane (which captures the visible pane), this reaches into
// scrollback using `tmux capture-pane -S -N` to return historical output.
func (c *Client) CaptureLines(ctx context.Context, containerID, session string, lines int) (string, error) {
	c.logger.Debug("capturing lines", "containerID", containerID, "session", session, "lines", lines)

	cmd := []string{"tmux", "capture-pane", "-t", session, "-p", "-S", "-" + strconv.Itoa(lines)}
	output, err := c.exec(ctx, containerID, cmd)
	if err != nil {
		c.logger.Error("failed to capture lines", "containerID", containerID, "session", session, "error", err)
		return "", err
	}

	output = trimTrailingEmptyLines(output)
	return output, nil
}

// CursorPosition returns the absolute cursor position for a tmux session.
// The absolute position is history_size + cursor_y, which increases monotonically
// as output scrolls, making it safe for cursor-based polling (unlike cursor_y alone,
// which wraps at the pane height).
func (c *Client) CursorPosition(ctx context.Context, containerID, session string) (int, error) {
	c.logger.Debug("getting cursor position", "containerID", containerID, "session", session)

	output, err := c.exec(ctx, containerID, []string{
		"tmux", "display-message", "-t", session, "-p", "#{history_size} #{cursor_y}",
	})
	if err != nil {
		c.logger.Error("failed to get cursor position", "containerID", containerID, "session", session, "error", err)
		return 0, err
	}

	parts := strings.Fields(strings.TrimSpace(output))
	if len(parts) != 2 {
		return 0, fmt.Errorf("unexpected cursor position output %q", output)
	}

	historySize, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("failed to parse history_size from %q: %w", output, err)
	}

	cursorY, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("failed to parse cursor_y from %q: %w", output, err)
	}

	return historySize + cursorY, nil
}

// SendKeys sends keys to a tmux session, followed by Enter.
func (c *Client) SendKeys(ctx context.Context, containerID, session, keys string) error {
	c.logger.Debug("sending keys", "containerID", containerID, "session", session)

	// Send the text first
	_, err := c.exec(ctx, containerID, []string{"tmux", "send-keys", "-t", session, keys})
	if err != nil {
		c.logger.Error("failed to send keys", "containerID", containerID, "session", session, "error", err)
		return err
	}
	// Send Enter separately — TUI apps (e.g. Claude Code) need this gap
	// to distinguish "submit" from "newline within input"
	_, err = c.exec(ctx, containerID, []string{"tmux", "send-keys", "-t", session, "Enter"})
	if err != nil {
		c.logger.Error("failed to send Enter", "containerID", containerID, "session", session, "error", err)
		return err
	}

	return nil
}

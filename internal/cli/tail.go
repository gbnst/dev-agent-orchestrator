// pattern: Imperative Shell
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"devagent/internal/instance"
)

// TailConfig configures the tail polling behavior.
type TailConfig struct {
	ContainerID string
	Session     string
	Interval    time.Duration
	NoColor     bool
	Writer      io.Writer
	ErrWriter   io.Writer
}

// TailSession polls a tmux session and streams new output to the writer.
// It blocks until the context is cancelled or an unrecoverable error occurs.
// Returns nil on clean exit (context cancel, session destroyed, container stopped).
// Returns error on TUI connection failure after retry.
func TailSession(ctx context.Context, client *instance.Client, cfg TailConfig) error {
	// Initial capture
	data, err := client.ReadSession(cfg.ContainerID, cfg.Session, 10)
	if err != nil {
		return err
	}

	var response struct {
		Content string `json:"content"`
		CursorY int    `json:"cursor_y"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("failed to parse initial capture: %w", err)
	}

	// Print initial content
	content := response.Content
	if cfg.NoColor {
		content = StripANSI(content)
	}
	_, _ = fmt.Fprint(cfg.Writer, content)

	lastCursorY := response.CursorY

	// Poll loop
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	retryCount := 0

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// Poll for new content from cursor
			data, err := client.ReadSessionFromCursor(cfg.ContainerID, cfg.Session, lastCursorY)
			if err != nil {
				// Check error type
				errMsg := err.Error()

				// Session not found (404)
				if strings.Contains(errMsg, "status 404") {
					_, _ = fmt.Fprintln(cfg.ErrWriter, "Session ended.")
					return nil
				}

				// Container not running (400)
				if strings.Contains(errMsg, "status 400") {
					_, _ = fmt.Fprintln(cfg.ErrWriter, "Container stopped.")
					return nil
				}

				// Connection failure - retry once
				if strings.Contains(errMsg, "failed to connect") {
					retryCount++
					if retryCount > 1 {
						return err
					}
					// Retry on next tick
					continue
				}

				return err
			}

			retryCount = 0 // Reset retry count on success

			if err := json.Unmarshal(data, &response); err != nil {
				return fmt.Errorf("failed to parse poll response: %w", err)
			}

			// Check for cursor reset
			if response.CursorY < lastCursorY {
				// Pane was reset (e.g., clear command). Do a full capture.
				data, err := client.ReadSession(cfg.ContainerID, cfg.Session, 0)
				if err != nil {
					errMsg := err.Error()
					if strings.Contains(errMsg, "status 404") {
						_, _ = fmt.Fprintln(cfg.ErrWriter, "Session ended.")
						return nil
					}
					if strings.Contains(errMsg, "status 400") {
						_, _ = fmt.Fprintln(cfg.ErrWriter, "Container stopped.")
						return nil
					}
					return err
				}

				if err := json.Unmarshal(data, &response); err != nil {
					return fmt.Errorf("failed to parse full capture after reset: %w", err)
				}

				content := response.Content
				if cfg.NoColor {
					content = StripANSI(content)
				}
				_, _ = fmt.Fprint(cfg.Writer, content)
				lastCursorY = response.CursorY
				continue
			}

			// If cursor_y > lastCursorY and content is non-empty, write it
			if response.CursorY > lastCursorY && response.Content != "" {
				content := response.Content
				if cfg.NoColor {
					content = StripANSI(content)
				}
				_, _ = fmt.Fprint(cfg.Writer, content)
				lastCursorY = response.CursorY
			}
			// If cursor_y == lastCursorY, skip (no new output)
		}
	}
}

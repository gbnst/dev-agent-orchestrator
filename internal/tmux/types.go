// pattern: Functional Core

package tmux

import (
	"fmt"
	"time"
)

// Session represents a tmux session inside a container.
type Session struct {
	Name        string
	ContainerID string
	Windows     int
	Attached    bool
	CreatedAt   time.Time
}

// AttachCommand returns the command to attach to this session.
// The user parameter specifies which user to exec as (typically "vscode").
// Includes -e flags for TERM/COLORTERM since docker exec inherits TERM=dumb by default,
// and -u flag for tmux to enable UTF-8 support.
func (s Session) AttachCommand(runtime string, user string) string {
	return fmt.Sprintf("%s exec -it -u %s -e TERM=xterm-256color -e COLORTERM=truecolor %s tmux -u attach -t %s", runtime, user, s.ContainerID, s.Name)
}

// IsActive returns true if the session has an attached client.
func (s Session) IsActive() bool {
	return s.Attached
}

// Age returns how long the session has been running.
func (s Session) Age() time.Duration {
	return time.Since(s.CreatedAt)
}

// Pane represents a tmux pane inside a session.
type Pane struct {
	Index   int
	Active  bool
	Content string
}

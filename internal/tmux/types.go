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
func (s Session) AttachCommand(runtime string) string {
	return fmt.Sprintf("%s exec -it %s tmux attach -t %s", runtime, s.ContainerID, s.Name)
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

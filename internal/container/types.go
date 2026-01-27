package container

import (
	"fmt"
	"time"
)

// ContainerState represents the current state of a container.
type ContainerState string

const (
	StateCreated ContainerState = "created"
	StateRunning ContainerState = "running"
	StateStopped ContainerState = "stopped"
)

// Session represents a tmux session inside a container.
// This is a copy of tmux.Session to avoid import cycles.
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

// Container represents a devagent-managed container.
type Container struct {
	ID          string
	Name        string
	ProjectPath string
	Template    string
	Agent       string
	State       ContainerState
	CreatedAt   time.Time
	Labels      map[string]string
	Sessions    []Session
}

// HasSessions returns true if the container has any tmux sessions.
func (c *Container) HasSessions() bool {
	return len(c.Sessions) > 0
}

// SessionCount returns the number of tmux sessions in the container.
func (c *Container) SessionCount() int {
	return len(c.Sessions)
}

// DevcontainerJSON represents the structure of a devcontainer.json file.
type DevcontainerJSON struct {
	Name              string                            `json:"name"`
	Image             string                            `json:"image,omitempty"`
	Features          map[string]map[string]interface{} `json:"features,omitempty"`
	Customizations    map[string]interface{}            `json:"customizations,omitempty"`
	PostCreateCommand string                            `json:"postCreateCommand,omitempty"`
	ContainerEnv      map[string]string                 `json:"containerEnv,omitempty"`
	RunArgs           []string                          `json:"runArgs,omitempty"`
}

// CreateOptions holds options for creating a new container.
type CreateOptions struct {
	ProjectPath string
	Template    string
	Name        string
	Agent       string
}

// Label constants for devagent metadata.
const (
	LabelManagedBy   = "devagent.managed"
	LabelProjectPath = "devagent.project_path"
	LabelTemplate    = "devagent.template"
	LabelAgent       = "devagent.agent"
)

// IsRunning returns true if the container is in a running state.
func (c *Container) IsRunning() bool {
	return c.State == StateRunning
}

// IsStopped returns true if the container is stopped or exited.
func (c *Container) IsStopped() bool {
	return c.State == StateStopped
}

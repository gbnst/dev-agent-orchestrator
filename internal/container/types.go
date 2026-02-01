// pattern: Functional Core

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
// The user parameter specifies which user to exec as (typically "vscode").
func (s Session) AttachCommand(runtime string, user string) string {
	return fmt.Sprintf("%s exec -it -u %s %s tmux attach -t %s", runtime, user, s.ContainerID, s.Name)
}

// Container represents a devagent-managed container.
type Container struct {
	ID          string
	Name        string
	ProjectPath string
	Template    string
	Agent       string
	RemoteUser  string // User for exec commands (default: vscode)
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

// BuildConfig represents the build section of a devcontainer.json.
type BuildConfig struct {
	Dockerfile string `json:"dockerfile,omitempty"`
	Context    string `json:"context,omitempty"`
}

// DevcontainerJSON represents the structure of a devcontainer.json file.
type DevcontainerJSON struct {
	Name              string                            `json:"name"`
	Image             string                            `json:"image,omitempty"`
	Build             *BuildConfig                      `json:"build,omitempty"`
	Features          map[string]map[string]interface{} `json:"features,omitempty"`
	Customizations    map[string]interface{}            `json:"customizations,omitempty"`
	PostCreateCommand string                            `json:"postCreateCommand,omitempty"`
	ContainerEnv      map[string]string                 `json:"containerEnv,omitempty"`
	RunArgs           []string                          `json:"runArgs,omitempty"`
	Mounts            []string                          `json:"mounts,omitempty"`
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
	LabelRemoteUser  = "devagent.remote_user"
)

// DefaultRemoteUser is the default user for devcontainer exec commands.
const DefaultRemoteUser = "vscode"

// IsRunning returns true if the container is in a running state.
func (c *Container) IsRunning() bool {
	return c.State == StateRunning
}

// IsStopped returns true if the container is stopped or exited.
func (c *Container) IsStopped() bool {
	return c.State == StateStopped
}

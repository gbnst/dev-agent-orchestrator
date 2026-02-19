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
// Includes -e flags for TERM/COLORTERM since docker exec inherits TERM=dumb by default,
// and -u flag for tmux to enable UTF-8 support.
func (s Session) AttachCommand(runtime string, user string) string {
	return fmt.Sprintf("%s exec -it -u %s -e TERM=xterm-256color -e COLORTERM=truecolor %s tmux -u attach -t %s", runtime, user, s.ContainerID, s.Name)
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

// Sidecar represents an auxiliary container that provides services to a devcontainer.
// ParentRef contains the compose project name, which groups the app and sidecar containers.
type Sidecar struct {
	ID        string // Container ID
	Type      string // Sidecar type (e.g., "proxy")
	ParentRef string // Compose project name linking sidecar to devcontainer
	State     ContainerState
}

// BuildConfig represents the build section of a devcontainer.json.
type BuildConfig struct {
	Dockerfile string `json:"dockerfile,omitempty"`
	Context    string `json:"context,omitempty"`
}

// DevcontainerJSON represents the structure of a devcontainer.json file.
type DevcontainerJSON struct {
	Name              string                    `json:"name"`
	Image             string                    `json:"image,omitempty"`
	Build             *BuildConfig              `json:"build,omitempty"`
	DockerComposeFile string                    `json:"dockerComposeFile,omitempty"`
	Service           string                    `json:"service,omitempty"`
	Features          map[string]map[string]any `json:"features,omitempty"`
	Customizations    map[string]any            `json:"customizations,omitempty"`
	PostCreateCommand string                    `json:"postCreateCommand,omitempty"`
	ContainerEnv      map[string]string         `json:"containerEnv,omitempty"`
	RunArgs           []string                  `json:"runArgs,omitempty"`
	Mounts            []string                  `json:"mounts,omitempty"`
	CapAdd            []string                  `json:"capAdd,omitempty"`
	SecurityOpt       []string                  `json:"securityOpt,omitempty"`
	WorkspaceFolder   string                    `json:"workspaceFolder,omitempty"`
	RemoteUser        string                    `json:"remoteUser,omitempty"`
}

// ProgressStep represents a step during container creation.
type ProgressStep struct {
	Step    string // "network", "proxy", "config", "devcontainer"
	Status  string // "started", "completed", "failed"
	Message string
}

// ProgressCallback is called during container creation to report progress.
type ProgressCallback func(ProgressStep)

// CreateOptions holds options for creating a new container.
type CreateOptions struct {
	ProjectPath string
	Template    string
	Name        string
	Agent       string
	OnProgress  ProgressCallback // Optional callback for progress updates
}

// Label constants for devagent metadata.
const (
	LabelManagedBy   = "devagent.managed"
	LabelProjectPath = "devagent.project_path"
	LabelTemplate    = "devagent.template"
	LabelAgent       = "devagent.agent"
	LabelRemoteUser  = "devagent.remote_user"
)

// Sidecar label constants
const (
	LabelSidecarType = "devagent.sidecar_type" // Type of sidecar (e.g., "proxy")
)

// Docker Compose label constants
const (
	LabelComposeProject = "com.docker.compose.project" // Set by devcontainer CLI / docker compose
)

// DefaultRemoteUser is the default user for devcontainer exec commands.
const DefaultRemoteUser = "vscode"

// HashTruncLen is the number of hex characters used for truncated SHA256 hashes.
// Used for project hashes in container names, sidecar refs, and directory paths.
const HashTruncLen = 12

// IsolationInfo holds runtime isolation details queried from the container.
// This information is retrieved from Docker/Podman inspect and associated sidecar data.
type IsolationInfo struct {
	// Security capabilities
	DroppedCaps []string // Capabilities dropped from container
	AddedCaps   []string // Capabilities added to container

	// Resource limits (human-readable format)
	MemoryLimit string // e.g., "4g", "512m", or empty if unlimited
	CPULimit    string // e.g., "2", "0.5", or empty if unlimited
	PidsLimit   int    // Process limit, 0 means unlimited

	// Network isolation
	NetworkIsolated bool     // True if container is on an isolated network
	NetworkName     string   // Name of the isolated network (if any)
	ContainerIP     string   // Container's IP on the isolated network
	Gateway         string   // Network gateway address
	ProxyAddress    string   // Proxy address from http_proxy env var
	ProxySidecar    *Sidecar // Proxy sidecar (if network isolation enabled)
	AllowedDomains  []string // Domains allowed through the proxy
}

// IsRunning returns true if the container is in a running state.
func (c *Container) IsRunning() bool {
	return c.State == StateRunning
}

// IsStopped returns true if the container is stopped or exited.
func (c *Container) IsStopped() bool {
	return c.State == StateStopped
}

// MountInfo represents a bind mount or volume mount on a container.
type MountInfo struct {
	Type        string `json:"type"`        // "bind" or "volume"
	Source      string `json:"source"`      // Host path or volume name
	Destination string `json:"destination"` // Container path
	ReadOnly    bool   `json:"read_only"`
}

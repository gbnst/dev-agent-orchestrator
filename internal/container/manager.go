// pattern: Imperative Shell

package container

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"devagent/internal/config"
	"devagent/internal/logging"
	"devagent/internal/tmux"
)

// RuntimeInterface abstracts container runtime operations for testing.
type RuntimeInterface interface {
	ListContainers(ctx context.Context) ([]Container, error)
	Exec(ctx context.Context, id string, cmd []string) (string, error)
	ExecAs(ctx context.Context, id string, user string, cmd []string) (string, error)
	InspectContainer(ctx context.Context, id string) (ContainerState, error)
	GetIsolationInfo(ctx context.Context, id string) (*IsolationInfo, error)

	// Compose lifecycle operations
	ComposeUp(ctx context.Context, projectDir string, projectName string, env map[string]string) error
	ComposeStart(ctx context.Context, projectDir string, projectName string) error
	ComposeStop(ctx context.Context, projectDir string, projectName string) error
	ComposeDown(ctx context.Context, projectDir string, projectName string) error
}

// Manager orchestrates container lifecycle operations.
type Manager struct {
	mu               sync.RWMutex // protects containers and sidecars maps
	cfg              *config.Config
	runtime          RuntimeInterface
	runtimeName      string            // "docker" or "podman" - used for attach commands
	runtimePath      string            // full path to binary - bypasses shell aliases
	composeGenerator *ComposeGenerator // for compose-based orchestration
	tmuxClient       *tmux.Client
	containers       map[string]*Container
	sidecars         map[string]*Sidecar // Maps sidecar container ID to Sidecar
	logger           *logging.ScopedLogger
	logManager       logging.LoggerProvider        // for per-container loggers
	proxyLogCancels  map[string]context.CancelFunc // proxyLogPath -> cancel func
	onChange         func()                        // called after state changes (e.g. to notify SSE clients)
}

// SetOnChange registers a callback invoked after container/session state changes.
// Must be set before any concurrent access to Manager (e.g. before goroutines call Refresh).
func (m *Manager) SetOnChange(fn func()) {
	m.onChange = fn
}

// notifyChange calls the onChange callback if set.
func (m *Manager) notifyChange() {
	if m.onChange != nil {
		m.onChange()
	}
}

// ManagerOptions holds all configuration options for creating a Manager.
type ManagerOptions struct {
	Config      *config.Config
	Templates   []config.Template
	Runtime     RuntimeInterface
	ComposeGen  *ComposeGenerator
	LogManager  logging.LoggerProvider
	RuntimeName string // "docker" or "podman" - used for attach commands
	RuntimePath string // full path to binary - bypasses shell aliases
}

// nopLoggerProvider is a no-op LoggerProvider that returns NopLogger for all scopes.
type nopLoggerProvider struct{}

func (n *nopLoggerProvider) For(scope string) *logging.ScopedLogger {
	return logging.NopLogger()
}

// NewManager creates a Manager with the provided options.
// Required fields: Runtime or Config (if Runtime is nil, it is auto-created from Config).
// Other fields are optional and created with sensible defaults if not provided.
func NewManager(opts ManagerOptions) *Manager {
	// Default runtime name/path from config if available
	if opts.RuntimeName == "" && opts.Config != nil {
		opts.RuntimeName = opts.Config.DetectedRuntime()
	}
	if opts.RuntimePath == "" && opts.Config != nil {
		opts.RuntimePath = opts.Config.DetectedRuntimePath()
	}

	// Auto-create runtime from config if not provided
	if opts.Runtime == nil && opts.Config != nil {
		opts.Runtime = NewRuntime(opts.RuntimeName)
	}

	// Default logger to NopLogger
	var logManager logging.LoggerProvider = opts.LogManager
	if logManager == nil {
		logManager = &nopLoggerProvider{}
	}
	logger := logManager.For("container")

	// Log initialization (skip for nop logger)
	if _, isNop := logManager.(*nopLoggerProvider); !isNop {
		logger.Debug("container manager initialized")
	}

	// Create ComposeGenerator if config and templates are provided but generator isn't
	if opts.ComposeGen == nil && opts.Config != nil && opts.Templates != nil {
		opts.ComposeGen = NewComposeGenerator(opts.Config, opts.Templates, logManager.For("compose"))
	}

	m := &Manager{
		cfg:              opts.Config,
		runtime:          opts.Runtime,
		runtimeName:      opts.RuntimeName,
		runtimePath:      opts.RuntimePath,
		composeGenerator: opts.ComposeGen,
		containers:       make(map[string]*Container),
		sidecars:         make(map[string]*Sidecar),
		logger:           logger,
		logManager:       logManager,
		proxyLogCancels:  make(map[string]context.CancelFunc),
	}

	// Create tmux.Client with executor that wraps runtime.ExecAs with user lookup
	m.tmuxClient = tmux.NewClient(func(ctx context.Context, containerID string, cmd []string) (string, error) {
		user := m.getContainerUser(containerID)
		return m.runtime.ExecAs(ctx, containerID, user, cmd)
	})

	return m
}

// Refresh updates the container list from the runtime.
func (m *Manager) Refresh(ctx context.Context) error {
	m.logger.Debug("refreshing container list")

	containers, err := m.runtime.ListContainers(ctx)
	if err != nil {
		m.logger.Error("failed to list containers", "error", err)
		return err
	}

	m.mu.Lock()

	// Rebuild containers map (exclude sidecars)
	m.containers = make(map[string]*Container)
	for i := range containers {
		c := containers[i]
		// Skip sidecars - they're tracked separately
		if _, isSidecar := c.Labels[LabelSidecarType]; !isSidecar {
			m.containers[c.ID] = &c
		}
	}

	// Rebuild sidecars map
	m.refreshSidecars(containers)

	m.logger.Debug("container list refreshed", "count", len(m.containers), "sidecars", len(m.sidecars))

	// Start proxy log readers for containers that don't have one yet
	m.startMissingProxyLogReaders()

	m.mu.Unlock()
	m.notifyChange()
	return nil
}

// List returns all known containers sorted by name for stable display order.
func (m *Manager) List() []*Container {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Container, 0, len(m.containers))
	for _, c := range m.containers {
		result = append(result, c)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// containerLogger returns a logger scoped to a specific container.
// Falls back to base "container" scope if name is empty or logManager is nil.
func (m *Manager) containerLogger(name string) *logging.ScopedLogger {
	if name == "" || m.logManager == nil {
		return m.logger
	}
	return m.logManager.For("container." + name)
}

// reportProgress logs a progress message and notifies the OnProgress callback if set.
func (m *Manager) reportProgress(logger *logging.ScopedLogger, callback ProgressCallback, step, status, msg string) {
	logger.Info(msg, "step", step, "status", status)
	if callback != nil {
		callback(ProgressStep{Step: step, Status: status, Message: msg})
	}
}

// refreshSidecars populates the sidecars map from container labels.
// Called during Refresh() to discover existing sidecars.
// Sidecars are identified by having a LabelSidecarType label and are grouped
// by their compose project name (com.docker.compose.project label).
func (m *Manager) refreshSidecars(allContainers []Container) {
	// Clear existing sidecar map
	m.sidecars = make(map[string]*Sidecar)

	for _, c := range allContainers {
		sidecarType := c.Labels[LabelSidecarType]
		if sidecarType == "" {
			continue
		}

		// Use compose project name as the parent reference for grouping
		composeProject := c.Labels[LabelComposeProject]

		sidecar := &Sidecar{
			ID:        c.ID,
			Name:      c.Name,
			Type:      sidecarType,
			ParentRef: composeProject, // Compose project name for grouping
			State:     c.State,
		}
		m.sidecars[c.ID] = sidecar
	}
}

// GetSidecarsForProject returns all sidecars associated with a project path.
// Finds the compose project name for the project by checking app containers,
// then returns sidecars with the same compose project name.
func (m *Manager) GetSidecarsForProject(projectPath string) []*Sidecar {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Find the compose project name by looking at the app container for this project
	var composeProject string
	for _, c := range m.containers {
		if c.ProjectPath == projectPath {
			composeProject = c.Labels[LabelComposeProject]
			break
		}
	}

	if composeProject == "" {
		return nil
	}

	var result []*Sidecar
	for _, s := range m.sidecars {
		if s.ParentRef == composeProject {
			result = append(result, s)
		}
	}
	return result
}

// GetContainerIsolationInfo returns isolation details for a container.
// Combines data from Docker inspect, sidecar lookup, and proxy configuration.
func (m *Manager) GetContainerIsolationInfo(ctx context.Context, c *Container) (*IsolationInfo, error) {
	if c == nil {
		return nil, fmt.Errorf("container is nil")
	}

	// Get runtime isolation info (caps, resources, network)
	info, err := m.runtime.GetIsolationInfo(ctx, c.ID)
	if err != nil {
		return nil, err
	}

	// Look up proxy sidecar
	sidecars := m.GetSidecarsForProject(c.ProjectPath)
	for _, s := range sidecars {
		if s.Type == "proxy" {
			info.ProxySidecar = s
			break
		}
	}

	// Read allowlist from filter script if network is isolated
	if info.NetworkIsolated {
		allowlist, err := ReadAllowlistFromFilterScript(c.ProjectPath)
		if err == nil && allowlist != nil {
			info.AllowedDomains = allowlist
		}
	}

	return info, nil
}

// RuntimeName returns the container runtime name ("docker" or "podman").
func (m *Manager) RuntimeName() string {
	if m.runtimeName == "" {
		return "docker" // fallback
	}
	return m.runtimeName
}

// RuntimePath returns the full path to the container runtime binary.
// This bypasses shell aliases when the user copies and pastes commands.
func (m *Manager) RuntimePath() string {
	if m.runtimePath == "" {
		return m.RuntimeName() // fallback to name
	}
	return m.runtimePath
}

// Get returns a container by ID.
func (m *Manager) Get(id string) (*Container, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, ok := m.containers[id]
	return c, ok
}

// GetByNameOrID looks up a container by ID first, then falls back to matching
// by Docker container name. Returns (nil, false) if no match is found.
// pattern: Functional Core
func (m *Manager) GetByNameOrID(ref string) (*Container, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Fast path: exact ID match
	if c, ok := m.containers[ref]; ok {
		return c, true
	}

	// Slow path: match by container name
	for _, c := range m.containers {
		if c.Name == ref {
			return c, true
		}
	}

	return nil, false
}

// GetByComposeProject returns the container with the given compose project name, or nil.
func (m *Manager) GetByComposeProject(composeName string) *Container {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.containers {
		if c.ComposeProject == composeName {
			return c
		}
	}
	return nil
}

// CreateWithCompose creates a new devcontainer using docker-compose orchestration.
func (m *Manager) CreateWithCompose(ctx context.Context, opts CreateOptions) (*Container, error) {
	// Ensure ProjectPath is absolute (relative paths break Docker Compose volume mounts —
	// Compose interprets "foo:/path" as named volume "foo" instead of bind mount "./foo")
	if opts.ProjectPath != "" {
		absPath, err := filepath.Abs(opts.ProjectPath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve project path: %w", err)
		}
		opts.ProjectPath = absPath
	}

	// Create scoped logger for this operation.
	logger := m.containerLogger(opts.Name)

	reportProgress := func(step, status, msg string) {
		m.reportProgress(logger, opts.OnProgress, step, status, msg)
	}

	reportProgress("compose", "started", "Generating compose configuration")

	// Ensure proxy cert directory exists
	certDir, err := GetProxyCertDir(opts.ProjectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy cert directory: %w", err)
	}
	logger.Debug("proxy cert directory ready", "path", certDir)

	// Generate compose files
	composeOpts := ComposeOptions{
		ProjectPath: opts.ProjectPath,
		Template:    opts.Template,
		Name:        opts.Name,
	}

	composeResult, err := m.composeGenerator.Generate(composeOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate compose config: %w", err)
	}

	reportProgress("compose", "completed", "Compose configuration generated")

	// Only write template files if the project doesn't already have a compose file.
	// Projects with their own .devcontainer/ setup should not be overwritten.
	composeFilePath := filepath.Join(opts.ProjectPath, ".devcontainer", "docker-compose.yml")
	if _, err := os.Stat(composeFilePath); os.IsNotExist(err) {
		reportProgress("files", "started", "Writing configuration files")

		if err := m.composeGenerator.WriteToProject(opts.ProjectPath, opts.Template, composeResult.TemplateData); err != nil {
			return nil, fmt.Errorf("failed to write template files: %w", err)
		}

		reportProgress("files", "completed", "Configuration files written")
	}
	if _, err := os.Stat(composeFilePath); err != nil {
		// Format error message to include filename for clarity
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no docker-compose.yml found at %s", composeFilePath)
		}
		return nil, fmt.Errorf("compose file not accessible at %s: %w", composeFilePath, err)
	}

	// Discover port env vars from the rendered compose file
	portVars, err := ParsePortEnvVars(composeFilePath)
	if err != nil {
		// Non-fatal: if compose file doesn't have env var ports, proceed with empty map
		m.logger.Warn("failed to parse port env vars", "error", err)
		portVars = make(map[string]string)
	}

	// Allocate free ports for discovered env vars
	allocatedPorts, err := AllocateFreePorts(portVars)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate ports: %w", err)
	}

	// Determine compose project name: use opts.Name if provided (e.g. worktree-specific name),
	// otherwise derive from the project directory base name
	composeName := opts.Name
	if composeName == "" {
		composeName = SanitizeComposeName(filepath.Base(opts.ProjectPath))
	}

	reportProgress("container", "started", "Starting devcontainer")

	// Start devcontainer using direct compose up
	if err := m.runtime.ComposeUp(ctx, opts.ProjectPath, composeName, allocatedPorts); err != nil {
		reportProgress("container", "failed", fmt.Sprintf("Failed to start: %v", err))
		return nil, fmt.Errorf("compose up failed: %w", err)
	}

	logger.Info("devcontainer started via compose", "projectName", composeName)
	reportProgress("container", "completed", "Devcontainer started successfully")

	// Refresh container list
	if err := m.Refresh(ctx); err != nil {
		logger.Warn("failed to refresh container list", "error", err)
	}

	// Find the created container by project path
	m.mu.RLock()
	var container *Container
	for _, c := range m.containers {
		if c.ProjectPath == opts.ProjectPath {
			container = c
			break
		}
	}
	m.mu.RUnlock()

	if container == nil {
		return nil, fmt.Errorf("container created but not found in refresh")
	}

	// Set ComposeProject and Ports on the found container
	container.ComposeProject = composeName
	container.Ports = allocatedPorts

	return container, nil
}

// composeProjectName returns the compose project name for a container.
// Reads from Docker's com.docker.compose.project label (set by devcontainer CLI).
// Falls back to the container name if label is missing (shouldn't happen for compose containers).
func composeProjectName(c *Container) string {
	if name := c.Labels[LabelComposeProject]; name != "" {
		return name
	}
	return c.Name
}

// startMissingProxyLogReaders starts proxy log readers for containers that don't have one.
// Must be called with m.mu held.
func (m *Manager) startMissingProxyLogReaders() {
	if m.logManager == nil {
		return
	}

	sink, ok := m.logManager.(interface{ GetChannelSink() *logging.ChannelSink })
	if !ok {
		return
	}

	for _, c := range m.containers {
		// Skip containers without project path
		if c.ProjectPath == "" {
			continue
		}

		// Key by log file path to prevent duplicate readers when multiple
		// containers share the same project path (e.g., exited + running)
		proxyLogPath := filepath.Join(c.ProjectPath, ".devcontainer", "containers", "proxy", "opt", "devagent-proxy", "logs", "requests.jsonl")
		if _, hasReader := m.proxyLogCancels[proxyLogPath]; hasReader {
			continue
		}

		// Start proxy log reader
		reader, err := logging.NewProxyLogReader(proxyLogPath, c.Name, sink.GetChannelSink())
		if err != nil {
			m.logger.Debug("failed to create proxy log reader", "container", c.Name, "error", err)
			continue
		}

		ctx, cancel := context.WithCancel(context.Background())
		m.proxyLogCancels[proxyLogPath] = cancel

		go func(r *logging.ProxyLogReader, containerName string) {
			if err := r.Start(ctx); err != nil && err != context.Canceled {
				m.logger.Debug("proxy log reader stopped", "container", containerName, "error", err)
			}
		}(reader, c.Name)

		m.logger.Info("started proxy log reader", "container", c.Name, "path", proxyLogPath)
	}
}

// StartWithCompose starts a compose-based devcontainer using docker-compose start.
// This is for containers created with CreateWithCompose().
func (m *Manager) StartWithCompose(ctx context.Context, containerID string) error {
	m.mu.Lock()
	c, ok := m.containers[containerID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("container not found: %s", containerID)
	}

	if c.ProjectPath == "" {
		m.mu.Unlock()
		return fmt.Errorf("container has no project path: %s", containerID)
	}
	m.mu.Unlock()

	logger := m.containerLogger(c.Name)
	logger.Info("starting compose container")

	projectName := composeProjectName(c)

	if err := m.runtime.ComposeStart(ctx, c.ProjectPath, projectName); err != nil {
		logger.Error("failed to start compose container", "error", err)
		return fmt.Errorf("failed to start compose: %w", err)
	}

	m.mu.Lock()
	c.State = StateRunning
	m.mu.Unlock()

	logger.Info("compose container started")
	m.notifyChange()
	return nil
}

// StopWithCompose stops a compose-based devcontainer using docker-compose stop.
func (m *Manager) StopWithCompose(ctx context.Context, containerID string) error {
	m.mu.Lock()
	c, ok := m.containers[containerID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("container not found: %s", containerID)
	}

	if c.ProjectPath == "" {
		m.mu.Unlock()
		return fmt.Errorf("container has no project path: %s", containerID)
	}
	m.mu.Unlock()

	logger := m.containerLogger(c.Name)
	logger.Info("stopping compose container")

	projectName := composeProjectName(c)

	if err := m.runtime.ComposeStop(ctx, c.ProjectPath, projectName); err != nil {
		logger.Error("failed to stop compose container", "error", err)
		return fmt.Errorf("failed to stop compose: %w", err)
	}

	m.mu.Lock()
	c.State = StateStopped
	// Stop proxy log reader — container is no longer running
	proxyLogPath := filepath.Join(c.ProjectPath, ".devcontainer", "containers", "proxy", "opt", "devagent-proxy", "logs", "requests.jsonl")
	if cancel, ok := m.proxyLogCancels[proxyLogPath]; ok {
		cancel()
		delete(m.proxyLogCancels, proxyLogPath)
	}
	m.mu.Unlock()

	logger.Info("compose container stopped")
	m.notifyChange()
	return nil
}

// DestroyWithCompose destroys a compose-based devcontainer using docker-compose down.
// This removes both app and proxy containers, networks, and volumes.
func (m *Manager) DestroyWithCompose(ctx context.Context, containerID string) error {
	m.mu.Lock()
	c, ok := m.containers[containerID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("container not found: %s", containerID)
	}

	if c.ProjectPath == "" {
		m.mu.Unlock()
		return fmt.Errorf("container has no project path: %s", containerID)
	}

	// Stop proxy log reader if running (mutex already held)
	proxyLogPath := filepath.Join(c.ProjectPath, ".devcontainer", "containers", "proxy", "opt", "devagent-proxy", "logs", "requests.jsonl")
	if cancel, ok := m.proxyLogCancels[proxyLogPath]; ok {
		cancel()
		delete(m.proxyLogCancels, proxyLogPath)
	}
	m.mu.Unlock()

	logger := m.containerLogger(c.Name)
	logger.Info("destroying compose container")

	projectName := composeProjectName(c)

	// docker-compose down removes containers and networks
	if err := m.runtime.ComposeDown(ctx, c.ProjectPath, projectName); err != nil {
		logger.Error("failed to destroy compose container", "error", err)
		return fmt.Errorf("failed to destroy compose: %w", err)
	}

	// Clean up proxy config directories (same as legacy destroy)
	if err := CleanupProxyConfigs(c.ProjectPath); err != nil {
		logger.Warn("failed to cleanup proxy configs", "error", err)
		// Continue - this is non-fatal
	}

	// Remove from containers map
	m.mu.Lock()
	delete(m.containers, containerID)
	m.mu.Unlock()

	logger.Info("compose container destroyed")
	m.notifyChange()
	return nil
}

// getContainerUser returns the remote user for a container, defaulting to DefaultRemoteUser.
func (m *Manager) getContainerUser(containerID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if c, ok := m.containers[containerID]; ok && c.RemoteUser != "" {
		return c.RemoteUser
	}
	return DefaultRemoteUser
}

// getContainerName returns the name of a container by ID, or empty string if not found.
func (m *Manager) getContainerName(containerID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if c, ok := m.containers[containerID]; ok {
		return c.Name
	}
	return ""
}

// CreateSession creates a tmux session inside a container.
func (m *Manager) CreateSession(ctx context.Context, containerID, sessionName string) error {
	containerName := m.getContainerName(containerID)
	scopedLogger := m.containerLogger(containerName).With("containerID", containerID, "session", sessionName)
	scopedLogger.Info("creating tmux session")

	// Delegate to tmux.Client
	if err := m.tmuxClient.CreateSession(ctx, containerID, sessionName); err != nil {
		scopedLogger.Error("failed to create session", "error", err)
		return err
	}

	scopedLogger.Info("session created")
	m.notifyChange()
	return nil
}

// KillSession destroys a tmux session inside a container.
func (m *Manager) KillSession(ctx context.Context, containerID, sessionName string) error {
	containerName := m.getContainerName(containerID)
	scopedLogger := m.containerLogger(containerName).With("containerID", containerID, "session", sessionName)
	scopedLogger.Info("killing tmux session")

	// Delegate to tmux.Client
	if err := m.tmuxClient.KillSession(ctx, containerID, sessionName); err != nil {
		scopedLogger.Error("failed to kill session", "error", err)
		return err
	}

	scopedLogger.Info("session killed")
	m.notifyChange()
	return nil
}

// ListSessions lists tmux sessions inside a container.
func (m *Manager) ListSessions(ctx context.Context, containerID string) ([]tmux.Session, error) {
	containerName := m.getContainerName(containerID)
	scopedLogger := m.containerLogger(containerName).With("containerID", containerID)
	scopedLogger.Debug("listing tmux sessions")

	// Delegate to tmux.Client
	sessions, err := m.tmuxClient.ListSessions(ctx, containerID)
	if err != nil {
		scopedLogger.Error("failed to list sessions", "error", err)
		return nil, err
	}

	scopedLogger.Debug("sessions listed", "count", len(sessions))
	return sessions, nil
}

// CaptureSession captures pane content from a tmux session in a container.
func (m *Manager) CaptureSession(ctx context.Context, containerID, sessionName string, opts tmux.CaptureOpts) (string, error) {
	containerName := m.getContainerName(containerID)
	scopedLogger := m.containerLogger(containerName).With("containerID", containerID, "session", sessionName)
	scopedLogger.Debug("capturing tmux session")

	content, err := m.tmuxClient.CapturePane(ctx, containerID, sessionName, opts)
	if err != nil {
		scopedLogger.Error("failed to capture session", "error", err)
		return "", err
	}

	return content, nil
}

// CaptureSessionLines captures the last N lines from a tmux session's scrollback history.
func (m *Manager) CaptureSessionLines(ctx context.Context, containerID, sessionName string, lines int) (string, error) {
	containerName := m.getContainerName(containerID)
	scopedLogger := m.containerLogger(containerName).With("containerID", containerID, "session", sessionName)
	scopedLogger.Debug("capturing tmux session lines", "lines", lines)

	content, err := m.tmuxClient.CaptureLines(ctx, containerID, sessionName, lines)
	if err != nil {
		scopedLogger.Error("failed to capture session lines", "error", err)
		return "", err
	}

	return content, nil
}

// CursorPosition returns the cursor row position for a tmux session.
func (m *Manager) CursorPosition(ctx context.Context, containerID, sessionName string) (int, error) {
	containerName := m.getContainerName(containerID)
	scopedLogger := m.containerLogger(containerName).With("containerID", containerID, "session", sessionName)
	scopedLogger.Debug("getting cursor position")

	pos, err := m.tmuxClient.CursorPosition(ctx, containerID, sessionName)
	if err != nil {
		scopedLogger.Error("failed to get cursor position", "error", err)
		return 0, err
	}

	return pos, nil
}

// SendToSession sends keystrokes to a tmux session in a container.
func (m *Manager) SendToSession(ctx context.Context, containerID, sessionName, text string) error {
	containerName := m.getContainerName(containerID)
	scopedLogger := m.containerLogger(containerName).With("containerID", containerID, "session", sessionName)
	scopedLogger.Info("sending keys to tmux session")

	if err := m.tmuxClient.SendKeys(ctx, containerID, sessionName, text); err != nil {
		scopedLogger.Error("failed to send keys", "error", err)
		return err
	}

	return nil
}

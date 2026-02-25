// pattern: Imperative Shell

package container

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
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
	ComposeUp(ctx context.Context, projectDir string, projectName string) error
	ComposeStart(ctx context.Context, projectDir string, projectName string) error
	ComposeStop(ctx context.Context, projectDir string, projectName string) error
	ComposeDown(ctx context.Context, projectDir string, projectName string) error
}

// Manager orchestrates container lifecycle operations.
type Manager struct {
	mu               sync.RWMutex // protects containers and sidecars maps
	cfg              *config.Config
	runtime          RuntimeInterface
	runtimeName      string // "docker" or "podman" - used for attach commands
	runtimePath      string // full path to binary - bypasses shell aliases
	generator        *DevcontainerGenerator
	composeGenerator *ComposeGenerator // for compose-based orchestration
	devCLI           *DevcontainerCLI
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
	Generator   *DevcontainerGenerator
	ComposeGen  *ComposeGenerator
	DevCLI      *DevcontainerCLI
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

	// Create generators if config and templates are provided but generators aren't
	if opts.Generator == nil && opts.Config != nil && opts.Templates != nil {
		opts.Generator = NewDevcontainerGenerator(opts.Config, opts.Templates)
	}
	if opts.ComposeGen == nil && opts.Config != nil && opts.Templates != nil {
		opts.ComposeGen = NewComposeGenerator(opts.Config, opts.Templates, logManager.For("compose"))
	}

	// Create DevcontainerCLI if config is provided but CLI isn't
	if opts.DevCLI == nil && opts.Config != nil {
		if opts.Config.Runtime != "" {
			opts.DevCLI = NewDevcontainerCLIWithRuntime(opts.Config.Runtime)
		} else {
			opts.DevCLI = NewDevcontainerCLI()
		}
	}

	m := &Manager{
		cfg:              opts.Config,
		runtime:          opts.Runtime,
		runtimeName:      opts.RuntimeName,
		runtimePath:      opts.RuntimePath,
		generator:        opts.Generator,
		composeGenerator: opts.ComposeGen,
		devCLI:           opts.DevCLI,
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
	reportProgress("devcontainer", "started", "Generating devcontainer configuration")

	// Generate devcontainer.json
	devcontainerResult, err := m.generator.Generate(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate devcontainer config: %w", err)
	}

	reportProgress("devcontainer", "completed", "Devcontainer configuration generated")
	reportProgress("files", "started", "Writing configuration files")

	// Write all files to project
	if err := m.generator.WriteAll(opts.ProjectPath, devcontainerResult, composeResult); err != nil {
		return nil, fmt.Errorf("failed to write configuration files: %w", err)
	}

	reportProgress("files", "completed", "Configuration files written")
	reportProgress("container", "started", "Starting devcontainer")

	// Start devcontainer (devcontainer CLI handles compose orchestration)
	containerID, err := m.devCLI.Up(ctx, opts.ProjectPath)
	if err != nil && containerID == "" {
		reportProgress("container", "failed", fmt.Sprintf("Failed to start: %v", err))
		return nil, fmt.Errorf("failed to start devcontainer: %w", err)
	}
	if err != nil {
		// Container was created but postCreateCommand or similar failed.
		// Log the error but continue — the container is usable.
		logger.Warn("devcontainer up completed with error (postCreateCommand may have failed)", "error", err)
		reportProgress("container", "completed", "Devcontainer started (with post-create warnings)")
	}

	displayID := containerID
	if len(displayID) > 12 {
		displayID = displayID[:12] // Display truncation for container IDs (not project hash)
	}
	logger.Info("devcontainer started", "containerID", displayID)
	reportProgress("container", "completed", "Devcontainer started successfully")

	// Refresh container list
	if err := m.Refresh(ctx); err != nil {
		logger.Warn("failed to refresh container list", "error", err)
	}

	// Find the created container
	// Container ID from devcontainer up may be truncated, so use prefix match
	m.mu.RLock()
	var container *Container
	for _, c := range m.containers {
		if strings.HasPrefix(c.ID, containerID) || strings.HasPrefix(containerID, c.ID) {
			container = c
			break
		}
	}

	// If not found by ID, try matching by project path
	if container == nil {
		for _, c := range m.containers {
			if c.ProjectPath == opts.ProjectPath {
				container = c
				break
			}
		}
	}
	m.mu.RUnlock()

	if container == nil {
		return nil, fmt.Errorf("container created but not found in refresh: %s", containerID)
	}

	return container, nil
}

// StartWorktreeContainer starts a devcontainer for an already-configured worktree directory.
// The worktree's .devcontainer/ is expected to be fully configured (compose YAML, Dockerfile, etc.).
// If devcontainer up fails but the container was created (e.g. postCreateCommand error),
// the container ID is returned along with the error.
func (m *Manager) StartWorktreeContainer(ctx context.Context, wtPath string) (string, error) {
	containerID, err := m.devCLI.Up(ctx, wtPath)
	if err != nil && containerID == "" {
		return "", fmt.Errorf("failed to start worktree container: %w", err)
	}
	if err := m.Refresh(ctx); err != nil {
		m.logger.Warn("failed to refresh after worktree container start", "error", err)
	}
	return containerID, nil
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

// pattern: Imperative Shell

package container

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"devagent/internal/config"
	"devagent/internal/logging"
)

// RuntimeInterface abstracts container runtime operations for testing.
type RuntimeInterface interface {
	ListContainers(ctx context.Context) ([]Container, error)
	StartContainer(ctx context.Context, id string) error
	StopContainer(ctx context.Context, id string) error
	RemoveContainer(ctx context.Context, id string) error
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
	cfg              *config.Config
	runtime          RuntimeInterface
	runtimeName      string // "docker" or "podman" - used for attach commands
	runtimePath      string // full path to binary - bypasses shell aliases
	generator        *DevcontainerGenerator
	composeGenerator *ComposeGenerator // for compose-based orchestration
	devCLI           *DevcontainerCLI
	containers       map[string]*Container
	sidecars         map[string]*Sidecar // Maps sidecar container ID to Sidecar
	logger           *logging.ScopedLogger
	logManager       interface{ For(string) *logging.ScopedLogger } // for per-container loggers
}

// NewManager creates a new Manager with the given config and templates.
func NewManager(cfg *config.Config, templates []config.Template) *Manager {
	runtimeName := cfg.DetectedRuntime()
	runtimePath := cfg.DetectedRuntimePath()
	runtime := NewRuntime(runtimeName)
	generator := NewDevcontainerGenerator(cfg, templates)
	composeGenerator := NewComposeGenerator(templates)

	// Use explicit runtime for devcontainer CLI if configured
	var devCLI *DevcontainerCLI
	if cfg.Runtime != "" {
		devCLI = NewDevcontainerCLIWithRuntime(cfg.Runtime)
	} else {
		devCLI = NewDevcontainerCLI()
	}

	return &Manager{
		cfg:              cfg,
		runtime:          runtime,
		runtimeName:      runtimeName,
		runtimePath:      runtimePath,
		generator:        generator,
		composeGenerator: composeGenerator,
		devCLI:           devCLI,
		containers:       make(map[string]*Container),
		sidecars:         make(map[string]*Sidecar),
		logger:           logging.NopLogger(),
	}
}

// NewManagerWithRuntime creates a Manager with a mock runtime for testing.
func NewManagerWithRuntime(runtime RuntimeInterface) *Manager {
	return &Manager{
		runtime:    runtime,
		containers: make(map[string]*Container),
		sidecars:   make(map[string]*Sidecar),
		logger:     logging.NopLogger(),
	}
}

// NewManagerWithDeps creates a Manager with all dependencies injected for testing.
func NewManagerWithDeps(runtime RuntimeInterface, generator *DevcontainerGenerator, devCLI *DevcontainerCLI) *Manager {
	return &Manager{
		runtime:    runtime,
		generator:  generator,
		devCLI:     devCLI,
		containers: make(map[string]*Container),
		sidecars:   make(map[string]*Sidecar),
		logger:     logging.NopLogger(),
	}
}

// NewManagerWithAllDeps creates a Manager with all dependencies including composeGenerator for testing.
// This is used by tests that need to verify compose-based operations.
func NewManagerWithAllDeps(cfg *config.Config, templates []config.Template, runtime RuntimeInterface, devCLI *DevcontainerCLI) *Manager {
	generator := NewDevcontainerGenerator(cfg, templates)
	composeGenerator := NewComposeGenerator(templates)

	return &Manager{
		cfg:              cfg,
		runtime:          runtime,
		runtimeName:      cfg.DetectedRuntime(),
		runtimePath:      cfg.DetectedRuntimePath(),
		generator:        generator,
		composeGenerator: composeGenerator,
		devCLI:           devCLI,
		containers:       make(map[string]*Container),
		sidecars:         make(map[string]*Sidecar),
		logger:           logging.NopLogger(),
	}
}

// NewManagerWithRuntimeAndLogger creates a Manager with a mock runtime and logger for testing.
// Accepts any type with a For(scope string) -> *ScopedLogger method.
func NewManagerWithRuntimeAndLogger(runtime RuntimeInterface, logManager interface{ For(string) *logging.ScopedLogger }) *Manager {
	logger := logManager.For("container")
	logger.Debug("container manager initialized")

	return &Manager{
		runtime:    runtime,
		containers: make(map[string]*Container),
		sidecars:   make(map[string]*Sidecar),
		logger:     logger,
		logManager: logManager,
	}
}

// NewManagerWithConfigAndLogger creates a Manager with config, templates, and logger.
// Used by TUI to create a fully-initialized manager with logging.
func NewManagerWithConfigAndLogger(cfg *config.Config, templates []config.Template, logManager interface{ For(string) *logging.ScopedLogger }) *Manager {
	runtimeName := cfg.DetectedRuntime()
	runtimePath := cfg.DetectedRuntimePath()
	runtime := NewRuntime(runtimeName)
	generator := NewDevcontainerGenerator(cfg, templates)
	composeGenerator := NewComposeGenerator(templates)

	// Use explicit runtime for devcontainer CLI if configured
	var devCLI *DevcontainerCLI
	if cfg.Runtime != "" {
		devCLI = NewDevcontainerCLIWithRuntime(cfg.Runtime)
	} else {
		devCLI = NewDevcontainerCLI()
	}

	logger := logManager.For("container")
	logger.Debug("container manager initialized")

	return &Manager{
		cfg:              cfg,
		runtimeName:      runtimeName,
		runtimePath:      runtimePath,
		runtime:          runtime,
		generator:        generator,
		composeGenerator: composeGenerator,
		devCLI:           devCLI,
		containers:       make(map[string]*Container),
		sidecars:         make(map[string]*Sidecar),
		logger:           logger,
		logManager:       logManager,
	}
}

// Refresh updates the container list from the runtime.
func (m *Manager) Refresh(ctx context.Context) error {
	m.logger.Debug("refreshing container list")

	containers, err := m.runtime.ListContainers(ctx)
	if err != nil {
		m.logger.Error("failed to list containers", "error", err)
		return err
	}

	// Rebuild containers map (exclude sidecars)
	m.containers = make(map[string]*Container)
	for i := range containers {
		c := containers[i]
		// Skip sidecars - they're tracked separately
		if _, isSidecar := c.Labels[LabelSidecarOf]; !isSidecar {
			m.containers[c.ID] = &c
		}
	}

	// Rebuild sidecars map
	m.refreshSidecars(ctx, containers)

	m.logger.Debug("container list refreshed", "count", len(m.containers), "sidecars", len(m.sidecars))
	return nil
}

// List returns all known containers.
func (m *Manager) List() []*Container {
	result := make([]*Container, 0, len(m.containers))
	for _, c := range m.containers {
		result = append(result, c)
	}
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

// refreshSidecars populates the sidecars map from container labels.
// Called during Refresh() to discover existing sidecars.
func (m *Manager) refreshSidecars(ctx context.Context, allContainers []Container) {
	// Clear existing sidecar map
	m.sidecars = make(map[string]*Sidecar)

	for _, c := range allContainers {
		// Check if this container is a sidecar (LabelSidecarOf contains project hash)
		if parentRef, ok := c.Labels[LabelSidecarOf]; ok {
			sidecarType := c.Labels[LabelSidecarType]

			// Determine network name from container name pattern
			// e.g., devagent-abc123-proxy -> devagent-abc123-net
			networkName := ""
			if strings.HasSuffix(c.Name, "-proxy") {
				networkName = strings.TrimSuffix(c.Name, "-proxy") + "-net"
			}

			sidecar := &Sidecar{
				ID:          c.ID,
				Type:        sidecarType,
				ParentRef:   parentRef, // Project path hash
				NetworkName: networkName,
				State:       c.State,
			}
			m.sidecars[c.ID] = sidecar
		}
	}
}

// GetSidecarsForProject returns all sidecars associated with a project path.
// Uses the project path hash (same hash used when creating sidecars).
func (m *Manager) GetSidecarsForProject(projectPath string) []*Sidecar {
	hash := sha256.Sum256([]byte(projectPath))
	hashStr := hex.EncodeToString(hash[:])[:12]

	var result []*Sidecar
	for _, s := range m.sidecars {
		if s.ParentRef == hashStr {
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
	// Use the compose container name (devagent-<hash>-app) so the TUI log filter matches.
	containerName := fmt.Sprintf("devagent-%s-app", projectHash(opts.ProjectPath))
	logger := m.containerLogger(containerName)

	reportProgress := func(step, status, msg string) {
		logger.Info(msg, "step", step, "status", status)
		if opts.OnProgress != nil {
			opts.OnProgress(ProgressStep{Step: step, Status: status, Message: msg})
		}
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
	if err != nil {
		reportProgress("container", "failed", fmt.Sprintf("Failed to start: %v", err))
		return nil, fmt.Errorf("failed to start devcontainer: %w", err)
	}

	displayID := containerID
	if len(displayID) > 12 {
		displayID = displayID[:12]
	}
	logger.Info("devcontainer started", "containerID", displayID)
	reportProgress("container", "completed", "Devcontainer started successfully")

	// Refresh container list
	if err := m.Refresh(ctx); err != nil {
		logger.Warn("failed to refresh container list", "error", err)
	}

	// Find the created container
	// Container ID from devcontainer up may be truncated, so use prefix match
	for _, c := range m.containers {
		if strings.HasPrefix(c.ID, containerID) || strings.HasPrefix(containerID, c.ID) {
			return c, nil
		}
	}

	// If not found by ID, try matching by project path
	for _, c := range m.containers {
		if c.ProjectPath == opts.ProjectPath {
			return c, nil
		}
	}

	return nil, fmt.Errorf("container created but not found in refresh: %s", containerID)
}

// Start starts a stopped container.
func (m *Manager) Start(ctx context.Context, id string) error {
	c, ok := m.containers[id]
	if !ok {
		m.logger.Error("failed to start container", "containerID", id, "error", "container not found")
		return errors.New("container not found: " + id)
	}

	scopedLogger := m.containerLogger(c.Name).With("containerID", id)
	scopedLogger.Info("starting container")

	if err := m.runtime.StartContainer(ctx, c.ID); err != nil {
		scopedLogger.Error("failed to start container", "error", err)
		return err
	}

	c.State = StateRunning
	scopedLogger.Info("container started")
	return nil
}

// Stop stops a running container.
func (m *Manager) Stop(ctx context.Context, id string) error {
	c, ok := m.containers[id]
	if !ok {
		m.logger.Error("failed to stop container", "containerID", id, "error", "container not found")
		return errors.New("container not found: " + id)
	}

	scopedLogger := m.containerLogger(c.Name).With("containerID", id)
	scopedLogger.Info("stopping container")

	if err := m.runtime.StopContainer(ctx, c.ID); err != nil {
		scopedLogger.Error("failed to stop container", "error", err)
		return err
	}

	c.State = StateStopped
	scopedLogger.Info("container stopped")
	return nil
}

// Destroy stops (if running) and removes a container.
func (m *Manager) Destroy(ctx context.Context, id string) error {
	c, ok := m.containers[id]
	if !ok {
		m.logger.Error("failed to destroy container", "containerID", id, "error", "container not found")
		return errors.New("container not found: " + id)
	}

	scopedLogger := m.containerLogger(c.Name).With("containerID", id)
	scopedLogger.Info("destroying container")

	// Stop first if running
	if c.State == StateRunning {
		scopedLogger.Debug("stopping running container before removal")
		if err := m.runtime.StopContainer(ctx, c.ID); err != nil {
			scopedLogger.Warn("failed to stop container before removal", "error", err)
		}
	}

	if err := m.runtime.RemoveContainer(ctx, c.ID); err != nil {
		scopedLogger.Error("failed to remove container", "error", err)
		return err
	}

	// Clean up proxy config directories
	if projectPath := c.ProjectPath; projectPath != "" {
		CleanupProxyConfigs(projectPath) //nolint:errcheck
	}

	delete(m.containers, id)
	scopedLogger.Info("container destroyed")
	return nil
}

// composeProjectName returns the compose project name for a container.
// Reads from Docker's com.docker.compose.project label (set by devcontainer CLI).
// Falls back to "devagent-<hash>" if label is missing (shouldn't happen for compose containers).
func composeProjectName(c *Container) string {
	if name := c.Labels[LabelComposeProject]; name != "" {
		return name
	}
	return "devagent-" + projectHash(c.ProjectPath)
}

// StartWithCompose starts a compose-based devcontainer using docker-compose start.
// This is for containers created with CreateWithCompose().
func (m *Manager) StartWithCompose(ctx context.Context, containerID string) error {
	c, ok := m.containers[containerID]
	if !ok {
		return fmt.Errorf("container not found: %s", containerID)
	}

	if c.ProjectPath == "" {
		return fmt.Errorf("container has no project path: %s", containerID)
	}

	logger := m.containerLogger(c.Name)
	logger.Info("starting compose container")

	projectName := composeProjectName(c)

	if err := m.runtime.ComposeStart(ctx, c.ProjectPath, projectName); err != nil {
		logger.Error("failed to start compose container", "error", err)
		return fmt.Errorf("failed to start compose: %w", err)
	}

	c.State = StateRunning
	logger.Info("compose container started")
	return nil
}

// StopWithCompose stops a compose-based devcontainer using docker-compose stop.
func (m *Manager) StopWithCompose(ctx context.Context, containerID string) error {
	c, ok := m.containers[containerID]
	if !ok {
		return fmt.Errorf("container not found: %s", containerID)
	}

	if c.ProjectPath == "" {
		return fmt.Errorf("container has no project path: %s", containerID)
	}

	logger := m.containerLogger(c.Name)
	logger.Info("stopping compose container")

	projectName := composeProjectName(c)

	if err := m.runtime.ComposeStop(ctx, c.ProjectPath, projectName); err != nil {
		logger.Error("failed to stop compose container", "error", err)
		return fmt.Errorf("failed to stop compose: %w", err)
	}

	c.State = StateStopped
	logger.Info("compose container stopped")
	return nil
}

// DestroyWithCompose destroys a compose-based devcontainer using docker-compose down.
// This removes both app and proxy containers, networks, and volumes.
func (m *Manager) DestroyWithCompose(ctx context.Context, containerID string) error {
	c, ok := m.containers[containerID]
	if !ok {
		return fmt.Errorf("container not found: %s", containerID)
	}

	if c.ProjectPath == "" {
		return fmt.Errorf("container has no project path: %s", containerID)
	}

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
	delete(m.containers, containerID)

	logger.Info("compose container destroyed")
	return nil
}

// IsComposeContainer checks if a container was created with compose orchestration.
// Returns true if the project has a docker-compose.yml file in .devcontainer/.
func (m *Manager) IsComposeContainer(containerID string) bool {
	c, ok := m.containers[containerID]
	if !ok || c.ProjectPath == "" {
		return false
	}

	composePath := filepath.Join(c.ProjectPath, ".devcontainer", "docker-compose.yml")
	_, err := os.Stat(composePath)
	return err == nil
}

// getContainerUser returns the remote user for a container, defaulting to DefaultRemoteUser.
func (m *Manager) getContainerUser(containerID string) string {
	if c, ok := m.containers[containerID]; ok && c.RemoteUser != "" {
		return c.RemoteUser
	}
	return DefaultRemoteUser
}

// getContainerName returns the name of a container by ID, or empty string if not found.
func (m *Manager) getContainerName(containerID string) string {
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

	user := m.getContainerUser(containerID)
	_, err := m.runtime.ExecAs(ctx, containerID, user, []string{"tmux", "-u", "new-session", "-d", "-s", sessionName})
	if err != nil {
		scopedLogger.Error("failed to create session", "error", err)
		return err
	}

	scopedLogger.Info("session created", "user", user)
	return nil
}

// KillSession destroys a tmux session inside a container.
func (m *Manager) KillSession(ctx context.Context, containerID, sessionName string) error {
	containerName := m.getContainerName(containerID)
	scopedLogger := m.containerLogger(containerName).With("containerID", containerID, "session", sessionName)
	scopedLogger.Info("killing tmux session")

	user := m.getContainerUser(containerID)
	_, err := m.runtime.ExecAs(ctx, containerID, user, []string{"tmux", "kill-session", "-t", sessionName})
	if err != nil {
		scopedLogger.Error("failed to kill session", "error", err)
		return err
	}

	scopedLogger.Info("session killed")
	return nil
}

// ListSessions lists tmux sessions inside a container.
func (m *Manager) ListSessions(ctx context.Context, containerID string) ([]Session, error) {
	containerName := m.getContainerName(containerID)
	scopedLogger := m.containerLogger(containerName).With("containerID", containerID)
	scopedLogger.Debug("listing tmux sessions")

	user := m.getContainerUser(containerID)
	output, err := m.runtime.ExecAs(ctx, containerID, user, []string{"tmux", "list-sessions"})
	if err != nil {
		// No tmux server running = no sessions
		scopedLogger.Debug("no tmux server running or no sessions", "error", err)
		return []Session{}, nil
	}

	sessions := parseTmuxSessions(containerID, output)
	scopedLogger.Debug("sessions listed", "count", len(sessions))
	return sessions, nil
}

// parseTmuxSessions parses tmux list-sessions output.
func parseTmuxSessions(containerID, output string) []Session {
	var sessions []Session
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}
		// Format: "name: N windows (created DATE) [(attached)]"
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) < 2 {
			continue
		}

		session := Session{
			Name:        parts[0],
			ContainerID: containerID,
			Attached:    strings.Contains(line, "(attached)"),
		}

		// Parse window count from "N windows" part
		// parts[1] looks like "1 windows (created Mon Jan 27 10:00:00 2025)"
		if len(parts) > 1 {
			var windows int
			// Try to parse "N windows" at the start (ignore parse errors - windows defaults to 0)
			_, _ = fmt.Sscanf(parts[1], "%d windows", &windows)
			session.Windows = windows
		}

		sessions = append(sessions, session)
	}

	return sessions
}

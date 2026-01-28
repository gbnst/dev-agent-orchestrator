// pattern: Imperative Shell

package container

import (
	"context"
	"errors"
	"fmt"
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
}

// Manager orchestrates container lifecycle operations.
type Manager struct {
	cfg         *config.Config
	runtime     RuntimeInterface
	runtimeName string // "docker" or "podman" - used for attach commands
	runtimePath string // full path to binary - bypasses shell aliases
	generator   *DevcontainerGenerator
	devCLI      *DevcontainerCLI
	containers  map[string]*Container
	logger      *logging.ScopedLogger
}

// NewManager creates a new Manager with the given config and templates.
func NewManager(cfg *config.Config, templates []config.Template) *Manager {
	runtimeName := cfg.DetectedRuntime()
	runtimePath := cfg.DetectedRuntimePath()
	runtime := NewRuntime(runtimeName)
	generator := NewDevcontainerGenerator(cfg, templates)

	// Use explicit runtime for devcontainer CLI if configured
	var devCLI *DevcontainerCLI
	if cfg.Runtime != "" {
		devCLI = NewDevcontainerCLIWithRuntime(cfg.Runtime)
	} else {
		devCLI = NewDevcontainerCLI()
	}

	return &Manager{
		cfg:         cfg,
		runtime:     runtime,
		runtimeName: runtimeName,
		runtimePath: runtimePath,
		generator:   generator,
		devCLI:      devCLI,
		containers:  make(map[string]*Container),
		logger:      logging.NopLogger(),
	}
}

// NewManagerWithRuntime creates a Manager with a mock runtime for testing.
func NewManagerWithRuntime(runtime RuntimeInterface) *Manager {
	return &Manager{
		runtime:    runtime,
		containers: make(map[string]*Container),
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
		logger:     logging.NopLogger(),
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
		logger:     logger,
	}
}

// NewManagerWithConfigAndLogger creates a Manager with config, templates, and logger.
// Used by TUI to create a fully-initialized manager with logging.
func NewManagerWithConfigAndLogger(cfg *config.Config, templates []config.Template, logManager interface{ For(string) *logging.ScopedLogger }) *Manager {
	runtimeName := cfg.DetectedRuntime()
	runtimePath := cfg.DetectedRuntimePath()
	runtime := NewRuntime(runtimeName)
	generator := NewDevcontainerGenerator(cfg, templates)

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
		cfg:         cfg,
		runtimeName: runtimeName,
		runtimePath: runtimePath,
		runtime:     runtime,
		generator:   generator,
		devCLI:      devCLI,
		containers:  make(map[string]*Container),
		logger:      logger,
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

	m.containers = make(map[string]*Container)
	for i := range containers {
		c := containers[i]
		m.containers[c.ID] = &c
	}

	m.logger.Debug("container list refreshed", "count", len(m.containers))
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

// Create generates a devcontainer.json and starts a new container.
func (m *Manager) Create(ctx context.Context, opts CreateOptions) (*Container, error) {
	scopedLogger := m.logger.With("projectPath", opts.ProjectPath, "template", opts.Template, "name", opts.Name)
	scopedLogger.Info("creating container")

	if m.generator == nil {
		scopedLogger.Error("generator not configured", "error", "generator is nil")
		return nil, errors.New("generator not configured")
	}

	// Generate devcontainer.json
	dc, err := m.generator.Generate(opts)
	if err != nil {
		scopedLogger.Error("failed to generate devcontainer config", "error", err)
		return nil, err
	}

	// Write to project
	if err := m.generator.WriteToProject(opts.ProjectPath, dc); err != nil {
		scopedLogger.Error("failed to write devcontainer.json", "error", err)
		return nil, err
	}

	scopedLogger.Debug("devcontainer.json written")

	// Start container using devcontainer CLI
	containerID, err := m.devCLI.Up(ctx, opts.ProjectPath)
	if err != nil {
		scopedLogger.Error("devcontainer up failed", "error", err)
		return nil, err
	}

	scopedLogger.Info("container created successfully", "containerID", containerID)

	// Refresh to get the new container
	if err := m.Refresh(ctx); err != nil {
		return nil, err
	}

	// Debug: log all container IDs we have
	var ids []string
	for id := range m.containers {
		ids = append(ids, id)
	}
	scopedLogger.Debug("container IDs after refresh", "lookingFor", containerID, "available", ids)

	container, ok := m.containers[containerID]
	if !ok {
		// Try to find by prefix match (devcontainer returns full ID, docker ps may return short)
		for id, c := range m.containers {
			if strings.HasPrefix(id, containerID) || strings.HasPrefix(containerID, id) {
				scopedLogger.Debug("found container by prefix match", "returnedID", containerID, "foundID", id)
				return c, nil
			}
		}
		scopedLogger.Error("container created but not found in refresh", "containerID", containerID)
		return nil, errors.New("container created but not found in refresh")
	}

	return container, nil
}

// Start starts a stopped container.
func (m *Manager) Start(ctx context.Context, id string) error {
	scopedLogger := m.logger.With("containerID", id)
	scopedLogger.Info("starting container")

	c, ok := m.containers[id]
	if !ok {
		scopedLogger.Error("failed to start container", "error", "container not found")
		return errors.New("container not found: " + id)
	}

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
	scopedLogger := m.logger.With("containerID", id)
	scopedLogger.Info("stopping container")

	c, ok := m.containers[id]
	if !ok {
		scopedLogger.Error("failed to stop container", "error", "container not found")
		return errors.New("container not found: " + id)
	}

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
	scopedLogger := m.logger.With("containerID", id)
	scopedLogger.Info("destroying container")

	c, ok := m.containers[id]
	if !ok {
		scopedLogger.Error("failed to destroy container", "error", "container not found")
		return errors.New("container not found: " + id)
	}

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

	delete(m.containers, id)
	scopedLogger.Info("container destroyed")
	return nil
}

// CreateSession creates a tmux session inside a container.
func (m *Manager) CreateSession(ctx context.Context, containerID, sessionName string) error {
	scopedLogger := m.logger.With("containerID", containerID, "session", sessionName)
	scopedLogger.Info("creating tmux session")

	_, err := m.runtime.Exec(ctx, containerID, []string{"tmux", "new-session", "-d", "-s", sessionName})
	if err != nil {
		scopedLogger.Error("failed to create session", "error", err)
		return err
	}

	scopedLogger.Info("session created")
	return nil
}

// KillSession destroys a tmux session inside a container.
func (m *Manager) KillSession(ctx context.Context, containerID, sessionName string) error {
	scopedLogger := m.logger.With("containerID", containerID, "session", sessionName)
	scopedLogger.Info("killing tmux session")

	_, err := m.runtime.Exec(ctx, containerID, []string{"tmux", "kill-session", "-t", sessionName})
	if err != nil {
		scopedLogger.Error("failed to kill session", "error", err)
		return err
	}

	scopedLogger.Info("session killed")
	return nil
}

// ListSessions lists tmux sessions inside a container.
func (m *Manager) ListSessions(ctx context.Context, containerID string) ([]Session, error) {
	scopedLogger := m.logger.With("containerID", containerID)
	scopedLogger.Debug("listing tmux sessions")

	output, err := m.runtime.Exec(ctx, containerID, []string{"tmux", "list-sessions"})
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

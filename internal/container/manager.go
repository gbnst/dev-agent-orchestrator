package container

import (
	"context"
	"errors"
	"strings"

	"devagent/internal/config"
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
	cfg        *config.Config
	runtime    RuntimeInterface
	generator  *DevcontainerGenerator
	devCLI     *DevcontainerCLI
	containers map[string]*Container
}

// NewManager creates a new Manager with the given config and templates.
func NewManager(cfg *config.Config, templates []config.Template) *Manager {
	runtime := NewRuntime(cfg.DetectedRuntime())
	generator := NewDevcontainerGenerator(cfg, templates)

	// Use explicit runtime for devcontainer CLI if configured
	var devCLI *DevcontainerCLI
	if cfg.Runtime != "" {
		devCLI = NewDevcontainerCLIWithRuntime(cfg.Runtime)
	} else {
		devCLI = NewDevcontainerCLI()
	}

	return &Manager{
		cfg:        cfg,
		runtime:    runtime,
		generator:  generator,
		devCLI:     devCLI,
		containers: make(map[string]*Container),
	}
}

// NewManagerWithRuntime creates a Manager with a mock runtime for testing.
func NewManagerWithRuntime(runtime RuntimeInterface) *Manager {
	return &Manager{
		runtime:    runtime,
		containers: make(map[string]*Container),
	}
}

// NewManagerWithDeps creates a Manager with all dependencies injected for testing.
func NewManagerWithDeps(runtime RuntimeInterface, generator *DevcontainerGenerator, devCLI *DevcontainerCLI) *Manager {
	return &Manager{
		runtime:    runtime,
		generator:  generator,
		devCLI:     devCLI,
		containers: make(map[string]*Container),
	}
}

// Refresh updates the container list from the runtime.
func (m *Manager) Refresh(ctx context.Context) error {
	containers, err := m.runtime.ListContainers(ctx)
	if err != nil {
		return err
	}

	m.containers = make(map[string]*Container)
	for i := range containers {
		c := containers[i]
		m.containers[c.ID] = &c
	}

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

// Get returns a container by ID.
func (m *Manager) Get(id string) (*Container, bool) {
	c, ok := m.containers[id]
	return c, ok
}

// Create generates a devcontainer.json and starts a new container.
func (m *Manager) Create(ctx context.Context, opts CreateOptions) (*Container, error) {
	if m.generator == nil {
		return nil, errors.New("generator not configured")
	}

	// Generate devcontainer.json
	dc, err := m.generator.Generate(opts)
	if err != nil {
		return nil, err
	}

	// Write to project
	if err := m.generator.WriteToProject(opts.ProjectPath, dc); err != nil {
		return nil, err
	}

	// Start container using devcontainer CLI
	containerID, err := m.devCLI.Up(ctx, opts.ProjectPath)
	if err != nil {
		return nil, err
	}

	// Refresh to get the new container
	if err := m.Refresh(ctx); err != nil {
		return nil, err
	}

	container, ok := m.containers[containerID]
	if !ok {
		return nil, errors.New("container created but not found in refresh")
	}

	return container, nil
}

// Start starts a stopped container.
func (m *Manager) Start(ctx context.Context, id string) error {
	c, ok := m.containers[id]
	if !ok {
		return errors.New("container not found: " + id)
	}

	if err := m.runtime.StartContainer(ctx, c.ID); err != nil {
		return err
	}

	c.State = StateRunning
	return nil
}

// Stop stops a running container.
func (m *Manager) Stop(ctx context.Context, id string) error {
	c, ok := m.containers[id]
	if !ok {
		return errors.New("container not found: " + id)
	}

	if err := m.runtime.StopContainer(ctx, c.ID); err != nil {
		return err
	}

	c.State = StateStopped
	return nil
}

// Destroy stops (if running) and removes a container.
func (m *Manager) Destroy(ctx context.Context, id string) error {
	c, ok := m.containers[id]
	if !ok {
		return errors.New("container not found: " + id)
	}

	// Stop first if running
	if c.State == StateRunning {
		if err := m.runtime.StopContainer(ctx, c.ID); err != nil {
			return err
		}
	}

	if err := m.runtime.RemoveContainer(ctx, c.ID); err != nil {
		return err
	}

	delete(m.containers, id)
	return nil
}

// CreateSession creates a tmux session inside a container.
func (m *Manager) CreateSession(ctx context.Context, containerID, sessionName string) error {
	_, err := m.runtime.Exec(ctx, containerID, []string{"tmux", "new-session", "-d", "-s", sessionName})
	return err
}

// KillSession destroys a tmux session inside a container.
func (m *Manager) KillSession(ctx context.Context, containerID, sessionName string) error {
	_, err := m.runtime.Exec(ctx, containerID, []string{"tmux", "kill-session", "-t", sessionName})
	return err
}

// ListSessions lists tmux sessions inside a container.
func (m *Manager) ListSessions(ctx context.Context, containerID string) ([]Session, error) {
	output, err := m.runtime.Exec(ctx, containerID, []string{"tmux", "list-sessions"})
	if err != nil {
		// No tmux server running = no sessions
		return []Session{}, nil
	}

	return parseTmuxSessions(containerID, output), nil
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
		sessions = append(sessions, session)
	}

	return sessions
}

// pattern: Imperative Shell

package container

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

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

	// Network operations
	CreateNetwork(ctx context.Context, name string) (string, error)
	RemoveNetwork(ctx context.Context, name string) error
	RunContainer(ctx context.Context, opts RunContainerOptions) (string, error)
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
	sidecars    map[string]*Sidecar // Maps sidecar container ID to Sidecar
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
		sidecars:    make(map[string]*Sidecar),
		logger:      logging.NopLogger(),
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
		sidecars:    make(map[string]*Sidecar),
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

// createProxySidecar creates a mitmproxy sidecar for network isolation.
// Returns the sidecar and the project hash (used as ParentRef).
func (m *Manager) createProxySidecar(ctx context.Context, projectPath string, networkConfig *config.NetworkConfig) (*Sidecar, string, error) {
	// Generate unique names based on project path hash
	hash := sha256.Sum256([]byte(projectPath))
	hashStr := hex.EncodeToString(hash[:])[:12]
	networkName := fmt.Sprintf("devagent-%s-net", hashStr)
	proxyName := fmt.Sprintf("devagent-%s-proxy", hashStr)

	m.logger.Info("creating proxy sidecar",
		"projectHash", hashStr,
		"network", networkName,
		"proxyName", proxyName,
	)

	// Create the isolated network
	_, err := m.runtime.CreateNetwork(ctx, networkName)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create network: %w", err)
	}

	// Get proxy config directory for certs and filter script
	certDir, err := GetProxyCertDir(projectPath)
	if err != nil {
		m.runtime.RemoveNetwork(ctx, networkName) //nolint:errcheck
		return nil, "", fmt.Errorf("failed to get cert directory: %w", err)
	}

	// Get filter script path
	configDir, err := GetProxyConfigDir(projectPath)
	if err != nil {
		m.runtime.RemoveNetwork(ctx, networkName) //nolint:errcheck
		return nil, "", fmt.Errorf("failed to get config directory: %w", err)
	}

	// Write the filter script
	allowlist := networkConfig.Allowlist
	if len(networkConfig.AllowlistExtend) > 0 {
		// Merge with default allowlist
		allowlist = append(config.DefaultIsolation.Network.Allowlist, networkConfig.AllowlistExtend...)
	}
	if _, err := WriteFilterScript(projectPath, allowlist); err != nil {
		m.runtime.RemoveNetwork(ctx, networkName) //nolint:errcheck
		return nil, "", fmt.Errorf("failed to write filter script: %w", err)
	}

	// Build mitmproxy command with --ignore-hosts for passthrough domains
	mitmCmd := []string{"mitmdump", "-s", "/data/filter.py"}
	if len(networkConfig.Passthrough) > 0 {
		pattern := GenerateIgnoreHostsPattern(networkConfig.Passthrough)
		mitmCmd = append(mitmCmd, "--ignore-hosts", pattern)
	}

	// Start the mitmproxy container
	sidecarID, err := m.runtime.RunContainer(ctx, RunContainerOptions{
		Image:      "mitmproxy/mitmproxy:latest",
		Name:       proxyName,
		Network:    networkName,
		Detach:     true,
		AutoRemove: false, // We manage lifecycle explicitly
		Labels: map[string]string{
			LabelManagedBy:   "true",
			LabelSidecarOf:   hashStr, // Use project hash as parent reference
			LabelSidecarType: "proxy",
		},
		Volumes: []string{
			certDir + ":/home/mitmproxy/.mitmproxy",
			configDir + ":/data:ro",
		},
		Command: mitmCmd,
	})
	if err != nil {
		m.runtime.RemoveNetwork(ctx, networkName) //nolint:errcheck
		return nil, "", fmt.Errorf("failed to start proxy container: %w", err)
	}

	// Wait for proxy to be ready (health check)
	if err := m.waitForProxyReady(ctx, sidecarID, 30*time.Second); err != nil {
		m.runtime.StopContainer(ctx, sidecarID)       //nolint:errcheck
		m.runtime.RemoveContainer(ctx, sidecarID)     //nolint:errcheck
		m.runtime.RemoveNetwork(ctx, networkName)      //nolint:errcheck
		return nil, "", fmt.Errorf("proxy failed to become ready: %w", err)
	}

	sidecar := &Sidecar{
		ID:          sidecarID,
		Type:        "proxy",
		ParentRef:   hashStr,
		NetworkName: networkName,
		State:       StateRunning,
	}

	m.sidecars[sidecarID] = sidecar
	return sidecar, hashStr, nil
}

// waitForProxyReady waits for the mitmproxy container to be running and accepting connections.
func (m *Manager) waitForProxyReady(ctx context.Context, containerID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		state, err := m.runtime.InspectContainer(ctx, containerID)
		if err == nil && state == StateRunning {
			// Container is running, verify port 8080 is listening
			// Use nc (netcat) to check if the port is accepting connections
			_, err := m.runtime.Exec(ctx, containerID, []string{"nc", "-z", "localhost", "8080"})
			if err == nil {
				return nil
			}
			// Port not ready yet, continue waiting
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
			// Continue waiting
		}
	}

	return fmt.Errorf("timeout waiting for proxy to be ready after %v", timeout)
}

// destroySidecar stops and removes a sidecar container and its network.
func (m *Manager) destroySidecar(ctx context.Context, sidecar *Sidecar) error {
	m.logger.Info("destroying sidecar",
		"sidecarID", sidecar.ID,
		"type", sidecar.Type,
		"network", sidecar.NetworkName,
	)

	// Stop the sidecar container
	if err := m.runtime.StopContainer(ctx, sidecar.ID); err != nil {
		m.logger.Warn("failed to stop sidecar", "error", err)
		// Continue with removal anyway
	}

	// Remove the sidecar container
	if err := m.runtime.RemoveContainer(ctx, sidecar.ID); err != nil {
		m.logger.Warn("failed to remove sidecar", "error", err)
		// Continue with network cleanup anyway
	}

	// Remove the network
	if sidecar.NetworkName != "" {
		if err := m.runtime.RemoveNetwork(ctx, sidecar.NetworkName); err != nil {
			m.logger.Warn("failed to remove network", "error", err)
		}
	}

	delete(m.sidecars, sidecar.ID)
	return nil
}

// destroySidecarsForProject removes all sidecars associated with a project.
func (m *Manager) destroySidecarsForProject(ctx context.Context, projectPath string) error {
	sidecars := m.GetSidecarsForProject(projectPath)
	for _, sidecar := range sidecars {
		if err := m.destroySidecar(ctx, sidecar); err != nil {
			m.logger.Warn("failed to destroy sidecar", "sidecarID", sidecar.ID, "error", err)
		}
	}
	return nil
}

// startSidecar starts a stopped sidecar container.
func (m *Manager) startSidecar(ctx context.Context, sidecar *Sidecar) error {
	if err := m.runtime.StartContainer(ctx, sidecar.ID); err != nil {
		return fmt.Errorf("failed to start sidecar: %w", err)
	}
	sidecar.State = StateRunning
	return nil
}

// stopSidecar stops a running sidecar container.
func (m *Manager) stopSidecar(ctx context.Context, sidecar *Sidecar) error {
	if err := m.runtime.StopContainer(ctx, sidecar.ID); err != nil {
		return fmt.Errorf("failed to stop sidecar: %w", err)
	}
	sidecar.State = StateStopped
	return nil
}

// startSidecarsForProject starts all sidecars for a project.
func (m *Manager) startSidecarsForProject(ctx context.Context, projectPath string) error {
	sidecars := m.GetSidecarsForProject(projectPath)
	for _, sidecar := range sidecars {
		if sidecar.State != StateRunning {
			if err := m.startSidecar(ctx, sidecar); err != nil {
				return err
			}
		}
	}
	return nil
}

// stopSidecarsForProject stops all sidecars for a project.
func (m *Manager) stopSidecarsForProject(ctx context.Context, projectPath string) error {
	sidecars := m.GetSidecarsForProject(projectPath)
	for _, sidecar := range sidecars {
		if sidecar.State == StateRunning {
			if err := m.stopSidecar(ctx, sidecar); err != nil {
				m.logger.Warn("failed to stop sidecar", "sidecarID", sidecar.ID, "error", err)
			}
		}
	}
	return nil
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

// getTemplate retrieves a template by name from the generator.
// Returns nil if template not found.
func (m *Manager) getTemplate(templateName string) *config.Template {
	if m.generator == nil {
		return nil
	}
	return m.generator.GetTemplate(templateName)
}

// reportProgress calls the progress callback if set.
func reportProgress(opts CreateOptions, step ProgressStep) {
	if opts.OnProgress != nil {
		opts.OnProgress(step)
	}
}

// Create generates a devcontainer.json and starts a new container.
func (m *Manager) Create(ctx context.Context, opts CreateOptions) (*Container, error) {
	scopedLogger := m.logger.With("projectPath", opts.ProjectPath, "template", opts.Template, "name", opts.Name)
	scopedLogger.Info("creating container")

	if m.generator == nil {
		scopedLogger.Error("generator not configured", "error", "generator is nil")
		reportProgress(opts, ProgressStep{Step: "config", Status: "failed", Message: "Generator not configured"})
		return nil, errors.New("generator not configured")
	}

	// Get template to check for isolation config
	tmpl := m.getTemplate(opts.Template)

	// Get effective isolation config (merges template with defaults)
	var effectiveIsolation *config.IsolationConfig
	if tmpl != nil {
		effectiveIsolation = tmpl.GetEffectiveIsolation()
	}

	// Create proxy sidecar if network isolation is configured and has allowlist
	var sidecar *Sidecar
	if effectiveIsolation != nil && effectiveIsolation.Network != nil && len(effectiveIsolation.Network.Allowlist) > 0 {
		// Report network creation started
		reportProgress(opts, ProgressStep{Step: "network", Status: "started", Message: "Creating network..."})

		// Create sidecar first (before devcontainer)
		var projectHash string
		sidecar, projectHash, err := m.createProxySidecar(ctx, opts.ProjectPath, effectiveIsolation.Network)
		if err != nil {
			reportProgress(opts, ProgressStep{Step: "network", Status: "failed", Message: "Failed to create network"})
			return nil, fmt.Errorf("failed to create proxy sidecar: %w", err)
		}

		reportProgress(opts, ProgressStep{Step: "network", Status: "completed", Message: "Network created"})
		reportProgress(opts, ProgressStep{Step: "proxy", Status: "completed", Message: "Proxy sidecar started"})

		// Update CreateOptions with proxy config for Generate()
		certDir, _ := GetProxyCertDir(opts.ProjectPath)
		opts.Proxy = &ProxyConfig{
			CertDir:     certDir,
			ProxyHost:   fmt.Sprintf("devagent-%s-proxy", projectHash),
			ProxyPort:   "8080",
			NetworkName: sidecar.NetworkName,
		}
		scopedLogger.Debug("sidecar created, updating Generate options with proxy config")
	}

	// Report config generation started
	reportProgress(opts, ProgressStep{Step: "config", Status: "started", Message: "Generating config..."})

	// Generate devcontainer.json
	result, err := m.generator.Generate(opts)
	if err != nil {
		scopedLogger.Error("failed to generate devcontainer config", "error", err)
		reportProgress(opts, ProgressStep{Step: "config", Status: "failed", Message: "Failed to generate config"})
		if sidecar != nil {
			// Clean up sidecar on failure
			m.destroySidecar(ctx, sidecar) //nolint:errcheck
		}
		return nil, err
	}

	// Write to project
	if err := m.generator.WriteToProject(opts.ProjectPath, result); err != nil {
		scopedLogger.Error("failed to write devcontainer.json", "error", err)
		reportProgress(opts, ProgressStep{Step: "config", Status: "failed", Message: "Failed to write config"})
		if sidecar != nil {
			// Clean up sidecar on failure
			m.destroySidecar(ctx, sidecar) //nolint:errcheck
		}
		return nil, err
	}

	scopedLogger.Debug("devcontainer.json written")
	reportProgress(opts, ProgressStep{Step: "config", Status: "completed", Message: "Config generated"})

	// Report devcontainer startup started
	reportProgress(opts, ProgressStep{Step: "devcontainer", Status: "started", Message: "Starting devcontainer..."})

	// Start container using devcontainer CLI
	containerID, err := m.devCLI.Up(ctx, opts.ProjectPath)
	if err != nil {
		scopedLogger.Error("devcontainer up failed", "error", err)
		reportProgress(opts, ProgressStep{Step: "devcontainer", Status: "failed", Message: "Failed to start devcontainer"})
		if sidecar != nil {
			// Clean up sidecar on failure
			m.destroySidecar(ctx, sidecar) //nolint:errcheck
		}
		return nil, err
	}

	scopedLogger.Info("container created successfully", "containerID", containerID)
	reportProgress(opts, ProgressStep{Step: "devcontainer", Status: "completed", Message: "Container ready"})

	// Refresh to get the new container
	if err := m.Refresh(ctx); err != nil {
		if sidecar != nil {
			// Clean up sidecar on failure
			m.destroySidecar(ctx, sidecar) //nolint:errcheck
		}
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
		if sidecar != nil {
			// Clean up sidecar on failure
			m.destroySidecar(ctx, sidecar) //nolint:errcheck
		}
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

	// Start sidecars first (using project path from container labels)
	if projectPath := c.ProjectPath; projectPath != "" {
		if err := m.startSidecarsForProject(ctx, projectPath); err != nil {
			scopedLogger.Error("failed to start sidecars", "error", err)
			return fmt.Errorf("failed to start sidecars: %w", err)
		}
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

	// Stop sidecars after container (using project path from container)
	if projectPath := c.ProjectPath; projectPath != "" {
		m.stopSidecarsForProject(ctx, projectPath) //nolint:errcheck
	}

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

	// Destroy sidecars and clean up proxy config directories
	if projectPath := c.ProjectPath; projectPath != "" {
		m.destroySidecarsForProject(ctx, projectPath) //nolint:errcheck
		// Clean up proxy config directories
		CleanupProxyConfigs(projectPath) //nolint:errcheck
	}

	delete(m.containers, id)
	scopedLogger.Info("container destroyed")
	return nil
}

// getContainerUser returns the remote user for a container, defaulting to DefaultRemoteUser.
func (m *Manager) getContainerUser(containerID string) string {
	if c, ok := m.containers[containerID]; ok && c.RemoteUser != "" {
		return c.RemoteUser
	}
	return DefaultRemoteUser
}

// CreateSession creates a tmux session inside a container.
func (m *Manager) CreateSession(ctx context.Context, containerID, sessionName string) error {
	scopedLogger := m.logger.With("containerID", containerID, "session", sessionName)
	scopedLogger.Info("creating tmux session")

	user := m.getContainerUser(containerID)
	_, err := m.runtime.ExecAs(ctx, containerID, user, []string{"tmux", "new-session", "-d", "-s", sessionName})
	if err != nil {
		scopedLogger.Error("failed to create session", "error", err)
		return err
	}

	scopedLogger.Info("session created", "user", user)
	return nil
}

// KillSession destroys a tmux session inside a container.
func (m *Manager) KillSession(ctx context.Context, containerID, sessionName string) error {
	scopedLogger := m.logger.With("containerID", containerID, "session", sessionName)
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
	scopedLogger := m.logger.With("containerID", containerID)
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

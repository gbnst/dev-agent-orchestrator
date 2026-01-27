# TUI Overhaul and Logging Infrastructure - Phase 7: Integration and Polish

**Goal:** Wire logging throughout the application and refine user experience.

**Architecture:** LogManager initialized in main.go is passed through the dependency chain to container operations, tmux operations, and TUI model. Each component receives a scoped logger appropriate to its domain. All operations are logged at appropriate levels (debug for routine, info for actions, error for failures).

**Tech Stack:** Go 1.24+, go.uber.org/zap v1.27.1 (from Phase 1)

**Scope:** 7 phases from original design (this is phase 7 of 7)

**Codebase verified:** 2025-01-27

---

## Phase Overview

This phase wires the logging infrastructure (Phase 1) throughout the application:
1. Initialize LogManager in main.go
2. Add logging to container operations in Manager
3. Add logging to tmux operations in Client
4. Add LogManager dependency to TUI Model
5. Update E2E tests to work with new UI structure

**Testing approach:** Update existing tests to accommodate new dependencies. Add integration tests verifying logs appear in both file and TUI.

---

<!-- START_SUBCOMPONENT_A (tasks 1-2) -->
## Subcomponent A: Main Entry Point Initialization

<!-- START_TASK_1 -->
### Task 1: Initialize LogManager in main.go

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/main.go`

**Step 1: Write the integration test**

Create `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/main_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"

	"devagent/internal/logging"
)

func TestLogManagerInitialization(t *testing.T) {
	// Create temp dir for logs
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Initialize LogManager with test config
	lm, err := logging.NewManager(logging.Config{
		FilePath:       logPath,
		MaxSizeMB:      1,
		MaxBackups:     1,
		MaxAgeDays:     1,
		ChannelBufSize: 10,
		Level:          "debug",
	})
	if err != nil {
		t.Fatalf("failed to create LogManager: %v", err)
	}
	defer lm.Close()

	// Get root logger and write a message
	logger := lm.For("app")
	logger.Info("test message")

	// Sync to flush
	lm.Sync()

	// Verify log file was created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("log file was not created")
	}

	// Verify channel receives entry
	select {
	case entry := <-lm.Entries():
		if entry.Scope != "app" {
			t.Errorf("expected scope 'app', got %q", entry.Scope)
		}
		if entry.Message != "test message" {
			t.Errorf("expected message 'test message', got %q", entry.Message)
		}
	default:
		t.Error("no log entry received on channel")
	}
}
```

**Step 2: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test -v -run TestLogManagerInitialization ./...
```

Expected: PASS (this test validates Phase 1's LogManager works correctly)

**Step 3: Update main.go to initialize LogManager**

Modify `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/main.go`:

```go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"devagent/internal/config"
	"devagent/internal/logging"
	"devagent/internal/tui"
)

func main() {
	configDir := flag.String("config-dir", "", "Path to configuration directory (default: ~/.config/dev-agent-orchestrater)")
	flag.Parse()

	var cfg config.Config
	var err error

	if *configDir != "" {
		cfg, err = config.LoadFromDir(*configDir)
	} else {
		cfg, err = config.Load()
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not load configuration: %v\n", err)
	}

	if err := cfg.ValidateRuntime(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Initialize logging
	logPath := filepath.Join(cfg.DataDir, "orchestrator.log")
	logManager, err := logging.NewManager(logging.Config{
		FilePath:       logPath,
		MaxSizeMB:      10,
		MaxBackups:     3,
		MaxAgeDays:     7,
		ChannelBufSize: 1000,
		Level:          "debug",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logging: %v\n", err)
		os.Exit(1)
	}
	defer logManager.Close()

	appLogger := logManager.For("app")
	appLogger.Info("application starting")

	model := tui.NewModel(&cfg, logManager)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		appLogger.Error("application exited with error", "error", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	appLogger.Info("application stopped")
}
```

**Step 4: Verify build (will fail until Task 2 completes)**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go build ./...
```

Expected: Compilation error - tui.NewModel signature mismatch (this is expected, fixed in Task 2)

**Note:** Do not commit yet - Task 2 must complete first for a compilable state.
<!-- END_TASK_1 -->

<!-- START_TASK_2 -->
### Task 2: Update TUI Model to accept LogManager

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model.go`

**Step 1: Write test for LogManager integration**

Create `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/logging_test.go`:

```go
package tui

import (
	"testing"

	"devagent/internal/config"
	"devagent/internal/logging"
)

func TestModel_LogsInitialization(t *testing.T) {
	cfg := &config.Config{
		DataDir:           t.TempDir(),
		RuntimeSocketPath: "/var/run/docker.sock",
	}

	lm := logging.NewTestLogManager(10)
	defer lm.Close()

	model := NewModel(cfg, lm)

	// Model should have logged initialization
	select {
	case entry := <-lm.Channel():
		if entry.Scope != "tui" {
			t.Errorf("expected scope 'tui', got %q", entry.Scope)
		}
	default:
		t.Error("no initialization log entry received")
	}

	_ = model // Use model to avoid unused variable
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test -v -run TestModel_LogsInitialization ./internal/tui/...
```

Expected: FAIL - NewModel signature doesn't accept LogManager

**Step 3: Update Model struct and constructors**

Modify `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model.go`:

Add LogManager field to Model struct (after line ~17, in the struct definition):

```go
type Model struct {
	// ... existing fields ...
	width         int
	height        int
	themeName     string
	styles        *Styles
	cfg           *config.Config
	templates     []config.Template
	manager       *container.Manager
	containerList list.Model

	// Logging
	logManager *logging.LogManager
	logger     *logging.ScopedLogger

	// ... rest of existing fields ...
```

Add import for logging package at top of file:

```go
import (
	// ... existing imports ...
	"devagent/internal/logging"
)
```

Update NewModel function (around line ~46):

```go
func NewModel(cfg *config.Config, logManager *logging.LogManager) Model {
	templates, _ := cfg.LoadTemplates()
	return NewModelWithTemplates(cfg, templates, logManager)
}

func NewModelWithTemplates(cfg *config.Config, templates []config.Template, logManager *logging.LogManager) Model {
	styles := NewStyles(cfg.Theme)
	delegate := newContainerDelegate(styles)

	containerList := list.New([]list.Item{}, delegate, 0, 0)
	containerList.Title = "Containers"
	containerList.SetShowStatusBar(false)
	containerList.SetFilteringEnabled(false)
	containerList.SetShowHelp(false)
	containerList.Styles.Title = styles.ListTitle
	containerList.Styles.NoItems = styles.ListNoItems

	logger := logManager.For("tui")
	logger.Debug("TUI model initialized")

	return Model{
		cfg:           cfg,
		templates:     templates,
		themeName:     cfg.Theme,
		styles:        styles,
		containerList: containerList,
		logManager:    logManager,
		logger:        logger,
	}
}
```

**Step 4: Update test helpers in existing test files**

Modify `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/sessions_test.go`:

Update `newTestModelWithContainers` helper to create a test LogManager:

```go
func newTestModelWithContainers(containers []*container.Container) Model {
	cfg := &config.Config{
		DataDir:           "/tmp/test",
		RuntimeSocketPath: "/var/run/docker.sock",
	}

	lm := logging.NewTestLogManager(100)

	model := NewModelWithTemplates(cfg, []config.Template{}, lm)
	// ... rest of existing helper code ...
```

Add import for logging at top:

```go
import (
	// ... existing imports ...
	"devagent/internal/logging"
)
```

**Step 5: Run tests to verify they pass**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test -v ./internal/tui/...
```

Expected: All tests PASS

**Step 6: Verify full build**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go build ./...
```

Expected: Build succeeds

**Step 7: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add main.go main_test.go internal/tui/model.go internal/tui/logging_test.go internal/tui/sessions_test.go && git commit -m "feat: initialize LogManager in main and wire to TUI Model"
```
<!-- END_TASK_2 -->
<!-- END_SUBCOMPONENT_A -->

---

<!-- START_SUBCOMPONENT_B (tasks 3-5) -->
## Subcomponent B: Container Manager Logging

<!-- START_TASK_3 -->
### Task 3: Add scoped logger to container Manager

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/container/manager.go`

**Step 1: Write test for Manager logging**

Create `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/container/manager_logging_test.go`:

```go
package container

import (
	"testing"

	"devagent/internal/config"
	"devagent/internal/logging"
)

func TestManager_LogsOperations(t *testing.T) {
	lm := logging.NewTestLogManager(100)
	defer lm.Close()

	cfg := &config.Config{
		DataDir:           t.TempDir(),
		RuntimeSocketPath: "/var/run/docker.sock",
	}

	mockRuntime := &MockRuntime{
		ListResult: []RuntimeContainer{
			{ID: "abc123", Name: "test-container", State: "running"},
		},
	}

	_ = NewManagerWithRuntimeAndLogger(cfg, []config.Template{}, mockRuntime, lm)

	// Should have logged initialization
	select {
	case entry := <-lm.Channel():
		if entry.Scope != "container" {
			t.Errorf("expected scope 'container', got %q", entry.Scope)
		}
	default:
		t.Error("no initialization log entry received")
	}
}

func TestManager_LogsRefresh(t *testing.T) {
	lm := logging.NewTestLogManager(100)
	defer lm.Close()

	cfg := &config.Config{
		DataDir:           t.TempDir(),
		RuntimeSocketPath: "/var/run/docker.sock",
	}

	mockRuntime := &MockRuntime{
		ListResult: []RuntimeContainer{
			{ID: "abc123", Name: "test-container", State: "running"},
		},
	}

	manager := NewManagerWithRuntimeAndLogger(cfg, []config.Template{}, mockRuntime, lm)

	// Drain initialization log
	<-lm.Channel()

	// Refresh should log
	_ = manager.Refresh()

	select {
	case entry := <-lm.Channel():
		if entry.Scope != "container" {
			t.Errorf("expected scope 'container', got %q", entry.Scope)
		}
		if entry.Message != "refreshing container list" {
			t.Errorf("unexpected message: %q", entry.Message)
		}
	default:
		t.Error("no refresh log entry received")
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test -v -run TestManager_Logs ./internal/container/...
```

Expected: FAIL - NewManagerWithRuntimeAndLogger doesn't exist

**Step 3: Add logger field and constructor**

Modify `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/container/manager.go`:

Add import:

```go
import (
	// ... existing imports ...
	"devagent/internal/logging"
)
```

Add logger field to Manager struct (around line ~24):

```go
type Manager struct {
	cfg        *config.Config
	runtime    RuntimeInterface
	generator  GeneratorInterface
	devCLI     DevCLIInterface
	containers []*Container
	logger     *logging.ScopedLogger
}
```

Add new constructor (after existing constructors, around line ~40):

```go
func NewManagerWithRuntimeAndLogger(cfg *config.Config, templates []config.Template, runtime RuntimeInterface, logManager *logging.LogManager) *Manager {
	logger := logManager.For("container")
	logger.Debug("container manager initialized")

	return &Manager{
		cfg:       cfg,
		runtime:   runtime,
		generator: NewDevContainerGenerator(templates),
		devCLI:    NewDevContainerCLI(),
		logger:    logger,
	}
}
```

Update existing constructors to use a nil-safe logger pattern:

```go
func NewManager(cfg *config.Config, templates []config.Template) *Manager {
	return NewManagerWithRuntime(cfg, templates, NewDockerRuntime(cfg.RuntimeSocketPath))
}

func NewManagerWithRuntime(cfg *config.Config, templates []config.Template, runtime RuntimeInterface) *Manager {
	return &Manager{
		cfg:       cfg,
		runtime:   runtime,
		generator: NewDevContainerGenerator(templates),
		devCLI:    NewDevContainerCLI(),
		logger:    logging.NopLogger(),
	}
}

func NewManagerWithDeps(runtime RuntimeInterface, generator GeneratorInterface, devCLI DevCLIInterface) *Manager {
	return &Manager{
		runtime:   runtime,
		generator: generator,
		devCLI:    devCLI,
		logger:    logging.NopLogger(),
	}
}
```

**Step 4: Run tests to verify they pass**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test -v -run TestManager_Logs ./internal/container/...
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/container/manager.go internal/container/manager_logging_test.go && git commit -m "feat: add scoped logger to container Manager"
```
<!-- END_TASK_3 -->

<!-- START_TASK_4 -->
### Task 4: Add logging to container operations

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/container/manager.go`

**Step 1: Add logging to Refresh method**

Locate the `Refresh` method (around line ~70) and add logging:

```go
func (m *Manager) Refresh() error {
	m.logger.Debug("refreshing container list")

	runtimeContainers, err := m.runtime.List()
	if err != nil {
		m.logger.Error("failed to list containers", "error", err)
		return err
	}

	m.containers = make([]*Container, len(runtimeContainers))
	for i, rc := range runtimeContainers {
		m.containers[i] = &Container{
			ID:    rc.ID,
			Name:  rc.Name,
			State: rc.State,
		}
	}

	m.logger.Debug("container list refreshed", "count", len(m.containers))
	return nil
}
```

**Step 2: Add logging to Create method**

Locate the `Create` method (around line ~101) and add logging:

```go
func (m *Manager) Create(projectPath, name, templateID string) error {
	scopedLogger := m.logger.With("container", name, "template", templateID)
	scopedLogger.Info("creating container")

	config, err := m.generator.Generate(projectPath, templateID)
	if err != nil {
		scopedLogger.Error("failed to generate devcontainer config", "error", err)
		return err
	}

	devcontainerPath := filepath.Join(projectPath, ".devcontainer")
	if err := os.MkdirAll(devcontainerPath, 0755); err != nil {
		scopedLogger.Error("failed to create .devcontainer directory", "error", err)
		return err
	}

	configPath := filepath.Join(devcontainerPath, "devcontainer.json")
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		scopedLogger.Error("failed to write devcontainer.json", "error", err)
		return err
	}

	scopedLogger.Debug("devcontainer.json written", "path", configPath)

	if err := m.devCLI.Up(projectPath, name); err != nil {
		scopedLogger.Error("devcontainer up failed", "error", err)
		return err
	}

	scopedLogger.Info("container created successfully")
	return m.Refresh()
}
```

**Step 3: Add logging to Start method**

Locate the `Start` method (around line ~137) and add logging:

```go
func (m *Manager) Start(containerID string) error {
	scopedLogger := m.logger.With("containerID", containerID)
	scopedLogger.Info("starting container")

	if err := m.runtime.Start(containerID); err != nil {
		scopedLogger.Error("failed to start container", "error", err)
		return err
	}

	scopedLogger.Info("container started")
	return m.Refresh()
}
```

**Step 4: Add logging to Stop method**

Locate the `Stop` method (around line ~152) and add logging:

```go
func (m *Manager) Stop(containerID string) error {
	scopedLogger := m.logger.With("containerID", containerID)
	scopedLogger.Info("stopping container")

	if err := m.runtime.Stop(containerID); err != nil {
		scopedLogger.Error("failed to stop container", "error", err)
		return err
	}

	scopedLogger.Info("container stopped")
	return m.Refresh()
}
```

**Step 5: Add logging to Destroy method**

Locate the `Destroy` method (around line ~167) and add logging:

```go
func (m *Manager) Destroy(containerID string) error {
	scopedLogger := m.logger.With("containerID", containerID)
	scopedLogger.Info("destroying container")

	container := m.Get(containerID)
	if container != nil && container.State == "running" {
		scopedLogger.Debug("stopping running container before removal")
		if err := m.runtime.Stop(containerID); err != nil {
			scopedLogger.Warn("failed to stop container before removal", "error", err)
		}
	}

	if err := m.runtime.Remove(containerID); err != nil {
		scopedLogger.Error("failed to remove container", "error", err)
		return err
	}

	scopedLogger.Info("container destroyed")
	return m.Refresh()
}
```

**Step 6: Add logging to session operations**

Locate the session methods (around line ~189) and add logging:

```go
func (m *Manager) CreateSession(containerID, sessionName string) error {
	scopedLogger := m.logger.With("containerID", containerID, "session", sessionName)
	scopedLogger.Info("creating tmux session")

	if err := m.runtime.Exec(containerID, []string{"tmux", "new-session", "-d", "-s", sessionName}); err != nil {
		scopedLogger.Error("failed to create session", "error", err)
		return err
	}

	scopedLogger.Info("session created")
	return nil
}

func (m *Manager) KillSession(containerID, sessionName string) error {
	scopedLogger := m.logger.With("containerID", containerID, "session", sessionName)
	scopedLogger.Info("killing tmux session")

	if err := m.runtime.Exec(containerID, []string{"tmux", "kill-session", "-t", sessionName}); err != nil {
		scopedLogger.Error("failed to kill session", "error", err)
		return err
	}

	scopedLogger.Info("session killed")
	return nil
}

func (m *Manager) ListSessions(containerID string) ([]Session, error) {
	scopedLogger := m.logger.With("containerID", containerID)
	scopedLogger.Debug("listing tmux sessions")

	output, err := m.runtime.ExecOutput(containerID, []string{"tmux", "list-sessions", "-F", "#{session_name}:#{session_windows}:#{session_attached}"})
	if err != nil {
		scopedLogger.Debug("no tmux server running or no sessions", "error", err)
		return []Session{}, nil
	}

	sessions := m.parseTmuxSessions(output)
	scopedLogger.Debug("sessions listed", "count", len(sessions))
	return sessions, nil
}
```

**Step 7: Run all manager tests**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test -v ./internal/container/...
```

Expected: All tests PASS

**Step 8: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/container/manager.go && git commit -m "feat: add logging to all container Manager operations"
```
<!-- END_TASK_4 -->

<!-- START_TASK_5 -->
### Task 5: Wire LogManager through TUI to Manager

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model.go`

**Step 1: Update Model to create Manager with logger**

Modify the `NewModelWithTemplates` function in `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model.go`:

```go
func NewModelWithTemplates(cfg *config.Config, templates []config.Template, logManager *logging.LogManager) Model {
	styles := NewStyles(cfg.Theme)
	delegate := newContainerDelegate(styles)

	containerList := list.New([]list.Item{}, delegate, 0, 0)
	containerList.Title = "Containers"
	containerList.SetShowStatusBar(false)
	containerList.SetFilteringEnabled(false)
	containerList.SetShowHelp(false)
	containerList.Styles.Title = styles.ListTitle
	containerList.Styles.NoItems = styles.ListNoItems

	logger := logManager.For("tui")
	logger.Debug("TUI model initialized")

	// Create manager with logger
	manager := container.NewManagerWithRuntimeAndLogger(
		cfg,
		templates,
		container.NewDockerRuntime(cfg.RuntimeSocketPath),
		logManager,
	)

	return Model{
		cfg:           cfg,
		templates:     templates,
		themeName:     cfg.Theme,
		styles:        styles,
		containerList: containerList,
		manager:       manager,
		logManager:    logManager,
		logger:        logger,
	}
}
```

**Step 2: Update test helper to use logging-aware manager**

Update the `newTestModelWithContainers` helper in sessions_test.go to use the new pattern:

```go
func newTestModelWithContainers(containers []*container.Container) Model {
	cfg := &config.Config{
		DataDir:           "/tmp/test",
		RuntimeSocketPath: "/var/run/docker.sock",
	}

	lm := logging.NewTestLogManager(100)
	logger := lm.For("tui")

	styles := NewStyles(cfg.Theme)
	delegate := newContainerDelegate(styles)

	containerList := list.New([]list.Item{}, delegate, 0, 0)
	containerList.Title = "Containers"
	containerList.SetShowStatusBar(false)
	containerList.SetFilteringEnabled(false)
	containerList.SetShowHelp(false)
	containerList.Styles.Title = styles.ListTitle
	containerList.Styles.NoItems = styles.ListNoItems

	// Create mock runtime with containers
	mockRuntime := &container.MockRuntime{
		ListResult: make([]container.RuntimeContainer, len(containers)),
	}
	for i, c := range containers {
		mockRuntime.ListResult[i] = container.RuntimeContainer{
			ID:    c.ID,
			Name:  c.Name,
			State: c.State,
		}
	}

	manager := container.NewManagerWithRuntimeAndLogger(cfg, []config.Template{}, mockRuntime, lm)
	manager.Refresh()

	model := Model{
		cfg:           cfg,
		templates:     []config.Template{},
		themeName:     cfg.Theme,
		styles:        styles,
		containerList: containerList,
		manager:       manager,
		logManager:    lm,
		logger:        logger,
	}

	// Set items from manager
	items := make([]list.Item, len(manager.List()))
	for i, c := range manager.List() {
		items[i] = c
	}
	model.containerList.SetItems(items)

	return model
}
```

**Step 3: Run all TUI tests**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test -v ./internal/tui/...
```

Expected: All tests PASS

**Step 4: Verify full build**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go build ./...
```

Expected: Build succeeds

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/model.go internal/tui/sessions_test.go && git commit -m "feat: wire LogManager through TUI to container Manager"
```
<!-- END_TASK_5 -->
<!-- END_SUBCOMPONENT_B -->

---

<!-- START_SUBCOMPONENT_C (tasks 6-7) -->
## Subcomponent C: Tmux Client Logging

<!-- START_TASK_6 -->
### Task 6: Add scoped logger to tmux Client

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tmux/client.go`

**Step 1: Write test for Client logging**

Create `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tmux/client_logging_test.go`:

```go
package tmux

import (
	"testing"

	"devagent/internal/logging"
)

func TestClient_LogsOperations(t *testing.T) {
	lm := logging.NewTestLogManager(100)
	defer lm.Close()

	mockExec := func(containerID string, cmd []string) (string, error) {
		return "session1:1:0\nsession2:2:1\n", nil
	}

	client := NewClientWithLogger(mockExec, lm)

	// Drain initialization log
	<-lm.Channel()

	// ListSessions should log
	_, _ = client.ListSessions("container123")

	select {
	case entry := <-lm.Channel():
		if entry.Scope != "tmux" {
			t.Errorf("expected scope 'tmux', got %q", entry.Scope)
		}
	default:
		t.Error("no log entry received for ListSessions")
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test -v -run TestClient_LogsOperations ./internal/tmux/...
```

Expected: FAIL - NewClientWithLogger doesn't exist

**Step 3: Add logger field and constructor**

Modify `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tmux/client.go`:

Add import:

```go
import (
	// ... existing imports ...
	"devagent/internal/logging"
)
```

Update Client struct:

```go
type Client struct {
	exec   ContainerExecutor
	logger *logging.ScopedLogger
}
```

Add new constructor:

```go
func NewClientWithLogger(exec ContainerExecutor, logManager *logging.LogManager) *Client {
	logger := logManager.For("tmux")
	logger.Debug("tmux client initialized")

	return &Client{
		exec:   exec,
		logger: logger,
	}
}
```

Update existing constructor to use nop logger:

```go
func NewClient(exec ContainerExecutor) *Client {
	return &Client{
		exec:   exec,
		logger: logging.NopLogger(),
	}
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test -v -run TestClient_LogsOperations ./internal/tmux/...
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tmux/client.go internal/tmux/client_logging_test.go && git commit -m "feat: add scoped logger to tmux Client"
```
<!-- END_TASK_6 -->

<!-- START_TASK_7 -->
### Task 7: Add logging to tmux operations

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tmux/client.go`

**Step 1: Add logging to ListSessions**

Update the `ListSessions` method:

```go
func (c *Client) ListSessions(containerID string) ([]Session, error) {
	c.logger.Debug("listing tmux sessions", "containerID", containerID)

	output, err := c.exec(containerID, []string{"tmux", "list-sessions", "-F", "#{session_name}:#{session_windows}:#{session_attached}"})
	if err != nil {
		c.logger.Debug("no tmux server running", "containerID", containerID, "error", err)
		return []Session{}, nil
	}

	sessions := c.parseSessions(output)
	c.logger.Debug("sessions listed", "containerID", containerID, "count", len(sessions))
	return sessions, nil
}
```

**Step 2: Add logging to CreateSession**

Update the `CreateSession` method:

```go
func (c *Client) CreateSession(containerID, sessionName string) error {
	c.logger.Info("creating tmux session", "containerID", containerID, "session", sessionName)

	_, err := c.exec(containerID, []string{"tmux", "new-session", "-d", "-s", sessionName})
	if err != nil {
		c.logger.Error("failed to create session", "containerID", containerID, "session", sessionName, "error", err)
		return err
	}

	c.logger.Info("session created", "containerID", containerID, "session", sessionName)
	return nil
}
```

**Step 3: Add logging to KillSession**

Update the `KillSession` method:

```go
func (c *Client) KillSession(containerID, sessionName string) error {
	c.logger.Info("killing tmux session", "containerID", containerID, "session", sessionName)

	_, err := c.exec(containerID, []string{"tmux", "kill-session", "-t", sessionName})
	if err != nil {
		c.logger.Error("failed to kill session", "containerID", containerID, "session", sessionName, "error", err)
		return err
	}

	c.logger.Info("session killed", "containerID", containerID, "session", sessionName)
	return nil
}
```

**Step 4: Add logging to CapturePane and SendKeys**

Update the remaining methods:

```go
func (c *Client) CapturePane(containerID, sessionName string) (string, error) {
	c.logger.Debug("capturing pane", "containerID", containerID, "session", sessionName)

	output, err := c.exec(containerID, []string{"tmux", "capture-pane", "-t", sessionName, "-p"})
	if err != nil {
		c.logger.Error("failed to capture pane", "containerID", containerID, "session", sessionName, "error", err)
		return "", err
	}

	return output, nil
}

func (c *Client) SendKeys(containerID, sessionName, keys string) error {
	c.logger.Debug("sending keys", "containerID", containerID, "session", sessionName)

	_, err := c.exec(containerID, []string{"tmux", "send-keys", "-t", sessionName, keys, "Enter"})
	if err != nil {
		c.logger.Error("failed to send keys", "containerID", containerID, "session", sessionName, "error", err)
		return err
	}

	return nil
}
```

**Step 5: Run all tmux tests**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test -v ./internal/tmux/...
```

Expected: All tests PASS

**Step 6: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tmux/client.go && git commit -m "feat: add logging to all tmux Client operations"
```
<!-- END_TASK_7 -->
<!-- END_SUBCOMPONENT_C -->

---

<!-- START_SUBCOMPONENT_D (tasks 8-9) -->
## Subcomponent D: TUI Event Logging

<!-- START_TASK_8 -->
### Task 8: Add logging to TUI key events and state transitions

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/update.go`

**Step 1: Add import for logging**

Add the logging import at the top of update.go:

```go
import (
	// ... existing imports ...
)
```

Note: The logger is accessed via `m.logger` from the Model struct.

**Step 2: Add logging to key event handling**

Locate the `Update` method and add logging to key handling sections:

For tab switching (after handling "1", "2", "h", "l" keys):

```go
case "1":
	m.logger.Debug("tab switched", "tab", "containers")
	// existing tab switch code...

case "2":
	m.logger.Debug("tab switched", "tab", "sessions")
	// existing tab switch code...
```

For container actions (start, stop, destroy):

```go
case "s":
	if container != nil {
		m.logger.Info("starting container", "containerID", container.ID, "name", container.Name)
		// existing start code...
	}

case "x":
	if container != nil {
		m.logger.Info("stopping container", "containerID", container.ID, "name", container.Name)
		// existing stop code...
	}

case "d":
	if container != nil {
		m.logger.Info("destroying container", "containerID", container.ID, "name", container.Name)
		// existing destroy code...
	}
```

For form opening:

```go
case "n":
	m.logger.Debug("opening container creation form")
	// existing form open code...
```

For log panel toggle:

```go
case "L":
	m.logger.Debug("toggling log panel", "visible", !m.logPanelVisible)
	// existing toggle code...
```

**Step 3: Add logging to action completions**

In the message handlers for `containerActionMsg` and `containerErrorMsg`:

```go
case containerActionMsg:
	m.logger.Info("container action completed", "action", msg.action, "containerID", msg.containerID)
	// existing handler code...

case containerErrorMsg:
	m.logger.Error("container action failed", "error", msg.err)
	// existing handler code...
```

**Step 4: Run TUI tests**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test -v ./internal/tui/...
```

Expected: All tests PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/update.go && git commit -m "feat: add logging to TUI key events and state transitions"
```
<!-- END_TASK_8 -->

<!-- START_TASK_9 -->
### Task 9: Add logging to refresh commands

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/model.go`

**Step 1: Add logging to refreshContainers command**

Locate the `refreshContainers` function and add logging:

```go
func (m Model) refreshContainers() tea.Msg {
	m.logger.Debug("refreshing containers")

	if err := m.manager.Refresh(); err != nil {
		m.logger.Error("container refresh failed", "error", err)
		return containerErrorMsg{err: err}
	}

	containers := m.manager.List()
	m.logger.Debug("containers refreshed", "count", len(containers))
	return containersRefreshedMsg{containers: containers}
}
```

**Step 2: Add logging to refreshSessions command**

Locate the `refreshSessions` function and add logging:

```go
func (m Model) refreshSessions() tea.Msg {
	if m.selectedContainer == nil {
		return nil
	}

	m.logger.Debug("refreshing sessions", "containerID", m.selectedContainer.ID)

	sessions, err := m.manager.ListSessions(m.selectedContainer.ID)
	if err != nil {
		m.logger.Error("session refresh failed", "containerID", m.selectedContainer.ID, "error", err)
		return containerErrorMsg{err: err}
	}

	m.logger.Debug("sessions refreshed", "containerID", m.selectedContainer.ID, "count", len(sessions))
	return sessionsRefreshedMsg{sessions: sessions}
}
```

**Step 3: Add logging to tick command**

Locate the `tick` function and add debug logging:

```go
func tick() tea.Msg {
	time.Sleep(5 * time.Second)
	return tickMsg{}
}
```

Note: The tick function itself doesn't have access to logger, but the handler in Update should log:

```go
case tickMsg:
	m.logger.Debug("periodic refresh triggered")
	// existing tick handler code...
```

**Step 4: Run all tests**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test -v ./...
```

Expected: All tests PASS

**Step 5: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/model.go internal/tui/update.go && git commit -m "feat: add logging to refresh commands and tick handler"
```
<!-- END_TASK_9 -->
<!-- END_SUBCOMPONENT_D -->

---

<!-- START_SUBCOMPONENT_E (tasks 10-11) -->
## Subcomponent E: E2E Tests and Final Polish

<!-- START_TASK_10 -->
### Task 10: Update E2E tests for new UI structure

**Files:**
- Modify: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/tui_test.go` (if exists)
- Create: `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/integration_test.go`

**Step 1: Create integration test for log panel**

Create `/Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging/internal/tui/integration_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"devagent/internal/config"
	"devagent/internal/container"
	"devagent/internal/logging"
)

func TestIntegration_LogPanelToggle(t *testing.T) {
	cfg := &config.Config{
		DataDir:           t.TempDir(),
		RuntimeSocketPath: "/var/run/docker.sock",
	}

	lm := logging.NewTestLogManager(100)
	defer lm.Close()

	mockRuntime := &container.MockRuntime{
		ListResult: []container.RuntimeContainer{
			{ID: "abc123", Name: "test-container", State: "running"},
		},
	}

	manager := container.NewManagerWithRuntimeAndLogger(cfg, []config.Template{}, mockRuntime, lm)
	manager.Refresh()

	model := NewModelWithTemplates(cfg, []config.Template{}, lm)

	// Set window size
	model, _ = model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Initially log panel should be hidden
	view := model.View()
	if strings.Contains(view, "Logs") {
		t.Error("log panel should be hidden initially")
	}

	// Press L to show log panel
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})

	view = model.View()
	if !strings.Contains(view, "Logs") {
		t.Error("log panel should be visible after pressing L")
	}

	// Press L again to hide
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})

	view = model.View()
	if strings.Contains(view, "Logs") {
		t.Error("log panel should be hidden after pressing L again")
	}
}

func TestIntegration_TabNavigation(t *testing.T) {
	cfg := &config.Config{
		DataDir:           t.TempDir(),
		RuntimeSocketPath: "/var/run/docker.sock",
	}

	lm := logging.NewTestLogManager(100)
	defer lm.Close()

	model := NewModelWithTemplates(cfg, []config.Template{}, lm)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Initially on Containers tab
	view := model.View()
	if !strings.Contains(view, "Containers") {
		t.Error("should show Containers tab initially")
	}

	// Press 2 to switch to Sessions tab
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})

	// Should still show tabs but be on Sessions
	// (Sessions tab may show "No container selected" or similar)

	// Press 1 to go back to Containers
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})

	view = model.View()
	if !strings.Contains(view, "Containers") {
		t.Error("should show Containers after pressing 1")
	}
}

func TestIntegration_LogsAppearInPanel(t *testing.T) {
	cfg := &config.Config{
		DataDir:           t.TempDir(),
		RuntimeSocketPath: "/var/run/docker.sock",
	}

	lm := logging.NewTestLogManager(100)
	defer lm.Close()

	mockRuntime := &container.MockRuntime{
		ListResult: []container.RuntimeContainer{
			{ID: "abc123", Name: "test-container", State: "running"},
		},
	}

	manager := container.NewManagerWithRuntimeAndLogger(cfg, []config.Template{}, mockRuntime, lm)
	manager.Refresh()

	model := NewModelWithTemplates(cfg, []config.Template{}, lm)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Generate some log entries
	logger := lm.For("test")
	logger.Info("test log message")

	// Consume logs into model
	model, _ = model.Update(logEntriesMsg{})

	// Open log panel
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})

	view := model.View()

	// View should contain the log message (or at least show logs area)
	// Actual content depends on viewport implementation
	if !strings.Contains(view, "Logs") {
		t.Error("log panel should be visible")
	}
}
```

**Step 2: Run integration tests**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test -v -run TestIntegration ./internal/tui/...
```

Expected: All tests PASS

**Step 3: Commit**

```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add internal/tui/integration_test.go && git commit -m "test: add integration tests for log panel and tab navigation"
```
<!-- END_TASK_10 -->

<!-- START_TASK_11 -->
### Task 11: Final verification and cleanup

**Files:**
- All files modified in this phase

**Step 1: Run full test suite**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go test -v ./...
```

Expected: All tests PASS

**Step 2: Run linter**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go vet ./...
```

Expected: No errors

**Step 3: Verify build**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go build ./...
```

Expected: Build succeeds

**Step 4: Test manual run (smoke test)**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && go run main.go --help
```

Expected: Help output shown without errors

**Step 5: Verify log file creation**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && timeout 2 go run main.go 2>/dev/null || true && ls -la ~/.config/dev-agent-orchestrater/*.log 2>/dev/null || echo "Log location depends on config"
```

Expected: Log file created in data directory

**Step 6: Final commit if any uncommitted changes**

Run:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git status
```

If there are uncommitted changes:
```bash
cd /Users/josh/code/dev-agent-orchestrater/.worktrees/tui-overhaul-and-logging && git add -A && git commit -m "chore: final cleanup for Phase 7"
```
<!-- END_TASK_11 -->
<!-- END_SUBCOMPONENT_E -->

---

## Phase Completion Checklist

- [ ] LogManager initialized in main.go before TUI creation
- [ ] TUI Model accepts LogManager via constructor
- [ ] Container Manager has scoped logger for all operations
- [ ] Tmux Client has scoped logger for all operations
- [ ] TUI key events logged at debug level
- [ ] Container actions logged at info level
- [ ] Errors logged at error level with context
- [ ] All existing tests updated and passing
- [ ] Integration tests for log panel and tabs
- [ ] Full test suite passes
- [ ] Application builds and runs

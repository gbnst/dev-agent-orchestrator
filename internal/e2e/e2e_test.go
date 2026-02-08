//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/container"
	"devagent/internal/tui"
)

// testCreateContainer tests creating a container with the specified runtime.
func testCreateContainer(t *testing.T, runtime string) {
	SkipIfRuntimeMissing(t, runtime)
	SkipIfDevcontainerMissing(t)

	cfg := TestConfig(runtime)
	templates := TestTemplates()
	projectDir := TestProject(t, "basic")

	// Create logging manager for the TUI model
	logMgr := TestLogManager(t)

	// Create model and test runner
	model := tui.NewModelWithTemplates(cfg, templates, logMgr)
	runner := NewTUITestRunner(t, model)

	// Initialize
	runner.SendWindowSize(120, 40)
	runner.Init()

	// Open create form
	runner.PressKey('c')
	if !runner.Model().FormOpen() {
		t.Fatal("Expected form to be open after pressing 'c'")
	}

	// Tab to project path field
	runner.PressSpecialKey(tea.KeyTab)

	// Type project path
	runner.TypeText(projectDir)

	// Tab to name field
	runner.PressSpecialKey(tea.KeyTab)

	// Type container name
	containerName := fmt.Sprintf("e2e-test-%s-%d", runtime, time.Now().UnixNano())
	runner.TypeText(containerName)

	// Submit form
	runner.PressSpecialKey(tea.KeyEnter)

	// Form should be closed
	if runner.Model().FormOpen() {
		t.Error("Expected form to be closed after submit")
	}

	// Wait for container to appear
	if !runner.WaitForContainerCount(1, 90*time.Second) {
		t.Fatalf("Container not created within timeout")
	}

	// Verify container exists
	c, ok := runner.Model().GetContainerByName(containerName)
	if !ok {
		t.Fatalf("Container %q not found in list", containerName)
	}

	// Cleanup
	t.Cleanup(func() {
		CleanupContainer(t, runtime, c.ID)
	})

	t.Logf("Successfully created container %s with ID %s", containerName, c.ID)
}

// testStartStopContainer tests starting and stopping a container.
func testStartStopContainer(t *testing.T, runtime string) {
	SkipIfRuntimeMissing(t, runtime)
	SkipIfDevcontainerMissing(t)

	cfg := TestConfig(runtime)
	templates := TestTemplates()
	projectDir := TestProject(t, "basic")

	// Create logging manager for the TUI model
	logMgr := TestLogManager(t)

	// Create model and test runner
	model := tui.NewModelWithTemplates(cfg, templates, logMgr)
	runner := NewTUITestRunner(t, model)

	// Initialize
	runner.SendWindowSize(120, 40)
	runner.Init()

	// First create a container
	containerName := fmt.Sprintf("e2e-startstop-%s-%d", runtime, time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	c, err := runner.Model().Manager().CreateWithCompose(ctx, container.CreateOptions{
		ProjectPath: projectDir,
		Template:    "basic",
		Name:        containerName,
	})
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_ = runner.Model().Manager().DestroyWithCompose(cleanupCtx, c.ID)
	})

	// Refresh to see the container
	runner.PressKey('r')
	time.Sleep(2 * time.Second)

	// Should have 1 container now
	if runner.Model().ContainerCount() != 1 {
		t.Fatalf("Expected 1 container, got %d", runner.Model().ContainerCount())
	}

	// Container should be running
	c, ok := runner.Model().GetContainerByName(containerName)
	if !ok {
		t.Fatalf("Container %q not found", containerName)
	}
	if c.State != container.StateRunning {
		t.Errorf("Expected running state, got %s", c.State)
	}

	// Stop the container
	runner.PressKey('x')
	time.Sleep(5 * time.Second)
	runner.PressKey('r')
	time.Sleep(2 * time.Second)

	c, ok = runner.Model().GetContainerByName(containerName)
	if !ok {
		t.Fatalf("Container %q not found after stop", containerName)
	}
	if c.State != container.StateStopped {
		t.Errorf("Expected stopped state after 'x', got %s", c.State)
	}

	// Start the container
	runner.PressKey('s')
	time.Sleep(5 * time.Second)
	runner.PressKey('r')
	time.Sleep(2 * time.Second)

	c, ok = runner.Model().GetContainerByName(containerName)
	if !ok {
		t.Fatalf("Container %q not found after start", containerName)
	}
	if c.State != container.StateRunning {
		t.Errorf("Expected running state after 's', got %s", c.State)
	}

	t.Logf("Successfully tested start/stop for container %s", containerName)
}

// testDestroyContainer tests destroying a container.
func testDestroyContainer(t *testing.T, runtime string) {
	SkipIfRuntimeMissing(t, runtime)
	SkipIfDevcontainerMissing(t)

	cfg := TestConfig(runtime)
	templates := TestTemplates()
	projectDir := TestProject(t, "basic")

	// Create logging manager for the TUI model
	logMgr := TestLogManager(t)

	// Create model and test runner
	model := tui.NewModelWithTemplates(cfg, templates, logMgr)
	runner := NewTUITestRunner(t, model)

	// Initialize
	runner.SendWindowSize(120, 40)
	runner.Init()

	// First create a container
	containerName := fmt.Sprintf("e2e-destroy-%s-%d", runtime, time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	c, err := runner.Model().Manager().CreateWithCompose(ctx, container.CreateOptions{
		ProjectPath: projectDir,
		Template:    "basic",
		Name:        containerName,
	})
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}

	containerID := c.ID

	// Refresh to see the container
	runner.PressKey('r')
	time.Sleep(2 * time.Second)

	// Should have 1 container
	if runner.Model().ContainerCount() != 1 {
		t.Fatalf("Expected 1 container, got %d", runner.Model().ContainerCount())
	}

	// Destroy the container
	runner.PressKey('d')
	time.Sleep(5 * time.Second)
	runner.PressKey('r')
	time.Sleep(2 * time.Second)

	// Should have 0 containers
	if runner.Model().ContainerCount() != 0 {
		t.Errorf("Expected 0 containers after destroy, got %d", runner.Model().ContainerCount())
		// Try to clean up anyway
		CleanupContainer(t, runtime, containerID)
	}

	t.Logf("Successfully destroyed container %s", containerName)
}

// testCreateTmuxSession tests creating a tmux session inside a container.
func testCreateTmuxSession(t *testing.T, runtime string) {
	SkipIfRuntimeMissing(t, runtime)
	SkipIfDevcontainerMissing(t)

	cfg := TestConfig(runtime)
	templates := TestTemplates()
	projectDir := TestProject(t, "basic")

	// Create logging manager for the TUI model
	logMgr := TestLogManager(t)

	// Create model and test runner
	model := tui.NewModelWithTemplates(cfg, templates, logMgr)
	runner := NewTUITestRunner(t, model)

	// Initialize
	runner.SendWindowSize(120, 40)
	runner.Init()

	// First create a container
	containerName := fmt.Sprintf("e2e-tmux-%s-%d", runtime, time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	c, err := runner.Model().Manager().CreateWithCompose(ctx, container.CreateOptions{
		ProjectPath: projectDir,
		Template:    "basic",
		Name:        containerName,
	})
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}

	t.Cleanup(func() {
		CleanupContainer(t, runtime, c.ID)
	})

	// Refresh to see the container
	runner.PressKey('r')
	time.Sleep(2 * time.Second)

	// Open session view by pressing Enter
	runner.PressSpecialKey(tea.KeyEnter)
	if !runner.Model().IsSessionViewOpen() {
		t.Fatal("Expected session view to be open after pressing Enter")
	}

	// Should have no sessions initially
	if runner.Model().VisibleSessionCount() != 0 {
		t.Errorf("Expected 0 sessions initially, got %d", runner.Model().VisibleSessionCount())
	}

	// Press 't' to open session form
	runner.PressKey('t')
	if !runner.Model().IsSessionFormOpen() {
		t.Fatal("Expected session form to be open after pressing 't'")
	}

	// Type session name
	runner.TypeText("dev")

	// Submit form
	runner.PressSpecialKey(tea.KeyEnter)

	// Wait for session to be created
	time.Sleep(3 * time.Second)

	// Verify session was created by listing via manager
	sessions, err := runner.Model().Manager().ListSessions(context.Background(), c.ID)
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("Expected 1 session, got %d", len(sessions))
	}

	if sessions[0].Name != "dev" {
		t.Errorf("Expected session name 'dev', got %q", sessions[0].Name)
	}

	t.Logf("Successfully created tmux session 'dev' in container %s", containerName)
}

// testKillTmuxSession tests killing a tmux session inside a container.
func testKillTmuxSession(t *testing.T, runtime string) {
	SkipIfRuntimeMissing(t, runtime)
	SkipIfDevcontainerMissing(t)

	cfg := TestConfig(runtime)
	templates := TestTemplates()
	projectDir := TestProject(t, "basic")

	// Create logging manager for the TUI model
	logMgr := TestLogManager(t)

	// Create model and test runner
	model := tui.NewModelWithTemplates(cfg, templates, logMgr)
	runner := NewTUITestRunner(t, model)

	// Initialize
	runner.SendWindowSize(120, 40)
	runner.Init()

	// First create a container
	containerName := fmt.Sprintf("e2e-tmux-kill-%s-%d", runtime, time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	c, err := runner.Model().Manager().CreateWithCompose(ctx, container.CreateOptions{
		ProjectPath: projectDir,
		Template:    "basic",
		Name:        containerName,
	})
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}

	t.Cleanup(func() {
		CleanupContainer(t, runtime, c.ID)
	})

	// Create a session directly via manager
	if err := runner.Model().Manager().CreateSession(ctx, c.ID, "test-session"); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Verify session exists
	sessions, err := runner.Model().Manager().ListSessions(ctx, c.ID)
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("Expected 1 session, got %d", len(sessions))
	}

	// Kill session via manager
	if err := runner.Model().Manager().KillSession(ctx, c.ID, "test-session"); err != nil {
		t.Fatalf("Failed to kill session: %v", err)
	}

	// Verify session was killed
	sessions, err = runner.Model().Manager().ListSessions(ctx, c.ID)
	if err != nil {
		t.Fatalf("Failed to list sessions after kill: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("Expected 0 sessions after kill, got %d", len(sessions))
	}

	t.Logf("Successfully killed tmux session in container %s", containerName)
}

// testTmuxAttachCommand tests that the attach command is correctly generated.
func testTmuxAttachCommand(t *testing.T, runtime string) {
	SkipIfRuntimeMissing(t, runtime)
	SkipIfDevcontainerMissing(t)

	cfg := TestConfig(runtime)
	templates := TestTemplates()
	projectDir := TestProject(t, "basic")

	// Create logging manager for the TUI model
	logMgr := TestLogManager(t)

	// Create model and test runner
	model := tui.NewModelWithTemplates(cfg, templates, logMgr)
	runner := NewTUITestRunner(t, model)

	// Initialize
	runner.SendWindowSize(120, 40)
	runner.Init()

	// First create a container
	containerName := fmt.Sprintf("e2e-attach-%s-%d", runtime, time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	c, err := runner.Model().Manager().CreateWithCompose(ctx, container.CreateOptions{
		ProjectPath: projectDir,
		Template:    "basic",
		Name:        containerName,
	})
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}

	t.Cleanup(func() {
		CleanupContainer(t, runtime, c.ID)
	})

	// Create a session
	if err := runner.Model().Manager().CreateSession(ctx, c.ID, "main"); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Refresh to see the container
	runner.PressKey('r')
	time.Sleep(2 * time.Second)

	// Open session view
	runner.PressSpecialKey(tea.KeyEnter)

	// Refresh sessions (the session view doesn't auto-load sessions yet)
	// We need to manually update the container's sessions
	sessions, _ := runner.Model().Manager().ListSessions(ctx, c.ID)

	// Get the container and update its sessions
	containerFromList, ok := runner.Model().GetContainerByName(containerName)
	if !ok {
		t.Fatal("Container not found in list")
	}
	containerFromList.Sessions = sessions

	// Re-open session view to pick up the updated container
	runner.PressSpecialKey(tea.KeyEscape)
	runner.PressSpecialKey(tea.KeyEnter)

	// Get attach command
	attachCmd := runner.Model().AttachCommand()
	expectedCmd := fmt.Sprintf("%s exec -it %s tmux attach -t main", runtime, c.ID)

	if attachCmd != expectedCmd {
		t.Errorf("AttachCommand() = %q, want %q", attachCmd, expectedCmd)
	}

	t.Logf("Attach command correctly generated: %s", attachCmd)
}

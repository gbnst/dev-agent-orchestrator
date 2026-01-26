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
	projectDir := TestProject(t, "default")

	// Create model and test runner
	model := tui.NewModelWithTemplates(cfg, templates)
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
	projectDir := TestProject(t, "default")

	// Create model and test runner
	model := tui.NewModelWithTemplates(cfg, templates)
	runner := NewTUITestRunner(t, model)

	// Initialize
	runner.SendWindowSize(120, 40)
	runner.Init()

	// First create a container
	containerName := fmt.Sprintf("e2e-startstop-%s-%d", runtime, time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	c, err := runner.Model().Manager().Create(ctx, container.CreateOptions{
		ProjectPath: projectDir,
		Template:    "default",
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
	projectDir := TestProject(t, "default")

	// Create model and test runner
	model := tui.NewModelWithTemplates(cfg, templates)
	runner := NewTUITestRunner(t, model)

	// Initialize
	runner.SendWindowSize(120, 40)
	runner.Init()

	// First create a container
	containerName := fmt.Sprintf("e2e-destroy-%s-%d", runtime, time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	c, err := runner.Model().Manager().Create(ctx, container.CreateOptions{
		ProjectPath: projectDir,
		Template:    "default",
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

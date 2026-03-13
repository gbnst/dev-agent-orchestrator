package worktree

import (
	"context"
	"errors"
	"testing"

	"devagent/internal/container"
)

// mockContainerOps is a mock implementation of ContainerOps for testing.
type mockContainerOps struct {
	getByComposeProject   *container.Container // return value for GetByComposeProject
	stopWithComposeErr    error
	destroyWithComposeErr error
	stopCalled            bool
	stopContainerID       string
	destroyCalled         bool
	destroyContainerID    string
	getByComposeCalled    string // captured compose name argument
}

func (m *mockContainerOps) GetByComposeProject(composeName string) *container.Container {
	m.getByComposeCalled = composeName
	return m.getByComposeProject
}

func (m *mockContainerOps) StopWithCompose(ctx context.Context, containerID string) error {
	m.stopCalled = true
	m.stopContainerID = containerID
	return m.stopWithComposeErr
}

func (m *mockContainerOps) DestroyWithCompose(ctx context.Context, containerID string) error {
	m.destroyCalled = true
	m.destroyContainerID = containerID
	return m.destroyWithComposeErr
}

// mockWorktreeOps is a mock implementation of WorktreeOps for testing.
type mockWorktreeOps struct {
	destroyCalled bool
	destroyErr    error
}

func (m *mockWorktreeOps) Destroy(projectPath, name string) error {
	m.destroyCalled = true
	return m.destroyErr
}

func TestDestroyWorktreeWithContainer_NoContainer(t *testing.T) {
	ctx := context.Background()
	containerOps := &mockContainerOps{
		getByComposeProject: nil, // no container found
	}

	// Mock the Destroy function to avoid actual git operations
	// We'll test this by ensuring no stop/destroy is called on containers
	err := DestroyWorktreeWithContainer(ctx, containerOps, "/home/user/project", "feature-x", nil)

	// Should fail because Destroy will actually try to run git commands
	// In real testing, this would be mocked at a lower level
	if err == nil {
		t.Errorf("expected error from git operations, got nil")
	}

	// Verify GetByComposeProject was called with correct compose name
	expectedComposeName := container.SanitizeComposeName("project-feature-x")
	if containerOps.getByComposeCalled != expectedComposeName {
		t.Errorf("expected GetByComposeProject called with %q, got %q", expectedComposeName, containerOps.getByComposeCalled)
	}

	// Verify no container operations were attempted
	if containerOps.stopCalled {
		t.Errorf("expected stopWithCompose not to be called")
	}
	if containerOps.destroyCalled {
		t.Errorf("expected destroyWithCompose not to be called")
	}
}

func TestDestroyWorktreeWithContainer_WithRunningContainer(t *testing.T) {
	ctx := context.Background()

	// Create a mock running container for the worktree
	runningContainer := &container.Container{
		ID:             "test-container-123",
		ComposeProject: "project-feature-x",
		State:          container.StateRunning,
	}

	containerOps := &mockContainerOps{
		getByComposeProject: runningContainer,
	}

	// Since DestroyWorktreeWithContainer calls the real Destroy function,
	// it will fail on git operations. However, we can verify that the
	// container operations were called in the correct order.
	err := DestroyWorktreeWithContainer(ctx, containerOps, "/home/user/project", "feature-x", nil)

	// Expect failure from git operations
	if err == nil {
		t.Errorf("expected error from git operations, got nil")
	}

	// Verify GetByComposeProject was called with correct compose name
	expectedComposeName := container.SanitizeComposeName("project-feature-x")
	if containerOps.getByComposeCalled != expectedComposeName {
		t.Errorf("expected GetByComposeProject called with %q, got %q", expectedComposeName, containerOps.getByComposeCalled)
	}

	// Verify stop was called
	if !containerOps.stopCalled {
		t.Errorf("expected stopWithCompose to be called")
	}
	if containerOps.stopContainerID != "test-container-123" {
		t.Errorf("stopWithCompose called with wrong ID: %s", containerOps.stopContainerID)
	}

	// Verify destroy was called
	if !containerOps.destroyCalled {
		t.Errorf("expected destroyWithCompose to be called")
	}
	if containerOps.destroyContainerID != "test-container-123" {
		t.Errorf("destroyWithCompose called with wrong ID: %s", containerOps.destroyContainerID)
	}
}

func TestDestroyWorktreeWithContainer_WithStoppedContainer(t *testing.T) {
	ctx := context.Background()

	// Create a mock stopped container for the worktree
	stoppedContainer := &container.Container{
		ID:             "test-container-456",
		ComposeProject: "project-feature-y",
		State:          container.StateStopped,
	}

	containerOps := &mockContainerOps{
		getByComposeProject: stoppedContainer,
	}

	err := DestroyWorktreeWithContainer(ctx, containerOps, "/home/user/project", "feature-y", nil)

	// Expect failure from git operations
	if err == nil {
		t.Errorf("expected error from git operations, got nil")
	}

	// Verify GetByComposeProject was called with correct compose name
	expectedComposeName := container.SanitizeComposeName("project-feature-y")
	if containerOps.getByComposeCalled != expectedComposeName {
		t.Errorf("expected GetByComposeProject called with %q, got %q", expectedComposeName, containerOps.getByComposeCalled)
	}

	// Verify stop was NOT called (container already stopped)
	if containerOps.stopCalled {
		t.Errorf("expected stopWithCompose not to be called for stopped container")
	}

	// Verify destroy was called
	if !containerOps.destroyCalled {
		t.Errorf("expected destroyWithCompose to be called")
	}
	if containerOps.destroyContainerID != "test-container-456" {
		t.Errorf("destroyWithCompose called with wrong ID: %s", containerOps.destroyContainerID)
	}
}

func TestDestroyWorktreeWithContainer_StopError(t *testing.T) {
	ctx := context.Background()

	runningContainer := &container.Container{
		ID:             "test-container-789",
		ComposeProject: "project-feature-z",
		State:          container.StateRunning,
	}

	containerOps := &mockContainerOps{
		getByComposeProject: runningContainer,
		stopWithComposeErr:  errors.New("compose stop failed"),
	}

	err := DestroyWorktreeWithContainer(ctx, containerOps, "/home/user/project", "feature-z", nil)

	// Should fail with stop error
	if err == nil {
		t.Errorf("expected error from stopWithCompose, got nil")
	}
	if !errors.Is(err, containerOps.stopWithComposeErr) && !errors.Is(errors.Unwrap(err), containerOps.stopWithComposeErr) {
		t.Errorf("expected error containing 'failed to stop container', got: %v", err)
	}

	// Verify GetByComposeProject was called with correct compose name
	expectedComposeName := container.SanitizeComposeName("project-feature-z")
	if containerOps.getByComposeCalled != expectedComposeName {
		t.Errorf("expected GetByComposeProject called with %q, got %q", expectedComposeName, containerOps.getByComposeCalled)
	}

	// Verify destroy was NOT called (stopped early due to stop error)
	if containerOps.destroyCalled {
		t.Errorf("expected destroyWithCompose not to be called after stop error")
	}
}

func TestDestroyWorktreeWithContainer_DestroyError(t *testing.T) {
	ctx := context.Background()

	stoppedContainer := &container.Container{
		ID:             "test-container-999",
		ComposeProject: "project-feature-w",
		State:          container.StateStopped,
	}

	containerOps := &mockContainerOps{
		getByComposeProject:   stoppedContainer,
		destroyWithComposeErr: errors.New("compose down failed"),
	}

	err := DestroyWorktreeWithContainer(ctx, containerOps, "/home/user/project", "feature-w", nil)

	// Should fail with destroy error
	if err == nil {
		t.Errorf("expected error from destroyWithCompose, got nil")
	}
	if !errors.Is(err, containerOps.destroyWithComposeErr) && !errors.Is(errors.Unwrap(err), containerOps.destroyWithComposeErr) {
		t.Errorf("expected error containing 'failed to destroy container', got: %v", err)
	}

	// Verify GetByComposeProject was called with correct compose name
	expectedComposeName := container.SanitizeComposeName("project-feature-w")
	if containerOps.getByComposeCalled != expectedComposeName {
		t.Errorf("expected GetByComposeProject called with %q, got %q", expectedComposeName, containerOps.getByComposeCalled)
	}
}

func TestDestroyWorktreeWithContainer_FullSuccess(t *testing.T) {
	ctx := context.Background()

	// Create a mock running container for the worktree
	runningContainer := &container.Container{
		ID:             "test-container-full-success",
		ComposeProject: "project-feature-full",
		State:          container.StateRunning,
	}

	containerOps := &mockContainerOps{
		getByComposeProject: runningContainer,
	}

	wtOps := &mockWorktreeOps{}

	// Call with mock WorktreeOps
	err := DestroyWorktreeWithContainer(ctx, containerOps, "/home/user/project", "feature-full", wtOps)

	// Should succeed (no errors from container or worktree ops)
	if err != nil {
		t.Errorf("expected success, got error: %v", err)
	}

	// Verify GetByComposeProject was called with correct compose name
	expectedComposeName := container.SanitizeComposeName("project-feature-full")
	if containerOps.getByComposeCalled != expectedComposeName {
		t.Errorf("expected GetByComposeProject called with %q, got %q", expectedComposeName, containerOps.getByComposeCalled)
	}

	// Verify Destroy was called (WorktreeDir is no longer called since we use compose project name)
	if !wtOps.destroyCalled {
		t.Errorf("expected Destroy to be called")
	}

	// Verify container operations were called
	if !containerOps.stopCalled {
		t.Errorf("expected stopWithCompose to be called")
	}
	if !containerOps.destroyCalled {
		t.Errorf("expected destroyWithCompose to be called")
	}
}

func TestDestroyWorktreeWithContainer_WorktreeDestroyError(t *testing.T) {
	ctx := context.Background()

	// Create a mock running container for the worktree
	runningContainer := &container.Container{
		ID:             "test-container-wtree-error",
		ComposeProject: "project-feature-err",
		State:          container.StateRunning,
	}

	containerOps := &mockContainerOps{
		getByComposeProject: runningContainer,
	}

	wtOps := &mockWorktreeOps{
		destroyErr: errors.New("git worktree remove failed"),
	}

	// Call with mock WorktreeOps that returns an error
	err := DestroyWorktreeWithContainer(ctx, containerOps, "/home/user/project", "feature-err", wtOps)

	// Should fail with the worktree destroy error
	if err == nil {
		t.Errorf("expected error from worktree destroy, got nil")
	}
	if !errors.Is(err, wtOps.destroyErr) {
		t.Errorf("expected worktree destroy error, got: %v", err)
	}

	// Verify GetByComposeProject was called with correct compose name
	expectedComposeName := container.SanitizeComposeName("project-feature-err")
	if containerOps.getByComposeCalled != expectedComposeName {
		t.Errorf("expected GetByComposeProject called with %q, got %q", expectedComposeName, containerOps.getByComposeCalled)
	}

	// Verify container operations were called (they should have succeeded)
	if !containerOps.stopCalled {
		t.Errorf("expected stopWithCompose to be called")
	}
	if !containerOps.destroyCalled {
		t.Errorf("expected destroyWithCompose to be called")
	}

	// Verify Destroy was called
	if !wtOps.destroyCalled {
		t.Errorf("expected Destroy to be called")
	}
}

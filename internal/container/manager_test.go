package container

import (
	"context"
	"errors"
	"testing"
)

// mockRuntime implements a testable Runtime interface
type mockRuntime struct {
	containers      []Container
	listErr         error
	startCalled     string
	stopCalled      string
	removeCalled    string
	startErr        error
	stopErr         error
	removeErr       error
}

func (m *mockRuntime) ListContainers(ctx context.Context) ([]Container, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.containers, nil
}

func (m *mockRuntime) StartContainer(ctx context.Context, id string) error {
	m.startCalled = id
	return m.startErr
}

func (m *mockRuntime) StopContainer(ctx context.Context, id string) error {
	m.stopCalled = id
	return m.stopErr
}

func (m *mockRuntime) RemoveContainer(ctx context.Context, id string) error {
	m.removeCalled = id
	return m.removeErr
}

func (m *mockRuntime) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	return "", nil
}

func TestList_Empty(t *testing.T) {
	mock := &mockRuntime{containers: []Container{}}
	mgr := NewManagerWithRuntime(mock)

	containers := mgr.List()
	if len(containers) != 0 {
		t.Errorf("Expected empty list, got %d", len(containers))
	}
}

func TestList_AfterRefresh(t *testing.T) {
	mock := &mockRuntime{
		containers: []Container{
			{ID: "abc123", Name: "container-1", State: StateRunning},
			{ID: "def456", Name: "container-2", State: StateStopped},
		},
	}
	mgr := NewManagerWithRuntime(mock)

	// Before refresh, should be empty
	if len(mgr.List()) != 0 {
		t.Error("Should be empty before refresh")
	}

	// Refresh
	ctx := context.Background()
	if err := mgr.Refresh(ctx); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// After refresh, should have containers
	containers := mgr.List()
	if len(containers) != 2 {
		t.Errorf("Expected 2 containers, got %d", len(containers))
	}
}

func TestGet_Found(t *testing.T) {
	mock := &mockRuntime{
		containers: []Container{
			{ID: "abc123", Name: "container-1", State: StateRunning},
		},
	}
	mgr := NewManagerWithRuntime(mock)
	ctx := context.Background()
	_ = mgr.Refresh(ctx)

	c, found := mgr.Get("abc123")
	if !found {
		t.Error("Expected to find container")
	}
	if c.Name != "container-1" {
		t.Errorf("Name: got %q", c.Name)
	}
}

func TestGet_NotFound(t *testing.T) {
	mock := &mockRuntime{containers: []Container{}}
	mgr := NewManagerWithRuntime(mock)
	ctx := context.Background()
	_ = mgr.Refresh(ctx)

	_, found := mgr.Get("unknown")
	if found {
		t.Error("Should not find unknown container")
	}
}

func TestStart_CallsRuntime(t *testing.T) {
	mock := &mockRuntime{
		containers: []Container{
			{ID: "abc123", Name: "container-1", State: StateStopped},
		},
	}
	mgr := NewManagerWithRuntime(mock)
	ctx := context.Background()
	_ = mgr.Refresh(ctx)

	err := mgr.Start(ctx, "abc123")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if mock.startCalled != "abc123" {
		t.Errorf("Expected StartContainer called with abc123, got %q", mock.startCalled)
	}
}

func TestStart_NotFound(t *testing.T) {
	mock := &mockRuntime{containers: []Container{}}
	mgr := NewManagerWithRuntime(mock)
	ctx := context.Background()
	_ = mgr.Refresh(ctx)

	err := mgr.Start(ctx, "unknown")
	if err == nil {
		t.Error("Expected error for unknown container")
	}
}

func TestStop_CallsRuntime(t *testing.T) {
	mock := &mockRuntime{
		containers: []Container{
			{ID: "abc123", Name: "container-1", State: StateRunning},
		},
	}
	mgr := NewManagerWithRuntime(mock)
	ctx := context.Background()
	_ = mgr.Refresh(ctx)

	err := mgr.Stop(ctx, "abc123")
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if mock.stopCalled != "abc123" {
		t.Errorf("Expected StopContainer called with abc123, got %q", mock.stopCalled)
	}
}

func TestDestroy_StopsFirst(t *testing.T) {
	mock := &mockRuntime{
		containers: []Container{
			{ID: "abc123", Name: "container-1", State: StateRunning},
		},
	}
	mgr := NewManagerWithRuntime(mock)
	ctx := context.Background()
	_ = mgr.Refresh(ctx)

	err := mgr.Destroy(ctx, "abc123")
	if err != nil {
		t.Fatalf("Destroy failed: %v", err)
	}

	// Should have called stop first (for running container)
	if mock.stopCalled != "abc123" {
		t.Error("Expected stop to be called first for running container")
	}
	if mock.removeCalled != "abc123" {
		t.Error("Expected remove to be called")
	}
}

func TestDestroy_SkipsStopForStopped(t *testing.T) {
	mock := &mockRuntime{
		containers: []Container{
			{ID: "abc123", Name: "container-1", State: StateStopped},
		},
	}
	mgr := NewManagerWithRuntime(mock)
	ctx := context.Background()
	_ = mgr.Refresh(ctx)

	err := mgr.Destroy(ctx, "abc123")
	if err != nil {
		t.Fatalf("Destroy failed: %v", err)
	}

	// Should NOT have called stop for already stopped container
	if mock.stopCalled != "" {
		t.Error("Should not call stop for already stopped container")
	}
	if mock.removeCalled != "abc123" {
		t.Error("Expected remove to be called")
	}
}

func TestRefresh_Error(t *testing.T) {
	mock := &mockRuntime{
		listErr: errors.New("docker not running"),
	}
	mgr := NewManagerWithRuntime(mock)
	ctx := context.Background()

	err := mgr.Refresh(ctx)
	if err == nil {
		t.Error("Expected error from Refresh")
	}
}

func TestStart_RuntimeError(t *testing.T) {
	mock := &mockRuntime{
		containers: []Container{
			{ID: "abc123", Name: "container-1", State: StateStopped},
		},
		startErr: errors.New("failed to start"),
	}
	mgr := NewManagerWithRuntime(mock)
	ctx := context.Background()
	_ = mgr.Refresh(ctx)

	err := mgr.Start(ctx, "abc123")
	if err == nil {
		t.Error("Expected error from Start")
	}
}

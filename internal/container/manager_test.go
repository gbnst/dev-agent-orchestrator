package container

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
)

// mockRuntime implements a testable Runtime interface
type mockRuntime struct {
	containers   []Container
	listErr      error
	startCalled  string
	stopCalled   string
	removeCalled string
	startErr     error
	stopErr      error
	removeErr    error

	// Network operation tracking
	createNetworkCalled string
	createNetworkErr    error
	createNetworkID     string
	removeNetworkCalled string
	removeNetworkErr    error
	runContainerCalled  *RunContainerOptions
	runContainerErr     error
	runContainerID      string
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

func (m *mockRuntime) ExecAs(ctx context.Context, id string, user string, cmd []string) (string, error) {
	return "", nil
}

func (m *mockRuntime) CreateNetwork(ctx context.Context, name string) (string, error) {
	m.createNetworkCalled = name
	if m.createNetworkErr != nil {
		return "", m.createNetworkErr
	}
	if m.createNetworkID == "" {
		return "mock-network-id", nil
	}
	return m.createNetworkID, nil
}

func (m *mockRuntime) RemoveNetwork(ctx context.Context, name string) error {
	m.removeNetworkCalled = name
	return m.removeNetworkErr
}

func (m *mockRuntime) RunContainer(ctx context.Context, opts RunContainerOptions) (string, error) {
	m.runContainerCalled = &opts
	if m.runContainerErr != nil {
		return "", m.runContainerErr
	}
	if m.runContainerID == "" {
		return "mock-container-id", nil
	}
	return m.runContainerID, nil
}

func (m *mockRuntime) InspectContainer(ctx context.Context, id string) (ContainerState, error) {
	return StateRunning, nil
}

func (m *mockRuntime) GetIsolationInfo(ctx context.Context, id string) (*IsolationInfo, error) {
	return &IsolationInfo{}, nil
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

func TestManager_GetSidecarsForProject(t *testing.T) {
	mock := &mockRuntime{}
	mgr := NewManagerWithRuntime(mock)

	// Calculate hashes for test projects
	project1Hash := calculateHash("/project/1")
	project2Hash := calculateHash("/project/2")

	// Add test sidecars
	mgr.sidecars["sidecar-1"] = &Sidecar{
		ID:        "sidecar-1",
		Type:      "proxy",
		ParentRef: project1Hash,
		State:     StateRunning,
	}
	mgr.sidecars["sidecar-2"] = &Sidecar{
		ID:        "sidecar-2",
		Type:      "proxy",
		ParentRef: project2Hash,
		State:     StateRunning,
	}
	mgr.sidecars["sidecar-3"] = &Sidecar{
		ID:        "sidecar-3",
		Type:      "other",
		ParentRef: project1Hash,
		State:     StateStopped,
	}

	// Get sidecars for project 1
	sidecars := mgr.GetSidecarsForProject("/project/1")
	if len(sidecars) != 2 {
		t.Errorf("GetSidecarsForProject() returned %d sidecars, want 2", len(sidecars))
	}

	// Get sidecars for project 2
	sidecars = mgr.GetSidecarsForProject("/project/2")
	if len(sidecars) != 1 {
		t.Errorf("GetSidecarsForProject() returned %d sidecars, want 1", len(sidecars))
	}

	// Get sidecars for non-existent project
	sidecars = mgr.GetSidecarsForProject("/project/nonexistent")
	if len(sidecars) != 0 {
		t.Errorf("GetSidecarsForProject() returned %d sidecars for non-existent project, want 0", len(sidecars))
	}
}

func TestManager_RefreshSidecars(t *testing.T) {
	mock := &mockRuntime{}
	mgr := NewManagerWithRuntime(mock)

	// Simulate containers returned from ListContainers
	allContainers := []Container{
		{
			ID:    "devcontainer-1",
			Name:  "devcontainer-1",
			State: StateRunning,
			Labels: map[string]string{
				LabelManagedBy: "true",
			},
		},
		{
			ID:    "proxy-sidecar-1",
			Name:  "devagent-abc123-proxy",
			State: StateRunning,
			Labels: map[string]string{
				LabelManagedBy:   "true",
				LabelSidecarOf:   "abc123", // Project hash, not container ID
				LabelSidecarType: "proxy",
			},
		},
	}

	mgr.refreshSidecars(context.Background(), allContainers)

	// Verify sidecar was discovered
	if len(mgr.sidecars) != 1 {
		t.Errorf("refreshSidecars() found %d sidecars, want 1", len(mgr.sidecars))
	}

	sidecar, ok := mgr.sidecars["proxy-sidecar-1"]
	if !ok {
		t.Fatal("sidecar not found in map")
	}

	if sidecar.ParentRef != "abc123" {
		t.Errorf("sidecar.ParentRef = %q, want %q", sidecar.ParentRef, "abc123")
	}
	if sidecar.Type != "proxy" {
		t.Errorf("sidecar.Type = %q, want %q", sidecar.Type, "proxy")
	}
	if sidecar.NetworkName != "devagent-abc123-net" {
		t.Errorf("sidecar.NetworkName = %q, want %q", sidecar.NetworkName, "devagent-abc123-net")
	}
}

// Helper function to calculate hash like manager does
func calculateHash(projectPath string) string {
	hash := sha256.Sum256([]byte(projectPath))
	return hex.EncodeToString(hash[:])[:12]
}

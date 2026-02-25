package container

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"devagent/internal/config"
)

// mockRuntime implements a testable Runtime interface
type mockRuntime struct {
	containers []Container
	listErr    error

	// Compose operations
	composeUpCalled     string // projectDir
	composeUpProject    string // projectName
	composeUpErr        error
	composeStartCalled  string
	composeStartProject string
	composeStartErr     error
	composeStopCalled   string
	composeStopProject  string
	composeStopErr      error
	composeDownCalled   string
	composeDownProject  string
	composeDownErr      error
}

func (m *mockRuntime) ListContainers(ctx context.Context) ([]Container, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.containers, nil
}

func (m *mockRuntime) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	return "", nil
}

func (m *mockRuntime) ExecAs(ctx context.Context, id string, user string, cmd []string) (string, error) {
	return "", nil
}

func (m *mockRuntime) InspectContainer(ctx context.Context, id string) (ContainerState, error) {
	return StateRunning, nil
}

func (m *mockRuntime) GetIsolationInfo(ctx context.Context, id string) (*IsolationInfo, error) {
	return &IsolationInfo{}, nil
}

func (m *mockRuntime) ComposeUp(ctx context.Context, projectDir string, projectName string) error {
	m.composeUpCalled = projectDir
	m.composeUpProject = projectName
	return m.composeUpErr
}

func (m *mockRuntime) ComposeStart(ctx context.Context, projectDir string, projectName string) error {
	m.composeStartCalled = projectDir
	m.composeStartProject = projectName
	return m.composeStartErr
}

func (m *mockRuntime) ComposeStop(ctx context.Context, projectDir string, projectName string) error {
	m.composeStopCalled = projectDir
	m.composeStopProject = projectName
	return m.composeStopErr
}

func (m *mockRuntime) ComposeDown(ctx context.Context, projectDir string, projectName string) error {
	m.composeDownCalled = projectDir
	m.composeDownProject = projectName
	return m.composeDownErr
}

func TestList_Empty(t *testing.T) {
	mock := &mockRuntime{containers: []Container{}}
	mgr := NewManager(ManagerOptions{Runtime: mock})

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
	mgr := NewManager(ManagerOptions{Runtime: mock})

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

func TestList_ReturnsDeterministicOrder(t *testing.T) {
	mock := &mockRuntime{
		containers: []Container{
			{ID: "ccc", Name: "charlie", State: StateRunning},
			{ID: "aaa", Name: "alpha", State: StateRunning},
			{ID: "bbb", Name: "bravo", State: StateStopped},
		},
	}
	mgr := NewManager(ManagerOptions{Runtime: mock})
	ctx := context.Background()
	if err := mgr.Refresh(ctx); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Call List() multiple times — order must be stable and sorted by Name
	for i := 0; i < 20; i++ {
		containers := mgr.List()
		if len(containers) != 3 {
			t.Fatalf("iteration %d: expected 3 containers, got %d", i, len(containers))
		}
		if containers[0].Name != "alpha" || containers[1].Name != "bravo" || containers[2].Name != "charlie" {
			t.Fatalf("iteration %d: expected [alpha, bravo, charlie], got [%s, %s, %s]",
				i, containers[0].Name, containers[1].Name, containers[2].Name)
		}
	}
}

func TestGet_Found(t *testing.T) {
	mock := &mockRuntime{
		containers: []Container{
			{ID: "abc123", Name: "container-1", State: StateRunning},
		},
	}
	mgr := NewManager(ManagerOptions{Runtime: mock})
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
	mgr := NewManager(ManagerOptions{Runtime: mock})
	ctx := context.Background()
	_ = mgr.Refresh(ctx)

	_, found := mgr.Get("unknown")
	if found {
		t.Error("Should not find unknown container")
	}
}

func TestRefresh_Error(t *testing.T) {
	mock := &mockRuntime{
		listErr: errors.New("docker not running"),
	}
	mgr := NewManager(ManagerOptions{Runtime: mock})
	ctx := context.Background()

	err := mgr.Refresh(ctx)
	if err == nil {
		t.Error("Expected error from Refresh")
	}
}

func TestManager_GetSidecarsForProject(t *testing.T) {
	mock := &mockRuntime{}
	mgr := NewManager(ManagerOptions{Runtime: mock})

	// Simulate containers with compose project labels
	mgr.containers["app-1"] = &Container{
		ID:          "app-1",
		Name:        "project1-app-1",
		ProjectPath: "/project/1",
		State:       StateRunning,
		Labels: map[string]string{
			LabelComposeProject: "project1",
		},
	}
	mgr.containers["app-2"] = &Container{
		ID:          "app-2",
		Name:        "project2-app-1",
		ProjectPath: "/project/2",
		State:       StateRunning,
		Labels: map[string]string{
			LabelComposeProject: "project2",
		},
	}

	// Add test sidecars keyed by compose project name
	mgr.sidecars["sidecar-1"] = &Sidecar{
		ID:        "sidecar-1",
		Type:      "proxy",
		ParentRef: "project1",
		State:     StateRunning,
	}
	mgr.sidecars["sidecar-2"] = &Sidecar{
		ID:        "sidecar-2",
		Type:      "proxy",
		ParentRef: "project2",
		State:     StateRunning,
	}
	mgr.sidecars["sidecar-3"] = &Sidecar{
		ID:        "sidecar-3",
		Type:      "other",
		ParentRef: "project1",
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
	mgr := NewManager(ManagerOptions{Runtime: mock})

	// Simulate containers returned from ListContainers
	allContainers := []Container{
		{
			ID:    "devcontainer-1",
			Name:  "myproject-app-1",
			State: StateRunning,
			Labels: map[string]string{
				LabelManagedBy:      "true",
				LabelComposeProject: "myproject",
			},
		},
		{
			ID:    "proxy-sidecar-1",
			Name:  "myproject-proxy-1",
			State: StateRunning,
			Labels: map[string]string{
				LabelManagedBy:      "true",
				LabelSidecarType:    "proxy",
				LabelComposeProject: "myproject",
			},
		},
	}

	mgr.refreshSidecars(allContainers)

	// Verify sidecar was discovered
	if len(mgr.sidecars) != 1 {
		t.Errorf("refreshSidecars() found %d sidecars, want 1", len(mgr.sidecars))
	}

	sidecar, ok := mgr.sidecars["proxy-sidecar-1"]
	if !ok {
		t.Fatal("sidecar not found in map")
	}

	if sidecar.ParentRef != "myproject" {
		t.Errorf("sidecar.ParentRef = %q, want %q", sidecar.ParentRef, "myproject")
	}
	if sidecar.Type != "proxy" {
		t.Errorf("sidecar.Type = %q, want %q", sidecar.Type, "proxy")
	}
}

func TestComposeGenerator_GeneratesAndWritesFiles(t *testing.T) {
	projectDir := t.TempDir()

	cfg := &config.Config{
		Runtime: "docker",
	}
	templates := []config.Template{
		{
			Name: "default",
		},
	}

	// Create template files in the template directory with .devcontainer structure
	templateDir := t.TempDir()
	devcontainerDir := filepath.Join(templateDir, ".devcontainer")
	templates[0].Path = templateDir

	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatalf("Failed to create .devcontainer dir: %v", err)
	}

	dockerfileContent := "FROM ubuntu:22.04\n"
	if err := os.WriteFile(filepath.Join(devcontainerDir, "Dockerfile"), []byte(dockerfileContent), 0644); err != nil {
		t.Fatalf("Failed to write Dockerfile: %v", err)
	}

	// Create docker-compose.yml.tmpl
	composeContent := `services:
  app:
    build:
      context: .
      dockerfile: Dockerfile
    depends_on:
      proxy:
        condition: service_started
    networks:
      - isolated
    volumes:
      - {{.ProjectPath}}:{{.WorkspaceFolder}}:cached
    labels:
      devagent.managed: "true"
      devagent.project_path: "{{.ProjectPath}}"
      devagent.template: "{{.TemplateName}}"
    command: ["sleep", "infinity"]

  proxy:
    image: mitmproxy/mitmproxy:latest
    networks:
      - isolated
    volumes:
      - proxy-certs:/home/mitmproxy/.mitmproxy
    command: ["mitmdump", "--listen-host", "0.0.0.0", "--listen-port", "8080", "-s", "/opt/devagent-proxy/filter.py"]
    labels:
      devagent.managed: "true"
      devagent.sidecar_type: "proxy"

networks:
  isolated:

volumes:
  proxy-certs:
`
	if err := os.WriteFile(filepath.Join(devcontainerDir, "docker-compose.yml.tmpl"), []byte(composeContent), 0644); err != nil {
		t.Fatalf("Failed to write docker-compose.yml.tmpl: %v", err)
	}

	mock := &mockRuntime{
		containers: []Container{
			{
				ID:          "abc123def456",
				Name:        "test-compose-container",
				ProjectPath: projectDir,
				State:       StateRunning,
			},
		},
	}

	// Create a manager with all dependencies for testing compose generation.
	// Pass nil for devCLI since we're testing file generation, not container creation.
	mgr := NewManager(ManagerOptions{
		Config:    cfg,
		Templates: templates,
		Runtime:   mock,
		DevCLI:    nil,
	})

	// Manually set the containers map to simulate devCLI.Up() success
	// This avoids needing a mock CLI - we're testing file generation
	mgr.containers["abc123def456"] = &Container{
		ID:          "abc123def456",
		Name:        "test-compose-container",
		ProjectPath: projectDir,
		State:       StateRunning,
	}

	// Test the compose file generation directly via composeGenerator
	composeOpts := ComposeOptions{
		ProjectPath: projectDir,
		Template:    "default",
		Name:        "test-compose-container",
	}

	result, err := mgr.composeGenerator.Generate(composeOpts)
	if err != nil {
		t.Fatalf("ComposeGenerator.Generate failed: %v", err)
	}

	// Verify TemplateData was populated
	if result.TemplateData.ProjectPath != projectDir {
		t.Error("TemplateData.ProjectPath not set correctly")
	}
	expectedProjectName := filepath.Base(projectDir)
	if result.TemplateData.ProjectName != expectedProjectName {
		t.Errorf("TemplateData.ProjectName = %q, want %q", result.TemplateData.ProjectName, expectedProjectName)
	}
}

func TestStartWithCompose_CallsRuntimeCompose(t *testing.T) {
	projectDir := t.TempDir()

	mock := &mockRuntime{}
	mgr := NewManager(ManagerOptions{Runtime: mock})
	mgr.containers["test-id"] = &Container{
		ID:          "test-id",
		Name:        "test-container",
		ProjectPath: projectDir,
		State:       StateStopped,
	}

	err := mgr.StartWithCompose(context.Background(), "test-id")
	if err != nil {
		t.Fatalf("StartWithCompose failed: %v", err)
	}

	if mock.composeStartCalled != projectDir {
		t.Errorf("Expected ComposeStart with projectDir %q, got %q", projectDir, mock.composeStartCalled)
	}

	// Verify state was updated
	if mgr.containers["test-id"].State != StateRunning {
		t.Error("Container state not updated to Running")
	}
}

func TestStopWithCompose_CallsRuntimeCompose(t *testing.T) {
	projectDir := t.TempDir()

	mock := &mockRuntime{}
	mgr := NewManager(ManagerOptions{Runtime: mock})
	mgr.containers["test-id"] = &Container{
		ID:          "test-id",
		Name:        "test-container",
		ProjectPath: projectDir,
		State:       StateRunning,
	}

	err := mgr.StopWithCompose(context.Background(), "test-id")
	if err != nil {
		t.Fatalf("StopWithCompose failed: %v", err)
	}

	if mock.composeStopCalled != projectDir {
		t.Errorf("Expected ComposeStop with projectDir %q, got %q", projectDir, mock.composeStopCalled)
	}

	// Verify state was updated
	if mgr.containers["test-id"].State != StateStopped {
		t.Error("Container state not updated to Stopped")
	}
}

func TestDestroyWithCompose_CallsRuntimeCompose(t *testing.T) {
	projectDir := t.TempDir()

	mock := &mockRuntime{}
	mgr := NewManager(ManagerOptions{Runtime: mock})
	mgr.containers["test-id"] = &Container{
		ID:          "test-id",
		Name:        "test-container",
		ProjectPath: projectDir,
		State:       StateRunning,
	}

	err := mgr.DestroyWithCompose(context.Background(), "test-id")
	if err != nil {
		t.Fatalf("DestroyWithCompose failed: %v", err)
	}

	if mock.composeDownCalled != projectDir {
		t.Errorf("Expected ComposeDown with projectDir %q, got %q", projectDir, mock.composeDownCalled)
	}

	// Verify container removed from map
	if _, exists := mgr.containers["test-id"]; exists {
		t.Error("Container not removed from map after destroy")
	}
}

func TestStartWithCompose_NotFound(t *testing.T) {
	mock := &mockRuntime{}
	mgr := NewManager(ManagerOptions{Runtime: mock})

	err := mgr.StartWithCompose(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent container")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

func TestManager_ConcurrentRefreshAndGet(t *testing.T) {
	mock := &mockRuntime{
		containers: []Container{
			{ID: "abc123", Name: "container-1", State: StateRunning},
		},
	}
	mgr := NewManager(ManagerOptions{Runtime: mock})
	ctx := context.Background()
	_ = mgr.Refresh(ctx)

	done := make(chan struct{})

	// Goroutine 1: refresh in a loop
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			_ = mgr.Refresh(ctx)
		}
	}()

	// Goroutine 2 (main): Get in a loop
	for i := 0; i < 100; i++ {
		_, _ = mgr.Get("abc123")
	}

	<-done
}

func TestManager_ConcurrentRefreshAndList(t *testing.T) {
	mock := &mockRuntime{
		containers: []Container{
			{ID: "abc123", Name: "container-1", State: StateRunning},
			{ID: "def456", Name: "container-2", State: StateStopped},
		},
	}
	mgr := NewManager(ManagerOptions{Runtime: mock})
	ctx := context.Background()
	_ = mgr.Refresh(ctx)

	done := make(chan struct{})

	// Goroutine 1: refresh in a loop
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			_ = mgr.Refresh(ctx)
		}
	}()

	// Goroutine 2 (main): List in a loop
	for i := 0; i < 100; i++ {
		_ = mgr.List()
	}

	<-done
}

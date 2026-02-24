package web_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/container"
	"devagent/internal/discovery"
	"devagent/internal/logging"
	"devagent/internal/tui"
	"devagent/internal/web"
)

// apiMockRuntime is a mock runtime for API handler tests.
type apiMockRuntime struct {
	containers []container.Container
	execOutput string
}

func (m *apiMockRuntime) ListContainers(_ context.Context) ([]container.Container, error) {
	return m.containers, nil
}

func (m *apiMockRuntime) StartContainer(_ context.Context, _ string) error { return nil }

func (m *apiMockRuntime) StopContainer(_ context.Context, _ string) error { return nil }

func (m *apiMockRuntime) RemoveContainer(_ context.Context, _ string) error { return nil }

func (m *apiMockRuntime) Exec(_ context.Context, _ string, _ []string) (string, error) {
	return m.execOutput, nil
}

func (m *apiMockRuntime) ExecAs(_ context.Context, _ string, _ string, _ []string) (string, error) {
	return m.execOutput, nil
}

func (m *apiMockRuntime) InspectContainer(_ context.Context, _ string) (container.ContainerState, error) {
	return container.StateRunning, nil
}

func (m *apiMockRuntime) GetIsolationInfo(_ context.Context, _ string) (*container.IsolationInfo, error) {
	return &container.IsolationInfo{}, nil
}

func (m *apiMockRuntime) ComposeUp(_ context.Context, _ string, _ string) error    { return nil }
func (m *apiMockRuntime) ComposeStart(_ context.Context, _ string, _ string) error { return nil }
func (m *apiMockRuntime) ComposeStop(_ context.Context, _ string, _ string) error  { return nil }
func (m *apiMockRuntime) ComposeDown(_ context.Context, _ string, _ string) error  { return nil }

// mutationMockRuntime is a mock runtime for session mutation tests.
// It maps tmux subcommands to canned outputs, allowing different responses for
// list-sessions, new-session, and kill-session calls.
type mutationMockRuntime struct {
	containers []container.Container
	// outputsByCmd maps tmux subcommand (e.g. "list-sessions", "new-session") to output.
	outputsByCmd map[string]string
}

func (m *mutationMockRuntime) ListContainers(_ context.Context) ([]container.Container, error) {
	return m.containers, nil
}

func (m *mutationMockRuntime) StartContainer(_ context.Context, _ string) error  { return nil }
func (m *mutationMockRuntime) StopContainer(_ context.Context, _ string) error   { return nil }
func (m *mutationMockRuntime) RemoveContainer(_ context.Context, _ string) error { return nil }

func (m *mutationMockRuntime) Exec(_ context.Context, _ string, _ []string) (string, error) {
	return "", nil
}

func (m *mutationMockRuntime) ExecAs(_ context.Context, _ string, _ string, cmd []string) (string, error) {
	// cmd is the full command, e.g. ["tmux", "-u", "new-session", "-d", "-s", "dev"]
	// Find the tmux subcommand (the first non-flag arg after "tmux")
	for _, arg := range cmd {
		if out, ok := m.outputsByCmd[arg]; ok {
			return out, nil
		}
	}
	return "", nil
}

func (m *mutationMockRuntime) InspectContainer(_ context.Context, _ string) (container.ContainerState, error) {
	return container.StateRunning, nil
}

func (m *mutationMockRuntime) GetIsolationInfo(_ context.Context, _ string) (*container.IsolationInfo, error) {
	return &container.IsolationInfo{}, nil
}

func (m *mutationMockRuntime) ComposeUp(_ context.Context, _ string, _ string) error    { return nil }
func (m *mutationMockRuntime) ComposeStart(_ context.Context, _ string, _ string) error { return nil }
func (m *mutationMockRuntime) ComposeStop(_ context.Context, _ string, _ string) error  { return nil }
func (m *mutationMockRuntime) ComposeDown(_ context.Context, _ string, _ string) error  { return nil }

// mockWorktreeOps is a mock implementation of worktreeOps for testing.
type mockWorktreeOps struct {
	validateErr error
	createPath  string
	createErr   error
	destroyErr  error
	wtDir       string
}

func (m *mockWorktreeOps) ValidateName(name string) error {
	return m.validateErr
}

func (m *mockWorktreeOps) Create(projectPath, name string) (string, error) {
	return m.createPath, m.createErr
}

func (m *mockWorktreeOps) Destroy(projectPath, name string) error {
	return m.destroyErr
}

func (m *mockWorktreeOps) WorktreeDir(projectPath, name string) string {
	return m.wtDir
}

// mockCommandExecutor returns a successful devcontainer up command result in the expected JSON format.
func mockCommandExecutor(ctx context.Context, name string, args ...string) (string, error) {
	// devcontainer up returns JSON with containerId field
	return `{"containerId":"mock-container-abc123"}`, nil
}

// startWorktreeTestServer creates a test server with a configurable mock worktreeOps.
func startWorktreeTestServer(t *testing.T, containers []container.Container, wt *mockWorktreeOps, notifyTUI func(tea.Msg)) string {
	t.Helper()
	runtime := &mutationMockRuntime{containers: containers}

	// Create DevcontainerCLI with mock executor to avoid actual container creation
	devCLI := container.NewDevcontainerCLIWithExecutor(mockCommandExecutor)

	mgr := container.NewManagerWithDeps(runtime, nil, devCLI)
	if err := mgr.Refresh(context.Background()); err != nil {
		t.Fatalf("manager.Refresh() error = %v", err)
	}
	lm := logging.NewTestLogManager(10)
	t.Cleanup(func() { _ = lm.Close() })

	s := web.New(web.Config{Bind: "127.0.0.1", Port: 0}, mgr, notifyTUI, lm, nil)
	// Override worktreeOps with mock
	s.SetWorktreeOpsForTest(wt)

	ln, err := s.Listen()
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- s.Serve(ln) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
		<-done
	})
	return "http://" + s.Addr()
}

// startMutationTestServer creates a test server using mutationMockRuntime and an optional notifyTUI callback.
func startMutationTestServer(t *testing.T, containers []container.Container, outputsByCmd map[string]string, notifyTUI func(tea.Msg)) string {
	t.Helper()

	runtime := &mutationMockRuntime{
		containers:   containers,
		outputsByCmd: outputsByCmd,
	}

	mgr := container.NewManagerWithRuntime(runtime)
	if err := mgr.Refresh(context.Background()); err != nil {
		t.Fatalf("manager.Refresh() error = %v", err)
	}

	lm := logging.NewTestLogManager(10)
	t.Cleanup(func() { _ = lm.Close() })

	s := web.New(web.Config{Bind: "127.0.0.1", Port: 0}, mgr, notifyTUI, lm, nil)

	ln, err := s.Listen()
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- s.Serve(ln)
	}()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
		<-done
	})

	return "http://" + s.Addr()
}

// startAPITestServer creates and starts a web.Server with a manager populated from the given containers.
// Returns the base URL and a shutdown function.
func startAPITestServer(t *testing.T, containers []container.Container, sessionOutput string) string {
	t.Helper()

	runtime := &apiMockRuntime{
		containers: containers,
		execOutput: sessionOutput,
	}

	mgr := container.NewManagerWithRuntime(runtime)
	if err := mgr.Refresh(context.Background()); err != nil {
		t.Fatalf("manager.Refresh() error = %v", err)
	}

	lm := logging.NewTestLogManager(10)
	t.Cleanup(func() { _ = lm.Close() })

	s := web.New(web.Config{Bind: "127.0.0.1", Port: 0}, mgr, nil, lm, nil)

	ln, err := s.Listen()
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- s.Serve(ln)
	}()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
		<-done
	})

	return "http://" + s.Addr()
}

// checkStringField asserts that a map field has the expected string value.
func checkStringField(t *testing.T, m map[string]any, key, want string) {
	t.Helper()
	val, ok := m[key]
	if !ok {
		t.Errorf("field %q missing from response", key)
		return
	}
	got, ok := val.(string)
	if !ok {
		t.Errorf("field %q type = %T, want string", key, val)
		return
	}
	if got != want {
		t.Errorf("field %q = %q, want %q", key, got, want)
	}
}

// startProjectsTestServer creates a test server with a scanner function for project discovery.
func startProjectsTestServer(t *testing.T, containers []container.Container, sessionOutput string, projects []discovery.DiscoveredProject) string {
	t.Helper()

	runtime := &apiMockRuntime{
		containers: containers,
		execOutput: sessionOutput,
	}

	mgr := container.NewManagerWithRuntime(runtime)
	if err := mgr.Refresh(context.Background()); err != nil {
		t.Fatalf("manager.Refresh() error = %v", err)
	}

	lm := logging.NewTestLogManager(10)
	t.Cleanup(func() { _ = lm.Close() })

	scanner := func(_ context.Context) []discovery.DiscoveredProject {
		return projects
	}

	s := web.New(web.Config{Bind: "127.0.0.1", Port: 0}, mgr, nil, lm, scanner)

	ln, err := s.Listen()
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- s.Serve(ln)
	}()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
		<-done
	})

	return "http://" + s.Addr()
}

// TestHandleListContainers_GH17AC11 verifies GET /api/containers returns 200 with JSON array.
func TestHandleListContainers_GH17AC11(t *testing.T) {
	createdAt := time.Date(2025, 1, 27, 10, 0, 0, 0, time.UTC)

	containers := []container.Container{
		{
			ID:          "abc123",
			Name:        "myproject-app-1",
			State:       container.StateRunning,
			Template:    "go",
			ProjectPath: "/home/user/myproject",
			RemoteUser:  "vscode",
			CreatedAt:   createdAt,
			Labels:      map[string]string{},
		},
	}

	t.Run("returns 200 with container fields", func(t *testing.T) {
		base := startAPITestServer(t, containers, "")

		resp, err := http.Get(base + "/api/containers")
		if err != nil {
			t.Fatalf("GET /api/containers error = %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var result []map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode error = %v", err)
		}

		if len(result) != 1 {
			t.Fatalf("len(result) = %d, want 1", len(result))
		}

		c := result[0]
		checkStringField(t, c, "id", "abc123")
		checkStringField(t, c, "name", "myproject-app-1")
		checkStringField(t, c, "state", "running")
		checkStringField(t, c, "template", "go")
		checkStringField(t, c, "project_path", "/home/user/myproject")
		checkStringField(t, c, "remote_user", "vscode")

		if _, ok := c["created_at"]; !ok {
			t.Error("response missing created_at field")
		}
	})

	t.Run("empty list returns array not null", func(t *testing.T) {
		base := startAPITestServer(t, []container.Container{}, "")

		resp, err := http.Get(base + "/api/containers")
		if err != nil {
			t.Fatalf("GET /api/containers error = %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("reading body error = %v", err)
		}

		// json.Encoder appends a newline; ensure it's an array (not null)
		bodyStr := string(body)
		if bodyStr != "[]\n" && bodyStr != "[]" {
			t.Errorf("empty list body = %q, want %q", bodyStr, "[]\n")
		}
	})
}

// TestHandleGetContainer_GH17AC12 verifies GET /api/containers/{id} returns single container with sessions.
func TestHandleGetContainer_GH17AC12(t *testing.T) {
	createdAt := time.Date(2025, 1, 27, 10, 0, 0, 0, time.UTC)

	containers := []container.Container{
		{
			ID:          "abc123",
			Name:        "myproject-app-1",
			State:       container.StateRunning,
			Template:    "go",
			ProjectPath: "/home/user/myproject",
			RemoteUser:  "vscode",
			CreatedAt:   createdAt,
			Labels:      map[string]string{},
		},
	}

	// Provide canned tmux session output for the running container
	sessionOutput := "main: 2 windows (created Mon Jan 27 10:00:00 2025)"

	base := startAPITestServer(t, containers, sessionOutput)

	resp, err := http.Get(base + "/api/containers/abc123")
	if err != nil {
		t.Fatalf("GET /api/containers/abc123 error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode error = %v", err)
	}

	checkStringField(t, result, "id", "abc123")
	checkStringField(t, result, "name", "myproject-app-1")

	sessions, ok := result["sessions"].([]any)
	if !ok {
		t.Fatalf("sessions field is not an array, got %T", result["sessions"])
	}
	if len(sessions) != 1 {
		t.Fatalf("len(sessions) = %d, want 1", len(sessions))
	}

	sess, ok := sessions[0].(map[string]any)
	if !ok {
		t.Fatalf("session is not an object, got %T", sessions[0])
	}
	checkStringField(t, sess, "name", "main")
}

// TestHandleListSessions_GH17AC13 verifies GET /api/containers/{id}/sessions returns sessions array.
func TestHandleListSessions_GH17AC13(t *testing.T) {
	containers := []container.Container{
		{
			ID:     "abc123",
			Name:   "myproject-app-1",
			State:  container.StateRunning,
			Labels: map[string]string{},
		},
	}

	sessionOutput := "main: 1 windows (created Mon Jan 27 10:00:00 2025)\nwork: 3 windows (created Mon Jan 27 11:00:00 2025) (attached)"
	base := startAPITestServer(t, containers, sessionOutput)

	resp, err := http.Get(base + "/api/containers/abc123/sessions")
	if err != nil {
		t.Fatalf("GET /api/containers/abc123/sessions error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode error = %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(result))
	}

	checkStringField(t, result[0], "name", "main")
	checkStringField(t, result[1], "name", "work")
}

// TestHandleGetContainer_GH17AC14 verifies unknown container ID returns 404 with error body.
func TestHandleGetContainer_GH17AC14(t *testing.T) {
	base := startAPITestServer(t, []container.Container{}, "")

	resp, err := http.Get(base + "/api/containers/unknown")
	if err != nil {
		t.Fatalf("GET /api/containers/unknown error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}

	if _, ok := body["error"]; !ok {
		t.Error("response missing error field")
	}
}

// runningContainer returns a container in running state for mutation tests.
func runningContainer(id string) container.Container {
	return container.Container{
		ID:     id,
		Name:   id + "-app-1",
		State:  container.StateRunning,
		Labels: map[string]string{},
	}
}

// stoppedContainer returns a container in stopped state for mutation tests.
func stoppedContainer(id string) container.Container {
	return container.Container{
		ID:     id,
		Name:   id + "-app-1",
		State:  container.StateStopped,
		Labels: map[string]string{},
	}
}

// postJSON sends a POST request with a JSON body.
func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal error = %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("POST %s error = %v", url, err)
	}
	return resp
}

// deleteRequest sends a DELETE request.
func deleteRequest(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("NewRequest DELETE error = %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s error = %v", url, err)
	}
	return resp
}

// TestHandleCreateSession_GH17AC21 verifies POST /api/containers/{id}/sessions creates a session and returns 201.
func TestHandleCreateSession_GH17AC21(t *testing.T) {
	containers := []container.Container{runningContainer("abc123")}
	// list-sessions returns empty (no existing sessions); new-session succeeds silently
	outputsByCmd := map[string]string{
		"list-sessions": "",
		"new-session":   "",
	}

	notifyCh := make(chan tea.Msg, 1)
	notifyFn := func(msg tea.Msg) { notifyCh <- msg }

	base := startMutationTestServer(t, containers, outputsByCmd, notifyFn)

	resp := postJSON(t, base+"/api/containers/abc123/sessions", map[string]string{"name": "dev"})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if result["name"] != "dev" {
		t.Errorf("name = %q, want %q", result["name"], "dev")
	}

	// GH-17.AC2.5: verify TUI notification was sent
	select {
	case msg := <-notifyCh:
		wsm, ok := msg.(tui.WebSessionActionMsg)
		if !ok {
			t.Fatalf("notifyTUI got %T, want tui.WebSessionActionMsg", msg)
		}
		if wsm.ContainerID != "abc123" {
			t.Errorf("ContainerID = %q, want %q", wsm.ContainerID, "abc123")
		}
	case <-time.After(time.Second):
		t.Error("notifyTUI was not called after successful session create")
	}
}

// TestHandleDestroySession_GH17AC22 verifies DELETE /api/containers/{id}/sessions/{name} destroys session and returns 200.
func TestHandleDestroySession_GH17AC22(t *testing.T) {
	containers := []container.Container{runningContainer("abc123")}
	outputsByCmd := map[string]string{
		"kill-session": "",
	}

	notifyCh := make(chan tea.Msg, 1)
	notifyFn := func(msg tea.Msg) { notifyCh <- msg }

	base := startMutationTestServer(t, containers, outputsByCmd, notifyFn)

	resp := deleteRequest(t, base+"/api/containers/abc123/sessions/dev")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if result["status"] != "destroyed" {
		t.Errorf("status = %q, want %q", result["status"], "destroyed")
	}

	// GH-17.AC2.5: verify TUI notification was sent
	select {
	case msg := <-notifyCh:
		wsm, ok := msg.(tui.WebSessionActionMsg)
		if !ok {
			t.Fatalf("notifyTUI got %T, want tui.WebSessionActionMsg", msg)
		}
		if wsm.ContainerID != "abc123" {
			t.Errorf("ContainerID = %q, want %q", wsm.ContainerID, "abc123")
		}
	case <-time.After(time.Second):
		t.Error("notifyTUI was not called after successful session destroy")
	}
}

// TestHandleCreateSession_GH17AC23 verifies creating a duplicate session returns 409.
func TestHandleCreateSession_GH17AC23(t *testing.T) {
	containers := []container.Container{runningContainer("abc123")}
	// list-sessions returns output containing "dev" session
	outputsByCmd := map[string]string{
		"list-sessions": "dev: 1 windows (created Mon Jan 27 10:00:00 2025)",
	}

	base := startMutationTestServer(t, containers, outputsByCmd, nil)

	resp := postJSON(t, base+"/api/containers/abc123/sessions", map[string]string{"name": "dev"})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Error("response missing error field")
	}
}

// TestHandleCreateSession_GH17AC24 verifies creating a session on a non-running container returns 400.
func TestHandleCreateSession_GH17AC24(t *testing.T) {
	containers := []container.Container{stoppedContainer("abc123")}
	base := startMutationTestServer(t, containers, map[string]string{}, nil)

	resp := postJSON(t, base+"/api/containers/abc123/sessions", map[string]string{"name": "dev"})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if !strings.Contains(body["error"], "not running") {
		t.Errorf("error = %q, want to contain %q", body["error"], "not running")
	}
}

// TestHandleDestroySession_GH17AC24 verifies destroying a session on a non-running container returns 400.
func TestHandleDestroySession_GH17AC24(t *testing.T) {
	containers := []container.Container{stoppedContainer("abc123")}
	base := startMutationTestServer(t, containers, map[string]string{}, nil)

	resp := deleteRequest(t, base+"/api/containers/abc123/sessions/dev")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if !strings.Contains(body["error"], "not running") {
		t.Errorf("error = %q, want to contain %q", body["error"], "not running")
	}
}

// TestHandleGetProjects_AC11 verifies GET /api/projects returns projects with worktrees and nested containers matched by path.
// web-lifecycle-ops.AC1.1 Success: GET /api/projects returns projects with worktrees and nested container data when container.ProjectPath matches worktree.Path
func TestHandleGetProjects_AC11(t *testing.T) {
	// Create a container with ProjectPath matching a project path
	projectPath := "/home/user/project1"
	containers := []container.Container{
		{
			ID:          "container1",
			Name:        "devcontainer",
			State:       container.StateRunning,
			Template:    "template1",
			ProjectPath: projectPath,
			RemoteUser:  "user",
			CreatedAt:   time.Now(),
		},
	}

	// Create a discovered project with the same path
	projects := []discovery.DiscoveredProject{
		{
			Name:        "project1",
			Path:        projectPath,
			HasMakefile: true,
			Worktrees:   []discovery.Worktree{},
		},
	}

	base := startProjectsTestServer(t, containers, "", projects)

	resp, err := http.Get(base + "/api/projects")
	if err != nil {
		t.Fatalf("GET /api/projects error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}

	// Verify projects array exists
	projectsArr, ok := body["projects"].([]any)
	if !ok {
		t.Fatalf("projects field missing or not an array")
	}
	if len(projectsArr) != 1 {
		t.Errorf("projects array length = %d, want 1", len(projectsArr))
	}

	// Get first project
	proj := projectsArr[0].(map[string]any)
	checkStringField(t, proj, "name", "project1")
	checkStringField(t, proj, "path", projectPath)

	// Verify worktrees array
	worktreesArr, ok := proj["worktrees"].([]any)
	if !ok {
		t.Fatalf("worktrees field missing or not an array")
	}
	if len(worktreesArr) != 1 {
		t.Errorf("worktrees array length = %d, want 1", len(worktreesArr))
	}

	// Verify main worktree has container data
	mainWt := worktreesArr[0].(map[string]any)
	checkStringField(t, mainWt, "name", "main")
	checkStringField(t, mainWt, "path", projectPath)

	// Verify container is nested in worktree
	containerData, ok := mainWt["container"].(map[string]any)
	if !ok {
		t.Fatalf("container field missing or not an object in main worktree")
	}
	checkStringField(t, containerData, "id", "container1")
	checkStringField(t, containerData, "name", "devcontainer")
}

// TestHandleGetProjects_AC12 verifies main worktree has is_main: true; linked worktrees have is_main: false.
// web-lifecycle-ops.AC1.2 Success: Main worktree (project root) has is_main: true; linked worktrees have is_main: false
func TestHandleGetProjects_AC12(t *testing.T) {
	projectPath := "/home/user/project2"
	linkedWorktreePath := "/home/user/project2/.worktrees/feature"

	// Create a container for the linked worktree
	containers := []container.Container{
		{
			ID:          "container2",
			Name:        "feature-container",
			State:       container.StateRunning,
			Template:    "template1",
			ProjectPath: linkedWorktreePath,
			RemoteUser:  "user",
			CreatedAt:   time.Now(),
		},
	}

	// Create a project with linked worktrees
	projects := []discovery.DiscoveredProject{
		{
			Name:        "project2",
			Path:        projectPath,
			HasMakefile: false,
			Worktrees: []discovery.Worktree{
				{
					Name:   "feature",
					Path:   linkedWorktreePath,
					Branch: "feature-branch",
				},
			},
		},
	}

	base := startProjectsTestServer(t, containers, "", projects)

	resp, err := http.Get(base + "/api/projects")
	if err != nil {
		t.Fatalf("GET /api/projects error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}

	projectsArr, ok := body["projects"].([]any)
	if !ok {
		t.Fatalf("projects field missing or not an array")
	}

	proj := projectsArr[0].(map[string]any)
	worktreesArr, ok := proj["worktrees"].([]any)
	if !ok {
		t.Fatalf("worktrees field missing or not an array")
	}
	if len(worktreesArr) != 2 {
		t.Errorf("worktrees array length = %d, want 2 (main + 1 linked)", len(worktreesArr))
	}

	// Main worktree should be first
	mainWt := worktreesArr[0].(map[string]any)
	isMain, ok := mainWt["is_main"].(bool)
	if !ok {
		t.Errorf("is_main field missing or not a bool in main worktree")
	}
	if !isMain {
		t.Errorf("main worktree is_main = %v, want true", isMain)
	}

	// Linked worktree should be second
	linkedWt := worktreesArr[1].(map[string]any)
	checkStringField(t, linkedWt, "name", "feature")
	checkStringField(t, linkedWt, "path", linkedWorktreePath)

	isMainLinked, ok := linkedWt["is_main"].(bool)
	if !ok {
		t.Errorf("is_main field missing or not a bool in linked worktree")
	}
	if isMainLinked {
		t.Errorf("linked worktree is_main = %v, want false", isMainLinked)
	}

	// Verify container is in the linked worktree
	containerData, ok := linkedWt["container"].(map[string]any)
	if !ok {
		t.Fatalf("container field missing or not an object in linked worktree")
	}
	checkStringField(t, containerData, "id", "container2")
}

// TestHandleGetProjects_AC13 verifies project with discovered worktrees but no matching containers returns container: null.
// web-lifecycle-ops.AC1.3 Edge: Project with no containers returns worktrees with container: null
func TestHandleGetProjects_AC13(t *testing.T) {
	projectPath := "/home/user/project3"

	// No containers - all worktrees should have container: null
	containers := []container.Container{}

	// Create a project with worktrees but no matching containers
	projects := []discovery.DiscoveredProject{
		{
			Name:        "project3",
			Path:        projectPath,
			HasMakefile: true,
			Worktrees: []discovery.Worktree{
				{
					Name:   "main-branch",
					Path:   "/home/user/project3/.worktrees/main-branch",
					Branch: "main",
				},
			},
		},
	}

	base := startProjectsTestServer(t, containers, "", projects)

	resp, err := http.Get(base + "/api/projects")
	if err != nil {
		t.Fatalf("GET /api/projects error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}

	projectsArr, ok := body["projects"].([]any)
	if !ok {
		t.Fatalf("projects field missing or not an array")
	}

	proj := projectsArr[0].(map[string]any)
	worktreesArr, ok := proj["worktrees"].([]any)
	if !ok {
		t.Fatalf("worktrees field missing or not an array")
	}
	if len(worktreesArr) != 2 {
		t.Errorf("worktrees array length = %d, want 2 (main + 1 linked)", len(worktreesArr))
	}

	// Main worktree should have container: null
	mainWt := worktreesArr[0].(map[string]any)
	containerValue := mainWt["container"]
	if containerValue != nil {
		t.Errorf("main worktree container = %v, want null", containerValue)
	}

	// Linked worktree should also have container: null
	linkedWt := worktreesArr[1].(map[string]any)
	containerValueLinked := linkedWt["container"]
	if containerValueLinked != nil {
		t.Errorf("linked worktree container = %v, want null", containerValueLinked)
	}
}

// TestHandleStartContainer_AC21 verifies POST /api/containers/{id}/start on a stopped container returns 200 and sends TUI notification.
// web-lifecycle-ops.AC2.1 Success: POST /api/containers/{id}/start starts a stopped container
func TestHandleStartContainer_AC21(t *testing.T) {
	containers := []container.Container{
		{
			ID:          "abc123",
			Name:        "abc123-app-1",
			State:       container.StateStopped,
			ProjectPath: "/home/user/myproject",
			Labels:      map[string]string{},
		},
	}

	notifyCh := make(chan tea.Msg, 1)
	notifyFn := func(msg tea.Msg) { notifyCh <- msg }

	base := startMutationTestServer(t, containers, map[string]string{}, notifyFn)

	resp := postJSON(t, base+"/api/containers/abc123/start", nil)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if result["status"] != "started" {
		t.Errorf("status = %q, want %q", result["status"], "started")
	}

	// Verify TUI notification was sent
	select {
	case msg := <-notifyCh:
		wsm, ok := msg.(tui.WebSessionActionMsg)
		if !ok {
			t.Fatalf("notifyTUI got %T, want tui.WebSessionActionMsg", msg)
		}
		if wsm.ContainerID != "abc123" {
			t.Errorf("ContainerID = %q, want %q", wsm.ContainerID, "abc123")
		}
	case <-time.After(time.Second):
		t.Error("notifyTUI was not called after successful container start")
	}
}

// TestHandleStopContainer_AC22 verifies POST /api/containers/{id}/stop on a running container returns 200 and sends TUI notification.
// web-lifecycle-ops.AC2.2 Success: POST /api/containers/{id}/stop stops a running container
func TestHandleStopContainer_AC22(t *testing.T) {
	containers := []container.Container{
		{
			ID:          "abc123",
			Name:        "abc123-app-1",
			State:       container.StateRunning,
			ProjectPath: "/home/user/myproject",
			Labels:      map[string]string{},
		},
	}

	notifyCh := make(chan tea.Msg, 1)
	notifyFn := func(msg tea.Msg) { notifyCh <- msg }

	base := startMutationTestServer(t, containers, map[string]string{}, notifyFn)

	resp := postJSON(t, base+"/api/containers/abc123/stop", nil)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if result["status"] != "stopped" {
		t.Errorf("status = %q, want %q", result["status"], "stopped")
	}

	// Verify TUI notification was sent
	select {
	case msg := <-notifyCh:
		wsm, ok := msg.(tui.WebSessionActionMsg)
		if !ok {
			t.Fatalf("notifyTUI got %T, want tui.WebSessionActionMsg", msg)
		}
		if wsm.ContainerID != "abc123" {
			t.Errorf("ContainerID = %q, want %q", wsm.ContainerID, "abc123")
		}
	case <-time.After(time.Second):
		t.Error("notifyTUI was not called after successful container stop")
	}
}

// TestHandleDestroyContainer_AC23 verifies DELETE /api/containers/{id} destroys a container and returns 200 with TUI notification.
// web-lifecycle-ops.AC2.3 Success: DELETE /api/containers/{id} destroys a container
func TestHandleDestroyContainer_AC23(t *testing.T) {
	containers := []container.Container{
		{
			ID:          "abc123",
			Name:        "abc123-app-1",
			State:       container.StateRunning,
			ProjectPath: "/home/user/myproject",
			Labels:      map[string]string{},
		},
	}

	notifyCh := make(chan tea.Msg, 1)
	notifyFn := func(msg tea.Msg) { notifyCh <- msg }

	base := startMutationTestServer(t, containers, map[string]string{}, notifyFn)

	resp := deleteRequest(t, base+"/api/containers/abc123")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if result["status"] != "destroyed" {
		t.Errorf("status = %q, want %q", result["status"], "destroyed")
	}

	// Verify TUI notification was sent
	select {
	case msg := <-notifyCh:
		wsm, ok := msg.(tui.WebSessionActionMsg)
		if !ok {
			t.Fatalf("notifyTUI got %T, want tui.WebSessionActionMsg", msg)
		}
		if wsm.ContainerID != "abc123" {
			t.Errorf("ContainerID = %q, want %q", wsm.ContainerID, "abc123")
		}
	case <-time.After(time.Second):
		t.Error("notifyTUI was not called after successful container destroy")
	}
}

// TestHandleStartContainer_AC24_Nonexistent verifies POST /api/containers/{id}/start on nonexistent container returns 404.
// web-lifecycle-ops.AC2.4 Failure: Start on nonexistent container returns 404
func TestHandleStartContainer_AC24_Nonexistent(t *testing.T) {
	base := startMutationTestServer(t, []container.Container{}, map[string]string{}, nil)

	resp := postJSON(t, base+"/api/containers/unknown/start", nil)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Error("response missing error field")
	}
}

// TestHandleStopContainer_AC24_Nonexistent verifies POST /api/containers/{id}/stop on nonexistent container returns 404.
// web-lifecycle-ops.AC2.4 Failure: Stop on nonexistent container returns 404
func TestHandleStopContainer_AC24_Nonexistent(t *testing.T) {
	base := startMutationTestServer(t, []container.Container{}, map[string]string{}, nil)

	resp := postJSON(t, base+"/api/containers/unknown/stop", nil)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Error("response missing error field")
	}
}

// TestHandleDestroyContainer_AC24_Nonexistent verifies DELETE /api/containers/{id} on nonexistent container returns 404.
// web-lifecycle-ops.AC2.4 Failure: Destroy on nonexistent container returns 404
func TestHandleDestroyContainer_AC24_Nonexistent(t *testing.T) {
	base := startMutationTestServer(t, []container.Container{}, map[string]string{}, nil)

	resp := deleteRequest(t, base+"/api/containers/unknown")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Error("response missing error field")
	}
}

// TestHandleStartContainer_AC25_AlreadyRunning verifies POST /api/containers/{id}/start on already-running container returns 400.
// web-lifecycle-ops.AC2.5 Failure: Start on already-running container returns 400
func TestHandleStartContainer_AC25_AlreadyRunning(t *testing.T) {
	containers := []container.Container{
		{
			ID:          "abc123",
			Name:        "abc123-app-1",
			State:       container.StateRunning,
			ProjectPath: "/home/user/myproject",
			Labels:      map[string]string{},
		},
	}

	base := startMutationTestServer(t, containers, map[string]string{}, nil)

	resp := postJSON(t, base+"/api/containers/abc123/start", nil)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if !strings.Contains(body["error"], "already running") {
		t.Errorf("error = %q, want to contain %q", body["error"], "already running")
	}
}

// TestHandleStopContainer_AlreadyStopped verifies POST /api/containers/{id}/stop on already-stopped container returns 400.
// web-lifecycle-ops AC: Stop on already-stopped container returns 400 with "not running" error
func TestHandleStopContainer_AlreadyStopped(t *testing.T) {
	containers := []container.Container{
		{
			ID:          "abc123",
			Name:        "abc123-app-1",
			State:       container.StateStopped,
			ProjectPath: "/home/user/myproject",
			Labels:      map[string]string{},
		},
	}

	base := startMutationTestServer(t, containers, map[string]string{}, nil)

	resp := postJSON(t, base+"/api/containers/abc123/stop", nil)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if !strings.Contains(body["error"], "not running") {
		t.Errorf("error = %q, want to contain %q", body["error"], "not running")
	}
}

// TestHandleCreateWorktree_AC31 verifies POST /api/projects/{path}/worktrees with valid name
// creates worktree, starts container, and returns 201 with name, path, container_id.
// web-lifecycle-ops.AC3.1: Create worktree and auto-start container
func TestHandleCreateWorktree_AC31(t *testing.T) {
	projectPath := "/home/user/myproject"
	encodedPath := base64.URLEncoding.EncodeToString([]byte(projectPath))
	wtPath := "/home/user/myproject/.git/worktrees/feature-x"

	wt := &mockWorktreeOps{
		createPath: wtPath,
	}

	base := startWorktreeTestServer(t, []container.Container{}, wt, nil)

	resp := postJSON(t, base+"/api/projects/"+encodedPath+"/worktrees", map[string]string{"name": "feature-x"})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
		return
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}

	checkStringField(t, body, "name", "feature-x")
	checkStringField(t, body, "path", wtPath)
	checkStringField(t, body, "container_id", "mock-container-abc123")
}

// TestHandleCreateWorktree_AC32 verifies POST with invalid name returns 400.
// web-lifecycle-ops.AC3.2: Invalid name returns 400
func TestHandleCreateWorktree_AC32(t *testing.T) {
	projectPath := "/home/user/myproject"
	encodedPath := base64.URLEncoding.EncodeToString([]byte(projectPath))

	wt := &mockWorktreeOps{
		validateErr: fmt.Errorf("invalid worktree name: contains invalid characters"),
	}

	base := startWorktreeTestServer(t, []container.Container{}, wt, nil)

	resp := postJSON(t, base+"/api/projects/"+encodedPath+"/worktrees", map[string]string{"name": "invalid@name"})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body["error"] == "" {
		t.Error("expected error message in response")
	}
}

// TestHandleCreateWorktree_AC33 verifies POST with duplicate branch name returns 409.
// web-lifecycle-ops.AC3.3: Duplicate branch name returns 409
func TestHandleCreateWorktree_AC33(t *testing.T) {
	projectPath := "/home/user/myproject"
	encodedPath := base64.URLEncoding.EncodeToString([]byte(projectPath))

	wt := &mockWorktreeOps{
		createErr: fmt.Errorf("fatal: 'feature-x' already exists"),
	}

	base := startWorktreeTestServer(t, []container.Container{}, wt, nil)

	resp := postJSON(t, base+"/api/projects/"+encodedPath+"/worktrees", map[string]string{"name": "feature-x"})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if !strings.Contains(body["error"], "already exists") {
		t.Errorf("error = %q, want to contain %q", body["error"], "already exists")
	}
}

// TestHandleDeleteWorktree_AC34 verifies DELETE /api/projects/{path}/worktrees/{name}
// performs stop + destroy + git worktree remove and returns 200.
// web-lifecycle-ops.AC3.4: Delete worktree
func TestHandleDeleteWorktree_AC34(t *testing.T) {
	projectPath := "/home/user/myproject"
	encodedPath := base64.URLEncoding.EncodeToString([]byte(projectPath))
	wtPath := "/home/user/myproject/.git/worktrees/feature-x"

	containers := []container.Container{
		{
			ID:          "container-abc",
			Name:        "myproject-app-1",
			State:       container.StateRunning,
			ProjectPath: wtPath,
			Labels:      map[string]string{},
		},
	}

	wt := &mockWorktreeOps{
		wtDir: wtPath,
	}

	base := startWorktreeTestServer(t, containers, wt, nil)

	resp := deleteRequest(t, base+"/api/projects/"+encodedPath+"/worktrees/feature-x")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	checkStringField(t, body, "status", "deleted")
}

// TestHandleDeleteWorktree_AC35 verifies DELETE with dirty worktree returns error with descriptive message.
// web-lifecycle-ops.AC3.5: Dirty worktree returns error
func TestHandleDeleteWorktree_AC35(t *testing.T) {
	projectPath := "/home/user/myproject"
	encodedPath := base64.URLEncoding.EncodeToString([]byte(projectPath))
	wtPath := "/home/user/myproject/.git/worktrees/feature-x"

	wt := &mockWorktreeOps{
		wtDir:      wtPath,
		destroyErr: fmt.Errorf("error: worktree feature-x is dirty, cannot remove"),
	}

	base := startWorktreeTestServer(t, []container.Container{}, wt, nil)

	resp := deleteRequest(t, base+"/api/projects/"+encodedPath+"/worktrees/feature-x")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if !strings.Contains(body["error"], "dirty") {
		t.Errorf("error = %q, want to contain %q", body["error"], "dirty")
	}
}

// startWorktreeContainerMockRuntime is a mock runtime that can return different containers
// on sequential ListContainers calls, used to simulate container creation during tests.
type startWorktreeContainerMockRuntime struct {
	// initialContainers returned on first ListContainers call
	initialContainers []container.Container
	// afterUpContainers returned on subsequent ListContainers calls (after devcontainer up)
	afterUpContainers []container.Container
	// listCallCount tracks how many times ListContainers has been called
	listCallCount int
	// outputsByCmd maps tmux subcommand to output
	outputsByCmd map[string]string
}

func (m *startWorktreeContainerMockRuntime) ListContainers(_ context.Context) ([]container.Container, error) {
	m.listCallCount++
	// Return afterUpContainers on subsequent calls (after devcontainer up has been called)
	if m.listCallCount > 1 {
		return m.afterUpContainers, nil
	}
	return m.initialContainers, nil
}

func (m *startWorktreeContainerMockRuntime) StartContainer(_ context.Context, _ string) error {
	return nil
}
func (m *startWorktreeContainerMockRuntime) StopContainer(_ context.Context, _ string) error {
	return nil
}
func (m *startWorktreeContainerMockRuntime) RemoveContainer(_ context.Context, _ string) error {
	return nil
}

func (m *startWorktreeContainerMockRuntime) Exec(_ context.Context, _ string, _ []string) (string, error) {
	return "", nil
}

func (m *startWorktreeContainerMockRuntime) ExecAs(_ context.Context, _ string, _ string, cmd []string) (string, error) {
	for _, arg := range cmd {
		if out, ok := m.outputsByCmd[arg]; ok {
			return out, nil
		}
	}
	return "", nil
}

func (m *startWorktreeContainerMockRuntime) InspectContainer(_ context.Context, _ string) (container.ContainerState, error) {
	return container.StateRunning, nil
}

func (m *startWorktreeContainerMockRuntime) GetIsolationInfo(_ context.Context, _ string) (*container.IsolationInfo, error) {
	return &container.IsolationInfo{}, nil
}

func (m *startWorktreeContainerMockRuntime) ComposeUp(_ context.Context, _ string, _ string) error {
	return nil
}
func (m *startWorktreeContainerMockRuntime) ComposeStart(_ context.Context, _ string, _ string) error {
	return nil
}
func (m *startWorktreeContainerMockRuntime) ComposeStop(_ context.Context, _ string, _ string) error {
	return nil
}
func (m *startWorktreeContainerMockRuntime) ComposeDown(_ context.Context, _ string, _ string) error {
	return nil
}

// startWorktreeContainerTestServer creates a test server with support for testing container creation.
// It uses a special mock runtime that returns different containers before/after devcontainer up.
func startWorktreeContainerTestServer(
	t *testing.T,
	initialContainers []container.Container,
	afterUpContainers []container.Container,
	wt *mockWorktreeOps,
	notifyTUI func(tea.Msg),
) string {
	t.Helper()
	runtime := &startWorktreeContainerMockRuntime{
		initialContainers: initialContainers,
		afterUpContainers: afterUpContainers,
		outputsByCmd:      make(map[string]string),
	}

	// Create DevcontainerCLI with mock executor
	devCLI := container.NewDevcontainerCLIWithExecutor(mockCommandExecutor)

	mgr := container.NewManagerWithDeps(runtime, nil, devCLI)
	if err := mgr.Refresh(context.Background()); err != nil {
		t.Fatalf("manager.Refresh() error = %v", err)
	}
	lm := logging.NewTestLogManager(10)
	t.Cleanup(func() { _ = lm.Close() })

	s := web.New(web.Config{Bind: "127.0.0.1", Port: 0}, mgr, notifyTUI, lm, nil)
	s.SetWorktreeOpsForTest(wt)

	ln, err := s.Listen()
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- s.Serve(ln) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
		<-done
	})
	return "http://" + s.Addr()
}

// TestHandleStartWorktreeContainer_AC21 verifies POST /api/projects/{path}/worktrees/{name}/start
// with no existing container returns 201 with container response JSON.
// start-missing-container.AC2.1: Success case - container created and returned
func TestHandleStartWorktreeContainer_AC21(t *testing.T) {
	// Create a temporary directory to act as the worktree
	tmpDir := t.TempDir()
	wtPath := tmpDir + "/feature-x"
	if err := os.Mkdir(wtPath, 0755); err != nil {
		t.Fatalf("mkdir error = %v", err)
	}

	projectPath := tmpDir
	encodedPath := base64.URLEncoding.EncodeToString([]byte(projectPath))

	wt := &mockWorktreeOps{
		wtDir: wtPath,
	}

	// After devcontainer up succeeds, this container should be returned by ListContainers
	afterUpContainers := []container.Container{
		{
			ID:          "mock-container-abc123",
			Name:        "myproject-app-1",
			State:       container.StateRunning,
			Template:    "go",
			ProjectPath: wtPath,
			RemoteUser:  "vscode",
			CreatedAt:   time.Now().UTC(),
			Labels:      map[string]string{},
		},
	}

	base := startWorktreeContainerTestServer(t, []container.Container{}, afterUpContainers, wt, nil)

	resp, err := http.Post(base+"/api/projects/"+encodedPath+"/worktrees/feature-x/start", "application/json", nil)
	if err != nil {
		t.Fatalf("POST error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
		body, _ := io.ReadAll(resp.Body)
		t.Logf("response body: %s", string(body))
		return
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode error = %v", err)
	}

	// Should have container response fields
	if id, ok := result["id"]; !ok || id == "" {
		t.Errorf("response missing or empty id field")
	}
	if state, ok := result["state"]; !ok || state != "running" {
		t.Errorf("response state = %v, want 'running'", state)
	}
}

// TestHandleStartWorktreeContainer_AC23 verifies TUI notification is sent.
// start-missing-container.AC2.3: TUI notification sent
func TestHandleStartWorktreeContainer_AC23(t *testing.T) {
	// Create a temporary directory to act as the worktree
	tmpDir := t.TempDir()
	wtPath := tmpDir + "/feature-x"
	if err := os.Mkdir(wtPath, 0755); err != nil {
		t.Fatalf("mkdir error = %v", err)
	}

	projectPath := tmpDir
	encodedPath := base64.URLEncoding.EncodeToString([]byte(projectPath))

	wt := &mockWorktreeOps{
		wtDir: wtPath,
	}

	afterUpContainers := []container.Container{
		{
			ID:          "mock-container-abc123",
			Name:        "myproject-app-1",
			State:       container.StateRunning,
			Template:    "go",
			ProjectPath: wtPath,
			RemoteUser:  "vscode",
			CreatedAt:   time.Now().UTC(),
			Labels:      map[string]string{},
		},
	}

	notifyCh := make(chan tea.Msg, 1)
	notifyFn := func(msg tea.Msg) { notifyCh <- msg }

	base := startWorktreeContainerTestServer(t, []container.Container{}, afterUpContainers, wt, notifyFn)

	resp, err := http.Post(base+"/api/projects/"+encodedPath+"/worktrees/feature-x/start", "application/json", nil)
	if err != nil {
		t.Fatalf("POST error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
		return
	}

	// Verify TUI was notified
	select {
	case msg := <-notifyCh:
		if actionMsg, ok := msg.(tui.WebSessionActionMsg); !ok || actionMsg.ContainerID == "" {
			t.Errorf("expected WebSessionActionMsg with non-empty ContainerID, got %T", msg)
		}
	case <-time.After(1 * time.Second):
		t.Error("timed out waiting for TUI notification")
	}
}

// TestHandleStartWorktreeContainer_AC24 verifies 404 when worktree not found.
// start-missing-container.AC2.4: 404 when worktree not found
func TestHandleStartWorktreeContainer_AC24(t *testing.T) {
	projectPath := "/home/user/myproject"
	encodedPath := base64.URLEncoding.EncodeToString([]byte(projectPath))

	wt := &mockWorktreeOps{
		wtDir: "/nonexistent/path", // Doesn't exist on disk
	}

	base := startWorktreeContainerTestServer(t, []container.Container{}, []container.Container{}, wt, nil)

	resp, err := http.Post(base+"/api/projects/"+encodedPath+"/worktrees/feature-x/start", "application/json", nil)
	if err != nil {
		t.Fatalf("POST error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d (not found)", resp.StatusCode, http.StatusNotFound)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body["error"] != "worktree not found" {
		t.Errorf("error = %q, want %q", body["error"], "worktree not found")
	}
}

// TestHandleStartWorktreeContainer_AC24_BadEncoding verifies 400 with malformed encoding.
func TestHandleStartWorktreeContainer_AC24_BadEncoding(t *testing.T) {
	wt := &mockWorktreeOps{
		wtDir: "/home/user/myproject/.git/worktrees/feature-x",
	}

	base := startWorktreeContainerTestServer(t, []container.Container{}, []container.Container{}, wt, nil)

	// Use invalid base64 encoding
	resp, err := http.Post(base+"/api/projects/invalid!!!path/worktrees/feature-x/start", "application/json", nil)
	if err != nil {
		t.Fatalf("POST error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d (bad request)", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestHandleStartWorktreeContainer_AC25 verifies 409 when container already exists.
// start-missing-container.AC2.5: 409 when container exists
func TestHandleStartWorktreeContainer_AC25(t *testing.T) {
	// Create a temporary directory to act as the worktree
	tmpDir := t.TempDir()
	wtPath := tmpDir + "/feature-x"
	if err := os.Mkdir(wtPath, 0755); err != nil {
		t.Fatalf("mkdir error = %v", err)
	}

	projectPath := tmpDir
	encodedPath := base64.URLEncoding.EncodeToString([]byte(projectPath))

	wt := &mockWorktreeOps{
		wtDir: wtPath,
	}

	// Container already exists for this worktree
	existingContainers := []container.Container{
		{
			ID:          "existing-container-123",
			Name:        "myproject-app-1",
			State:       container.StateRunning,
			Template:    "go",
			ProjectPath: wtPath, // Matches the worktree path
			RemoteUser:  "vscode",
			CreatedAt:   time.Now().UTC(),
			Labels:      map[string]string{},
		},
	}

	base := startWorktreeContainerTestServer(t, existingContainers, existingContainers, wt, nil)

	resp, err := http.Post(base+"/api/projects/"+encodedPath+"/worktrees/feature-x/start", "application/json", nil)
	if err != nil {
		t.Fatalf("POST error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want %d (conflict)", resp.StatusCode, http.StatusConflict)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if body["error"] != "worktree already has a container" {
		t.Errorf("error = %q, want %q", body["error"], "worktree already has a container")
	}
}

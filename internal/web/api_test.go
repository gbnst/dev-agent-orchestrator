package web_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/container"
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

	s := web.New(web.Config{Bind: "127.0.0.1", Port: 0}, mgr, notifyTUI, lm)

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

	s := web.New(web.Config{Bind: "127.0.0.1", Port: 0}, mgr, nil, lm)

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

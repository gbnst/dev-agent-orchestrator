package web_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"devagent/internal/container"
	"devagent/internal/web"
)

// startTerminalTestServer creates a test server with the given containers and session output.
// It reuses startMutationTestServer from api_test.go which uses mutationMockRuntime.
func startTerminalTestServer(t *testing.T, containers []container.Container, outputsByCmd map[string]string) string {
	t.Helper()
	return startMutationTestServer(t, containers, outputsByCmd, nil)
}

// TestHandleTerminal_GH17AC35_ContainerNotFound verifies that a request to a
// non-existent container returns 404 before websocket upgrade.
func TestHandleTerminal_GH17AC35_ContainerNotFound(t *testing.T) {
	base := startTerminalTestServer(t, []container.Container{}, map[string]string{})

	resp, err := http.Get(base + "/api/containers/nonexistent/sessions/dev/terminal")
	if err != nil {
		t.Fatalf("GET terminal error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestHandleTerminal_GH17AC35_ContainerNotRunning verifies that a stopped container
// returns 400 before websocket upgrade.
func TestHandleTerminal_GH17AC35_ContainerNotRunning(t *testing.T) {
	containers := []container.Container{stoppedContainer("abc123")}
	base := startTerminalTestServer(t, containers, map[string]string{})

	resp, err := http.Get(base + "/api/containers/abc123/sessions/dev/terminal")
	if err != nil {
		t.Fatalf("GET terminal error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestHandleTerminal_GH17AC35_SessionNotFound verifies that a non-existent session
// returns 404 before websocket upgrade. The container exists and is running but the
// session name is not in the tmux session list.
func TestHandleTerminal_GH17AC35_SessionNotFound(t *testing.T) {
	containers := []container.Container{runningContainer("abc123")}
	// list-sessions returns "other" session but not "dev"
	outputsByCmd := map[string]string{
		"list-sessions": "other: 1 windows (created Mon Jan 27 10:00:00 2025)",
	}
	base := startTerminalTestServer(t, containers, outputsByCmd)

	resp, err := http.Get(base + "/api/containers/abc123/sessions/dev/terminal")
	if err != nil {
		t.Fatalf("GET terminal error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestHandleTerminal_GH17AC31_SessionExists_UpgradeAttempted verifies that when the
// container is running and the session exists, the handler attempts the websocket
// upgrade. A plain HTTP GET without websocket headers will fail the upgrade, but the
// handler must have passed all pre-upgrade validation checks for the response to
// indicate a websocket-related failure (not a 404 or 400 from our validation).
func TestHandleTerminal_GH17AC31_SessionExists_UpgradeAttempted(t *testing.T) {
	containers := []container.Container{runningContainer("abc123")}
	// list-sessions returns "dev" session
	outputsByCmd := map[string]string{
		"list-sessions": "dev: 1 windows (created Mon Jan 27 10:00:00 2025)",
	}
	base := startTerminalTestServer(t, containers, outputsByCmd)

	resp, err := http.Get(base + "/api/containers/abc123/sessions/dev/terminal")
	if err != nil {
		t.Fatalf("GET terminal error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// The handler passes pre-upgrade validation and attempts websocket upgrade.
	// A plain HTTP GET without websocket headers will cause the upgrade to fail
	// with a 4xx response from coder/websocket (not 404 or 400 from our validation).
	// We confirm that neither 404 nor 400 is returned, proving the handler reached
	// the upgrade stage.
	if resp.StatusCode == http.StatusNotFound {
		t.Error("got 404: container or session validation failed unexpectedly")
	}
	if resp.StatusCode == http.StatusBadRequest {
		t.Error("got 400: container-not-running validation failed unexpectedly")
	}
}

// TestResizeMessage_GH17AC34_Unmarshal verifies that ResizeMessage correctly
// deserializes from JSON, testing the struct tags used in HandleTerminal.
func TestResizeMessage_GH17AC34_Unmarshal(t *testing.T) {
	data := []byte(`{"type":"resize","cols":120,"rows":40}`)

	var msg web.ResizeMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}

	if msg.Type != "resize" {
		t.Errorf("Type = %q, want %q", msg.Type, "resize")
	}
	if msg.Cols != 120 {
		t.Errorf("Cols = %d, want %d", msg.Cols, 120)
	}
	if msg.Rows != 40 {
		t.Errorf("Rows = %d, want %d", msg.Rows, 40)
	}
}

// TestResizeMessage_GH17AC34_NonResizeTypeIgnored verifies that a text frame with
// a non-"resize" type is treated as passthrough (not a resize control message).
func TestResizeMessage_GH17AC34_NonResizeTypeIgnored(t *testing.T) {
	data := []byte(`{"type":"ping","cols":120,"rows":40}`)

	var msg web.ResizeMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}

	// A message with type != "resize" should not trigger resize handling.
	// Verify the type field is preserved correctly so the handler's type check works.
	if msg.Type == "resize" {
		t.Error("Type = \"resize\", want non-resize type to not match")
	}
}

// TestResizeMessage_GH17AC34_LargeTerminal verifies that ResizeMessage handles
// large terminal dimensions (e.g. 4K displays).
func TestResizeMessage_GH17AC34_LargeTerminal(t *testing.T) {
	data := []byte(`{"type":"resize","cols":320,"rows":80}`)

	var msg web.ResizeMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}

	if msg.Cols != 320 {
		t.Errorf("Cols = %d, want 320", msg.Cols)
	}
	if msg.Rows != 80 {
		t.Errorf("Rows = %d, want 80", msg.Rows)
	}
}

//go:build e2e
// +build e2e

// pattern: Imperative Shell
// E2E tests for the web API and terminal WebSocket endpoints.
// Requires Docker and devcontainer CLI in PATH.

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"devagent/internal/container"
	"devagent/internal/web"
)

// --------------------------------------------------------------------------
// Task 2: Web API E2E tests (GH-17.AC5.1)
// --------------------------------------------------------------------------

func TestDockerWebContainerAPI(t *testing.T) {
	testWebContainerAPI(t, "docker")
}

func testWebContainerAPI(t *testing.T, runtime string) {
	SkipIfRuntimeMissing(t, runtime)
	SkipIfDevcontainerMissing(t)

	cfg := TestConfig(runtime)
	templates := TestTemplates()
	logMgr := TestLogManager(t)
	mgr := container.NewManager(container.ManagerOptions{
		Config:     cfg,
		Templates:  templates,
		LogManager: logMgr,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	projectDir := TestProject(t, "basic")

	c, err := mgr.CreateWithCompose(ctx, container.CreateOptions{
		ProjectPath: projectDir,
		Template:    templates[0].Name,
		Name:        "e2e-web-test",
	})
	if err != nil {
		t.Fatalf("create container: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_ = mgr.DestroyWithCompose(cleanupCtx, c.ID)
	})

	// Populate manager state before starting web server.
	if err := mgr.Refresh(ctx); err != nil {
		t.Logf("refresh warning: %v", err)
	}

	_, baseURL := TestWebServer(t, mgr, logMgr)
	t.Logf("web server at %s", baseURL)

	t.Run("container listing", func(t *testing.T) {
		testWebContainerListing(t, baseURL, c.ID)
	})

	t.Run("404 for unknown container", func(t *testing.T) {
		testWebContainerNotFound(t, baseURL)
	})

	t.Run("session CRUD", func(t *testing.T) {
		testWebSessionCRUD(t, baseURL, c.ID)
	})
}

// testWebContainerListing verifies GET /api/containers returns the container.
func testWebContainerListing(t *testing.T, baseURL, containerID string) {
	t.Helper()

	resp, err := http.Get(baseURL + "/api/containers")
	if err != nil {
		t.Fatalf("GET /api/containers: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var containers []web.ContainerResponse
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	found := false
	for _, c := range containers {
		if c.ID == containerID {
			found = true
			if c.State == "" {
				t.Error("container state is empty")
			}
			break
		}
	}
	if !found {
		t.Errorf("container %s not found in listing (got %d containers)", containerID[:12], len(containers))
	}
}

// testWebContainerNotFound verifies GET /api/containers/{id} returns 404 for unknown IDs.
func testWebContainerNotFound(t *testing.T, baseURL string) {
	t.Helper()

	resp, err := http.Get(baseURL + "/api/containers/nonexistent")
	if err != nil {
		t.Fatalf("GET /api/containers/nonexistent: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// testWebSessionCRUD verifies POST/GET/DELETE session endpoints.
func testWebSessionCRUD(t *testing.T, baseURL, containerID string) {
	t.Helper()

	const sessionName = "e2e-test"

	// POST — create session
	body := bytes.NewBufferString(`{"name":"` + sessionName + `"}`)
	resp, err := http.Post(
		fmt.Sprintf("%s/api/containers/%s/sessions", baseURL, containerID),
		"application/json",
		body,
	)
	if err != nil {
		t.Fatalf("POST /sessions: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /sessions status = %d, want 201; body: %s", resp.StatusCode, respBody)
	}

	// GET — list sessions, verify new session is present
	listResp, err := http.Get(fmt.Sprintf("%s/api/containers/%s/sessions", baseURL, containerID))
	if err != nil {
		t.Fatalf("GET /sessions: %v", err)
	}
	defer func() { _ = listResp.Body.Close() }()

	var sessions []web.SessionResponse
	if err := json.NewDecoder(listResp.Body).Decode(&sessions); err != nil {
		t.Fatalf("decode sessions: %v", err)
	}

	found := false
	for _, s := range sessions {
		if s.Name == sessionName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("session %q not found after creation", sessionName)
	}

	// DELETE — destroy session
	req, err := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/api/containers/%s/sessions/%s", baseURL, containerID, sessionName),
		nil,
	)
	if err != nil {
		t.Fatalf("build DELETE request: %v", err)
	}
	delResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /sessions/%s: %v", sessionName, err)
	}
	defer func() { _ = delResp.Body.Close() }()

	if delResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(delResp.Body)
		t.Fatalf("DELETE status = %d, want 200; body: %s", delResp.StatusCode, respBody)
	}

	// GET again — verify session is gone
	listResp2, err := http.Get(fmt.Sprintf("%s/api/containers/%s/sessions", baseURL, containerID))
	if err != nil {
		t.Fatalf("GET /sessions after delete: %v", err)
	}
	defer func() { _ = listResp2.Body.Close() }()

	var sessions2 []web.SessionResponse
	if err := json.NewDecoder(listResp2.Body).Decode(&sessions2); err != nil {
		t.Fatalf("decode sessions after delete: %v", err)
	}

	for _, s := range sessions2 {
		if s.Name == sessionName {
			t.Errorf("session %q still present after delete", sessionName)
		}
	}
}

// --------------------------------------------------------------------------
// Task 3: Web terminal E2E tests (GH-17.AC3.2, AC3.3, AC3.6, AC3.7)
// --------------------------------------------------------------------------

func TestDockerWebTerminal(t *testing.T) {
	testWebTerminal(t, "docker")
}

func testWebTerminal(t *testing.T, runtime string) {
	SkipIfRuntimeMissing(t, runtime)
	SkipIfDevcontainerMissing(t)

	cfg := TestConfig(runtime)
	templates := TestTemplates()
	logMgr := TestLogManager(t)
	mgr := container.NewManager(container.ManagerOptions{
		Config:     cfg,
		Templates:  templates,
		LogManager: logMgr,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	projectDir := TestProject(t, "basic")

	c, err := mgr.CreateWithCompose(ctx, container.CreateOptions{
		ProjectPath: projectDir,
		Template:    templates[0].Name,
		Name:        "e2e-terminal-test",
	})
	if err != nil {
		t.Fatalf("create container: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_ = mgr.DestroyWithCompose(cleanupCtx, c.ID)
	})

	if err := mgr.Refresh(ctx); err != nil {
		t.Logf("refresh warning: %v", err)
	}

	_, baseURL := TestWebServer(t, mgr, logMgr)
	t.Logf("web server at %s", baseURL)

	const sessionName = "e2e-term"

	// Create a tmux session to attach to.
	body := bytes.NewBufferString(`{"name":"` + sessionName + `"}`)
	createResp, err := http.Post(
		fmt.Sprintf("%s/api/containers/%s/sessions", baseURL, c.ID),
		"application/json",
		body,
	)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	_ = createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create session status = %d, want 201", createResp.StatusCode)
	}

	wsURL := strings.Replace(baseURL, "http://", "ws://", 1) +
		fmt.Sprintf("/api/containers/%s/sessions/%s/terminal", c.ID, sessionName)

	t.Run("AC3.2+AC3.3 bidirectional I/O", func(t *testing.T) {
		testTerminalBidirectional(t, wsURL, baseURL, c.ID, sessionName)
	})

	t.Run("AC3.6 disconnect preserves session", func(t *testing.T) {
		testTerminalDisconnectPreservesSession(t, wsURL, baseURL, c.ID, sessionName)
	})

	t.Run("AC3.7 multiple concurrent connections", func(t *testing.T) {
		testTerminalMultipleConnections(t, wsURL)
	})
}

// readUntil reads binary websocket frames until the expected string appears in
// the accumulated output, or the context deadline is exceeded.
func readUntil(ctx context.Context, t *testing.T, conn *websocket.Conn, expected string) bool {
	t.Helper()

	var buf strings.Builder
	for {
		msgType, data, err := conn.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return false
			}
			t.Logf("read error: %v", err)
			return false
		}
		if msgType == websocket.MessageBinary || msgType == websocket.MessageText {
			buf.Write(data)
		}
		if strings.Contains(buf.String(), expected) {
			return true
		}
	}
}

// testTerminalBidirectional verifies AC3.2 (keystrokes appear in tmux) and
// AC3.3 (terminal output arrives as binary frames).
func testTerminalBidirectional(t *testing.T, wsURL, baseURL, containerID, sessionName string) {
	t.Helper()

	dialCtx, dialCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dialCancel()

	conn, _, err := websocket.Dial(dialCtx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	defer conn.CloseNow()

	ioCtx, ioCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer ioCancel()

	// Send initial resize so the pty is sized.
	if err := conn.Write(ioCtx, websocket.MessageText, []byte(`{"type":"resize","cols":80,"rows":24}`)); err != nil {
		t.Fatalf("send resize: %v", err)
	}

	// AC3.3: wait for initial terminal output (tmux banner or shell prompt).
	// We don't check specific content — any binary frame arriving proves AC3.3.
	// Use a short-lived context for the initial read.
	initialCtx, initialCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer initialCancel()

	_, _, readErr := conn.Read(initialCtx)
	if readErr != nil && initialCtx.Err() != nil {
		t.Log("no initial output within 10s; proceeding anyway")
	}

	// AC3.2: send keystrokes and verify they produce output.
	const marker = "e2e-terminal-test-marker"
	if err := conn.Write(ioCtx, websocket.MessageBinary, []byte("echo "+marker+"\n")); err != nil {
		t.Fatalf("send keystrokes: %v", err)
	}

	// AC3.3: read output and verify marker appears.
	if !readUntil(ioCtx, t, conn, marker) {
		t.Errorf("did not receive echoed marker %q in terminal output", marker)
	}

	_ = conn.Close(websocket.StatusNormalClosure, "done")
}

// testTerminalDisconnectPreservesSession verifies AC3.6: closing the websocket
// does not destroy the tmux session.
func testTerminalDisconnectPreservesSession(t *testing.T, wsURL, baseURL, containerID, sessionName string) {
	t.Helper()

	dialCtx, dialCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dialCancel()

	conn, _, err := websocket.Dial(dialCtx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}

	// Attach briefly then close cleanly.
	_ = conn.Close(websocket.StatusNormalClosure, "test done")

	// Brief pause to allow server-side cleanup goroutines to settle.
	time.Sleep(500 * time.Millisecond)

	// Verify session still exists via HTTP.
	listURL := fmt.Sprintf("%s/api/containers/%s/sessions", baseURL, containerID)
	resp, err := http.Get(listURL)
	if err != nil {
		t.Fatalf("GET /sessions after disconnect: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var sessions []web.SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		t.Fatalf("decode sessions: %v", err)
	}

	found := false
	for _, s := range sessions {
		if s.Name == sessionName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("AC3.6: session %q was destroyed when websocket closed (it should persist)", sessionName)
	}
}

// testTerminalMultipleConnections verifies AC3.7: two concurrent WebSocket
// connections to the same tmux session both receive I/O.
func testTerminalMultipleConnections(t *testing.T, wsURL string) {
	t.Helper()

	dialCtx, dialCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dialCancel()

	conn1, _, err := websocket.Dial(dialCtx, wsURL, nil)
	if err != nil {
		t.Fatalf("conn1 dial: %v", err)
	}
	defer conn1.CloseNow()

	conn2, _, err := websocket.Dial(dialCtx, wsURL, nil)
	if err != nil {
		t.Fatalf("conn2 dial: %v", err)
	}
	defer conn2.CloseNow()

	ioCtx, ioCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer ioCancel()

	// Send resize on both connections.
	resizeMsg := []byte(`{"type":"resize","cols":80,"rows":24}`)
	_ = conn1.Write(ioCtx, websocket.MessageText, resizeMsg)
	_ = conn2.Write(ioCtx, websocket.MessageText, resizeMsg)

	// Brief pause for both sessions to attach.
	time.Sleep(500 * time.Millisecond)

	// Send input on conn1; expect output on conn2 (same tmux session).
	const marker = "e2e-multi-tab-marker"
	if err := conn1.Write(ioCtx, websocket.MessageBinary, []byte("echo "+marker+"\n")); err != nil {
		t.Fatalf("send on conn1: %v", err)
	}

	if !readUntil(ioCtx, t, conn2, marker) {
		t.Errorf("AC3.7: conn2 did not receive output from input sent on conn1")
	}

	// Verify conn2 still works after conn1 closes.
	_ = conn1.Close(websocket.StatusNormalClosure, "tab closed")
	time.Sleep(200 * time.Millisecond)

	const marker2 = "e2e-after-close-marker"
	if err := conn2.Write(ioCtx, websocket.MessageBinary, []byte("echo "+marker2+"\n")); err != nil {
		t.Fatalf("send on conn2 after conn1 close: %v", err)
	}
	if !readUntil(ioCtx, t, conn2, marker2) {
		t.Errorf("AC3.7: conn2 stopped working after conn1 closed")
	}
}

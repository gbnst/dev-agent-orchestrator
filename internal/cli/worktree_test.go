// pattern: Imperative Shell
package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"devagent/internal/instance"
)

func TestWorktreeCreate_NoInstance_ExitsCode2(t *testing.T) {
	// Setup temp dir (no port file, simulating no running instance)
	tmpDir := t.TempDir()

	exitCode := -1
	stderr := &bytes.Buffer{}

	delegate := Delegate{
		ConfigDir: tmpDir,
		ExitFunc: func(code int) {
			exitCode = code
		},
		Stderr: stderr,
	}

	// Execute a command with no instance available
	delegate.Run(func(client *instance.Client) error {
		return nil
	})

	// Should have exited with code 2 (no instance)
	if exitCode != 2 {
		t.Errorf("exit code = %d, want 2 (no instance found)", exitCode)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "no running devagent instance found") {
		t.Errorf("stderr should contain 'no running devagent instance found', got: %s", errOutput)
	}
}

func TestWorktreeCreate_Success(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/api/projects/L3BhdGg=/worktrees":
			// Expect POST with JSON body
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if req["name"] != "feature" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"name":"feature","path":"/path","container_id":"abc"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Setup temp dir with lock and port file
	tmpDir := t.TempDir()
	fl, err := instance.Lock(tmpDir)
	if err != nil {
		t.Fatalf("failed to lock: %v", err)
	}
	defer fl.Unlock()

	portFile := filepath.Join(tmpDir, "devagent.port")
	err = os.WriteFile(portFile, []byte(server.Listener.Addr().String()), 0600)
	if err != nil {
		t.Fatalf("failed to write port file: %v", err)
	}

	// Test the delegation logic
	exitCode := -1
	exitFn := func(code int) {
		exitCode = code
	}
	stderr := &bytes.Buffer{}
	stdout := &bytes.Buffer{}

	// Redirect stdout for capturing JSON output
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()
	r, w, _ := os.Pipe()
	os.Stdout = w

	delegate := Delegate{
		ConfigDir: tmpDir,
		ExitFunc:  exitFn,
		Stderr:    stderr,
	}

	delegate.Run(func(client *instance.Client) error {
		data, err := client.CreateWorktree("/path", "feature", false)
		if err != nil {
			return err
		}
		return PrintJSON(data)
	})

	w.Close()
	stdout.ReadFrom(r)
	os.Stdout = oldStdout

	// Should not have exited with error
	if exitCode != -1 && exitCode != 0 {
		t.Errorf("exit code = %d, want -1 or 0 (no error)", exitCode)
	}

	// Verify JSON output was printed (check that stdout contains container_id)
	outStr := stdout.String()
	if !strings.Contains(outStr, "container_id") {
		t.Errorf("stdout should contain JSON with container_id, got: %s", outStr)
	}
}

func TestWorktreeCreate_NoStartFlag(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/api/projects/L3BhdGg=/worktrees":
			// Expect POST with JSON body including no_start
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			// Verify no_start is true
			if noStart, ok := req["no_start"].(bool); !ok || !noStart {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"no_start should be true"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"name":"feature","path":"/path"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Setup temp dir with lock and port file
	tmpDir := t.TempDir()
	fl, err := instance.Lock(tmpDir)
	if err != nil {
		t.Fatalf("failed to lock: %v", err)
	}
	defer fl.Unlock()

	portFile := filepath.Join(tmpDir, "devagent.port")
	err = os.WriteFile(portFile, []byte(server.Listener.Addr().String()), 0600)
	if err != nil {
		t.Fatalf("failed to write port file: %v", err)
	}

	// Test the delegation logic
	exitCode := -1
	exitFn := func(code int) {
		exitCode = code
	}
	stderr := &bytes.Buffer{}

	delegate := Delegate{
		ConfigDir: tmpDir,
		ExitFunc:  exitFn,
		Stderr:    stderr,
	}

	delegate.Run(func(client *instance.Client) error {
		_, err := client.CreateWorktree("/path", "feature", true)
		return err
	})

	// Should not have exited with error
	if exitCode != -1 && exitCode != 0 {
		t.Errorf("exit code = %d, want -1 or 0 (no error)", exitCode)
	}

	if stderr.Len() > 0 {
		t.Errorf("stderr should be empty on success, got: %s", stderr.String())
	}
}

func TestWorktreeCreate_ServerError(t *testing.T) {
	// Create a test HTTP server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/api/projects/L3BhdGg=/worktrees":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid worktree name"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Setup temp dir with lock and port file
	tmpDir := t.TempDir()
	fl, err := instance.Lock(tmpDir)
	if err != nil {
		t.Fatalf("failed to lock: %v", err)
	}
	defer fl.Unlock()

	portFile := filepath.Join(tmpDir, "devagent.port")
	err = os.WriteFile(portFile, []byte(server.Listener.Addr().String()), 0600)
	if err != nil {
		t.Fatalf("failed to write port file: %v", err)
	}

	// Test the delegation logic
	exitCode := -1
	exitFn := func(code int) {
		exitCode = code
	}
	stderr := &bytes.Buffer{}

	delegate := Delegate{
		ConfigDir: tmpDir,
		ExitFunc:  exitFn,
		Stderr:    stderr,
	}

	delegate.Run(func(client *instance.Client) error {
		_, err := client.CreateWorktree("/path", "invalid@name", false)
		return err
	})

	// Should have exited with code 1
	if exitCode != 1 {
		t.Errorf("exit code = %d, want 1", exitCode)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "invalid worktree name") {
		t.Errorf("stderr should contain error message, got: %s", errOutput)
	}
}

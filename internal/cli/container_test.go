// pattern: Imperative Shell
package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"devagent/internal/instance"
)

// TestContainerCommands verifies that container commands delegate correctly.
// Note: These are integration-style tests that test the delegation logic
// through the Delegate struct and client methods, not through app.Execute
// (which would call os.Exit and terminate tests).

func TestContainerStart_DelegateSuccess(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/api/containers/test-id/start":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"started"}`))
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
		_, err := client.StartContainer("test-id")
		return err
	})

	// Should not have exited
	if exitCode != -1 {
		t.Errorf("exit code = %d, want -1 (no exit)", exitCode)
	}
}

func TestContainerStart_DelegateError(t *testing.T) {
	// Create a test HTTP server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/api/containers/test-id/start":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"container is already running"}`))
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
		_, err := client.StartContainer("test-id")
		return err
	})

	// Should have exited with code 1
	if exitCode != 1 {
		t.Errorf("exit code = %d, want 1", exitCode)
	}

	errOutput := stderr.String()
	if !bytes.Contains([]byte(errOutput), []byte("already running")) {
		t.Errorf("stderr should contain error message, got: %s", errOutput)
	}
}

func TestContainerStop_DelegateSuccess(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/api/containers/test-id/stop":
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"stopped"}`))
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
		_, err := client.StopContainer("test-id")
		return err
	})

	// Should not have exited
	if exitCode != -1 {
		t.Errorf("exit code = %d, want -1 (no exit)", exitCode)
	}
}

func TestContainerDestroy_DelegateSuccess(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/api/containers/test-id":
			if r.Method != http.MethodDelete {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"destroyed"}`))
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
		_, err := client.DestroyContainer("test-id")
		return err
	})

	// Should not have exited
	if exitCode != -1 {
		t.Errorf("exit code = %d, want -1 (no exit)", exitCode)
	}
}

func TestContainerStart_MissingArg_ReturnsError(t *testing.T) {
	// Create a minimal test setup
	tmpDir := t.TempDir()

	// Create a command via BuildApp and get the container start command
	app := BuildApp("1.0.0", tmpDir)
	containerGroup := app.groups["container"]
	if containerGroup == nil {
		t.Fatal("container group not found")
	}

	// Find start command
	var startCmd *Command
	for _, cmd := range containerGroup.Commands {
		if cmd.Name == "start" {
			startCmd = cmd
			break
		}
	}
	if startCmd == nil {
		t.Fatal("start command not found")
	}

	// Call with no args - should return an error
	err := startCmd.Run([]string{})
	if err == nil {
		t.Error("expected error when calling container start with no args, got nil")
	}
	if err != nil && err.Error() != "usage: devagent container start <id-or-name>" {
		t.Errorf("expected usage error, got: %v", err)
	}
}

func TestContainerStop_MissingArg_ReturnsError(t *testing.T) {
	// Create a minimal test setup
	tmpDir := t.TempDir()

	// Create a command via BuildApp and get the container stop command
	app := BuildApp("1.0.0", tmpDir)
	containerGroup := app.groups["container"]
	if containerGroup == nil {
		t.Fatal("container group not found")
	}

	// Find stop command
	var stopCmd *Command
	for _, cmd := range containerGroup.Commands {
		if cmd.Name == "stop" {
			stopCmd = cmd
			break
		}
	}
	if stopCmd == nil {
		t.Fatal("stop command not found")
	}

	// Call with no args - should return an error
	err := stopCmd.Run([]string{})
	if err == nil {
		t.Error("expected error when calling container stop with no args, got nil")
	}
	if err != nil && err.Error() != "usage: devagent container stop <id-or-name>" {
		t.Errorf("expected usage error, got: %v", err)
	}
}

func TestContainerDestroy_MissingArg_ReturnsError(t *testing.T) {
	// Create a minimal test setup
	tmpDir := t.TempDir()

	// Create a command via BuildApp and get the container destroy command
	app := BuildApp("1.0.0", tmpDir)
	containerGroup := app.groups["container"]
	if containerGroup == nil {
		t.Fatal("container group not found")
	}

	// Find destroy command
	var destroyCmd *Command
	for _, cmd := range containerGroup.Commands {
		if cmd.Name == "destroy" {
			destroyCmd = cmd
			break
		}
	}
	if destroyCmd == nil {
		t.Fatal("destroy command not found")
	}

	// Call with no args - should return an error
	err := destroyCmd.Run([]string{})
	if err == nil {
		t.Error("expected error when calling container destroy with no args, got nil")
	}
	if err != nil && err.Error() != "usage: devagent container destroy <id-or-name>" {
		t.Errorf("expected usage error, got: %v", err)
	}
}

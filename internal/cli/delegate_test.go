// pattern: Imperative Shell
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"devagent/internal/instance"
)

func TestDelegate_Run_NoInstance_ExitsCode2(t *testing.T) {
	// Create a Delegate with a temp config dir (no port file)
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

	delegate.Run(func(client *instance.Client) error {
		return fmt.Errorf("should not be called")
	})

	if exitCode != 2 {
		t.Errorf("exit code = %d, want 2", exitCode)
	}

	errOutput := stderr.String()
	if !bytes.Contains([]byte(errOutput), []byte("no running devagent instance found")) {
		t.Errorf("stderr should contain 'no running devagent instance found', got: %s", errOutput)
	}
}

func TestDelegate_Run_Success(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/api/projects":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"projects":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create a temp dir and write port file
	tmpDir := t.TempDir()
	// First lock the dataDir so discover thinks there's an instance
	fl, err := instance.Lock(tmpDir)
	if err != nil {
		t.Fatalf("failed to lock: %v", err)
	}
	defer fl.Unlock()

	// Extract port from server URL and write port file
	portFile := filepath.Join(tmpDir, "devagent.port")
	portData := server.Listener.Addr().String()
	err = os.WriteFile(portFile, []byte(portData), 0600)
	if err != nil {
		t.Fatalf("failed to write port file: %v", err)
	}

	exitCode := -1
	stderr := &bytes.Buffer{}
	clientCalled := false

	delegate := Delegate{
		ConfigDir: tmpDir,
		ExitFunc: func(code int) {
			exitCode = code
		},
		Stderr: stderr,
	}

	delegate.Run(func(client *instance.Client) error {
		clientCalled = true
		_, err := client.List()
		return err
	})

	if !clientCalled {
		t.Errorf("client function was not called")
	}

	if exitCode != -1 && exitCode != 0 {
		t.Errorf("exit code = %d, want no exit call or 0", exitCode)
	}

	if stderr.Len() > 0 {
		t.Errorf("stderr should be empty on success, got: %s", stderr.String())
	}
}

func TestDelegate_Run_ClientError_ExitsCode1(t *testing.T) {
	// Create a test HTTP server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/api/containers/nonexistent/start":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"something went wrong"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create a temp dir and write port file
	tmpDir := t.TempDir()
	fl, err := instance.Lock(tmpDir)
	if err != nil {
		t.Fatalf("failed to lock: %v", err)
	}
	defer fl.Unlock()

	portFile := filepath.Join(tmpDir, "devagent.port")
	portData := server.Listener.Addr().String()
	err = os.WriteFile(portFile, []byte(portData), 0600)
	if err != nil {
		t.Fatalf("failed to write port file: %v", err)
	}

	exitCode := -1
	stderr := &bytes.Buffer{}

	delegate := Delegate{
		ConfigDir: tmpDir,
		ExitFunc: func(code int) {
			exitCode = code
		},
		Stderr: stderr,
	}

	delegate.Run(func(client *instance.Client) error {
		// Call a method that will fail
		_, err := client.StartContainer("nonexistent")
		return err
	})

	if exitCode != 1 {
		t.Errorf("exit code = %d, want 1", exitCode)
	}

	errOutput := stderr.String()
	if !bytes.Contains([]byte(errOutput), []byte("something went wrong")) {
		t.Errorf("stderr should contain error message, got: %s", errOutput)
	}
}

func TestPrintJSON_Terminal_PrettyPrints(t *testing.T) {
	// This test is tricky because we need to mock os.Stdout.Stat()
	// For now, just test that PrintJSON handles valid JSON
	data := []byte(`{"key":"value","number":42}`)

	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()

	r, w, _ := os.Pipe()
	os.Stdout = w

	err := PrintJSON(data)

	w.Close()
	buf := &bytes.Buffer{}
	buf.ReadFrom(r)
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("PrintJSON returned error: %v", err)
	}

	output := buf.String()
	// Check that output contains the expected keys
	var parsed map[string]any
	err = json.Unmarshal([]byte(output), &parsed)
	if err != nil {
		t.Errorf("PrintJSON output is not valid JSON: %v\nOutput: %s", err, output)
	}

	if parsed["key"] != "value" || parsed["number"] != float64(42) {
		t.Errorf("PrintJSON output has wrong content: %v", parsed)
	}
}

func TestPrintJSON_InvalidJSON_WritesRaw(t *testing.T) {
	data := []byte(`not json`)

	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()

	r, w, _ := os.Pipe()
	os.Stdout = w

	err := PrintJSON(data)

	w.Close()
	buf := &bytes.Buffer{}
	buf.ReadFrom(r)
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("PrintJSON returned error: %v", err)
	}

	output := buf.String()
	if output != "not json" {
		t.Errorf("PrintJSON output = %q, want %q", output, "not json")
	}
}

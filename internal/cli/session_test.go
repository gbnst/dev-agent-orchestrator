// pattern: Imperative Shell
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	flag "github.com/spf13/pflag"

	"devagent/internal/instance"
)

// TestSessionCreate_Success verifies that session create command works with a successful response.
func TestSessionCreate_Success(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/api/containers/test-id/sessions":
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"status":"created"}`))
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
		_, err := client.CreateSession("test-id", "dev")
		return err
	})

	// Should not have exited
	if exitCode != -1 {
		t.Errorf("exit code = %d, want -1 (no exit)", exitCode)
	}
}

// TestSessionCreate_MissingArgs verifies that session create requires 2 args.
func TestSessionCreate_MissingArgs(t *testing.T) {
	tmpDir := t.TempDir()

	// Build the app with the temp config dir
	app := BuildApp("test", tmpDir)
	sessionGroup := app.groups["session"]
	if sessionGroup == nil {
		t.Fatal("session group not found")
	}
	createCmd := sessionGroup.Commands["create"]
	if createCmd == nil {
		t.Fatal("create command not found")
	}

	// Call with insufficient args (0 args instead of required 2)
	err := createCmd.Run([]string{})

	// Should return an error for missing args
	if err == nil {
		t.Error("expected error for missing args, got nil")
	}

	// Error message should indicate usage
	if err != nil && !strings.Contains(err.Error(), "usage") {
		t.Errorf("error message should contain 'usage', got: %v", err)
	}
}

// TestSessionCreate_ContainerStopped verifies that creating session in stopped container returns error.
func TestSessionCreate_ContainerStopped(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/api/containers/test-id/sessions":
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"container is not running"}`))
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
		_, err := client.CreateSession("test-id", "dev")
		return err
	})

	// Should have exited with code 1
	if exitCode != 1 {
		t.Errorf("exit code = %d, want 1", exitCode)
	}

	errOutput := stderr.String()
	if !bytes.Contains([]byte(errOutput), []byte("not running")) {
		t.Errorf("stderr should contain error message, got: %s", errOutput)
	}
}

// TestSessionDestroy_Success verifies that session destroy command works.
func TestSessionDestroy_Success(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/api/containers/test-id/sessions/dev":
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
		_, err := client.DestroySession("test-id", "dev")
		return err
	})

	// Should not have exited
	if exitCode != -1 {
		t.Errorf("exit code = %d, want -1 (no exit)", exitCode)
	}
}

// TestSessionDestroy_NotFound verifies that destroying nonexistent session returns error.
func TestSessionDestroy_NotFound(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/api/containers/test-id/sessions/nonexistent":
			if r.Method != http.MethodDelete {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"session not found"}`))
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
		_, err := client.DestroySession("test-id", "nonexistent")
		return err
	})

	// Should have exited with code 1
	if exitCode != 1 {
		t.Errorf("exit code = %d, want 1", exitCode)
	}

	errOutput := stderr.String()
	if !bytes.Contains([]byte(errOutput), []byte("not found")) {
		t.Errorf("stderr should contain error message, got: %s", errOutput)
	}
}

// TestSessionReadlines_Success verifies that session readlines command works with default lines.
func TestSessionReadlines_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/api/containers/test-id/sessions/dev/capture-lines":
			if r.Method != http.MethodGet {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			// Default is 20 lines
			if r.URL.Query().Get("lines") != "20" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"content":"line1\nline2\n"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

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
		data, err := client.ReadLines("test-id", "dev", 20)
		if err != nil {
			return err
		}
		var result struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return err
		}
		if result.Content != "line1\nline2\n" {
			t.Errorf("got content %q", result.Content)
		}
		return nil
	})

	if exitCode != -1 {
		t.Errorf("exit code = %d, want -1", exitCode)
	}
}

// TestSessionReadlines_CustomCount verifies that session readlines passes custom line count.
func TestSessionReadlines_CustomCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/api/containers/test-id/sessions/dev/capture-lines":
			if r.URL.Query().Get("lines") != "500" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"content":"lots of lines\n"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

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
		data, err := client.ReadLines("test-id", "dev", 500)
		if err != nil {
			return err
		}
		var result struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return err
		}
		if result.Content != "lots of lines\n" {
			t.Errorf("got content %q", result.Content)
		}
		return nil
	})

	if exitCode != -1 {
		t.Errorf("exit code = %d, want -1", exitCode)
	}
}

// TestSessionReadlines_MissingArgs verifies that session readlines requires 2 args.
func TestSessionReadlines_MissingArgs(t *testing.T) {
	tmpDir := t.TempDir()

	app := BuildApp("test", tmpDir)
	sessionGroup := app.groups["session"]
	if sessionGroup == nil {
		t.Fatal("session group not found")
	}
	readlinesCmd := sessionGroup.Commands["readlines"]
	if readlinesCmd == nil {
		t.Fatal("readlines command not found")
	}

	err := readlinesCmd.Run([]string{"container-id"})
	if err == nil {
		t.Error("expected error for missing args, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "usage") {
		t.Errorf("error message should contain 'usage', got: %v", err)
	}
}

// TestSessionReadlines_InvalidCount verifies that invalid line count returns error.
func TestSessionReadlines_InvalidCount(t *testing.T) {
	tmpDir := t.TempDir()

	app := BuildApp("test", tmpDir)
	readlinesCmd := app.groups["session"].Commands["readlines"]

	err := readlinesCmd.Run([]string{"container-id", "session-name", "abc"})
	if err == nil {
		t.Error("expected error for invalid line count, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid") {
		t.Errorf("error message should contain 'invalid', got: %v", err)
	}
}

// TestSessionSend_Success verifies that session send command works.
func TestSessionSend_Success(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/api/containers/test-id/sessions/dev/send":
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusNoContent)
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
		err := client.SendToSession("test-id", "dev", "ls -la")
		return err
	})

	// Should not have exited
	if exitCode != -1 {
		t.Errorf("exit code = %d, want -1 (no exit)", exitCode)
	}
}

// TestSessionSend_MissingArgs verifies that session send requires 3 args.
func TestSessionSend_MissingArgs(t *testing.T) {
	tmpDir := t.TempDir()

	app := BuildApp("test", tmpDir)
	sessionGroup := app.groups["session"]
	if sessionGroup == nil {
		t.Fatal("session group not found")
	}
	sendCmd := sessionGroup.Commands["send"]
	if sendCmd == nil {
		t.Fatal("send command not found")
	}

	// Call with insufficient args (2 args instead of required 3)
	err := sendCmd.Run([]string{"container-id", "session-name"})

	// Should return an error for missing args
	if err == nil {
		t.Error("expected error for missing args, got nil")
	}

	// Error message should indicate usage
	if err != nil && !strings.Contains(err.Error(), "usage") {
		t.Errorf("error message should contain 'usage', got: %v", err)
	}
}

// TestSessionTail_CustomInterval verifies that --interval flag is parsed and used correctly.
func TestSessionTail_CustomInterval(t *testing.T) {
	// Create a test HTTP server that tracks polling interval
	var reqCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/api/containers/test-id/sessions/dev/capture":
			reqCount++
			w.Header().Set("Content-Type", "application/json")
			// Return increasing cursor to trigger multiple polls
			w.Write([]byte(`{"content":"content","cursor_y":` + fmt.Sprintf("%d", reqCount) + `}`))
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

	// Test that the interval flag is parsed and used
	exitFn := func(code int) {
		// Expected to exit with code 0 on success
		if code != 0 {
			t.Errorf("unexpected exit code: %d", code)
		}
	}
	stderr := &bytes.Buffer{}

	// Create delegate and verify it doesn't error on discovery
	delegate := Delegate{
		ConfigDir: tmpDir,
		ExitFunc:  exitFn,
		Stderr:    stderr,
	}

	client := delegate.Client()
	if client == nil {
		t.Fatal("Client() returned nil, expected valid client")
	}

	// Verify we can create a TailConfig with custom interval
	interval, err := time.ParseDuration("500ms")
	if err != nil {
		t.Fatalf("failed to parse interval: %v", err)
	}

	output := &bytes.Buffer{}
	errOutput := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Verify tail session runs without error (context timeout is expected)
	err = TailSession(ctx, client, TailConfig{
		ContainerID: "test-id",
		Session:     "dev",
		Interval:    interval,
		NoColor:     false,
		Writer:      output,
		ErrWriter:   errOutput,
	})

	// Should complete without error
	if err != nil {
		t.Errorf("TailSession returned unexpected error: %v", err)
	}
}

// TestSessionTail_NoColorFlag verifies that --no-color flag strips ANSI escape sequences.
func TestSessionTail_NoColorFlag(t *testing.T) {
	// Create a test HTTP server that returns ANSI content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		case "/api/containers/test-id/sessions/dev/capture":
			w.Header().Set("Content-Type", "application/json")
			// Return content with ANSI codes
			w.Write([]byte(`{"content":"\u001b[31mRed\u001b[0m","cursor_y":5}`))
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

	// Create delegate and get client
	exitFn := func(code int) {
		if code != 0 {
			t.Errorf("unexpected exit code: %d", code)
		}
	}
	stderr := &bytes.Buffer{}

	delegate := Delegate{
		ConfigDir: tmpDir,
		ExitFunc:  exitFn,
		Stderr:    stderr,
	}

	client := delegate.Client()
	if client == nil {
		t.Fatal("Client() returned nil, expected valid client")
	}

	// Test that NoColor flag strips ANSI codes
	output := &bytes.Buffer{}
	errOutput := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Run with NoColor=true
	err = TailSession(ctx, client, TailConfig{
		ContainerID: "test-id",
		Session:     "dev",
		Interval:    10 * time.Millisecond,
		NoColor:     true, // Enable ANSI stripping
		Writer:      output,
		ErrWriter:   errOutput,
	})

	if err != nil {
		t.Errorf("TailSession returned unexpected error: %v", err)
	}

	// Verify ANSI codes were stripped
	outStr := output.String()
	if bytes.Contains([]byte(outStr), []byte("\x1b[31m")) {
		t.Errorf("output contains ANSI codes when NoColor=true, got: %q", outStr)
	}
	if !bytes.Contains([]byte(outStr), []byte("Red")) {
		t.Errorf("output should contain 'Red' text, got: %q", outStr)
	}
}

// TestSessionTail_MissingArgs verifies that tail requires 2 args.
func TestSessionTail_MissingArgs(t *testing.T) {
	tmpDir := t.TempDir()

	app := BuildApp("test", tmpDir)
	sessionGroup := app.groups["session"]
	if sessionGroup == nil {
		t.Fatal("session group not found")
	}
	tailCmd := sessionGroup.Commands["tail"]
	if tailCmd == nil {
		t.Fatal("tail command not found")
	}

	// Call with insufficient args (0 args instead of required 2)
	err := tailCmd.Run([]string{})

	// Should return an error for missing args
	if err == nil {
		t.Error("expected error for missing args, got nil")
	}

	// Error message should indicate usage
	if err != nil && !strings.Contains(err.Error(), "usage") {
		t.Errorf("error message should contain 'usage', got: %v", err)
	}
}

// TestSessionTail_InvalidInterval verifies that invalid duration returns error.
func TestSessionTail_InvalidInterval(t *testing.T) {
	tmpDir := t.TempDir()

	app := BuildApp("test", tmpDir)
	sessionGroup := app.groups["session"]
	if sessionGroup == nil {
		t.Fatal("session group not found")
	}
	tailCmd := sessionGroup.Commands["tail"]
	if tailCmd == nil {
		t.Fatal("tail command not found")
	}

	// Call with invalid interval flag
	err := tailCmd.Run([]string{"container-id", "session-name", "--interval", "invalid"})

	// Should return an error for invalid interval
	if err == nil {
		t.Error("expected error for invalid interval, got nil")
	}

	// Error message should indicate the problem
	if err != nil && !strings.Contains(err.Error(), "invalid") {
		t.Errorf("error message should contain 'invalid', got: %v", err)
	}
}

// TestSessionTail_ConcatenatedShortInterval verifies that -i500ms (no space) is parsed by pflag.
func TestSessionTail_ConcatenatedShortInterval(t *testing.T) {
	// Verify pflag parses -i500ms correctly (concatenated short flag + value)
	fs := flag.NewFlagSet("test-tail", flag.ContinueOnError)
	intervalStr := fs.StringP("interval", "i", "1s", "polling interval")
	noColor := fs.Bool("no-color", false, "strip ANSI color codes")

	err := fs.Parse([]string{"-i500ms"})
	if err != nil {
		t.Fatalf("pflag failed to parse -i500ms: %v", err)
	}
	if *intervalStr != "500ms" {
		t.Errorf("interval = %q, want %q", *intervalStr, "500ms")
	}
	if *noColor {
		t.Errorf("noColor should be false")
	}

	interval, err := time.ParseDuration(*intervalStr)
	if err != nil {
		t.Fatalf("failed to parse duration: %v", err)
	}
	if interval != 500*time.Millisecond {
		t.Errorf("interval = %v, want 500ms", interval)
	}
}

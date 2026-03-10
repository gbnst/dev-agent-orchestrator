// pattern: Imperative Shell
package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestBuildApp_VersionCommand_PrintsVersion(t *testing.T) {
	app := BuildApp("1.2.3", "")

	// Find the version command
	versionCmd, ok := app.commands["version"]
	if !ok {
		t.Fatal("version command not registered")
	}

	// Capture stdout
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()

	r, w, _ := os.Pipe()
	os.Stdout = w

	err := versionCmd.Run(nil)

	w.Close()
	buf := &bytes.Buffer{}
	buf.ReadFrom(r)
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("version command returned error: %v", err)
	}

	output := buf.String()
	if output != "1.2.3\n" {
		t.Errorf("version command output = %q, want \"1.2.3\\n\"", output)
	}
}

func TestBuildApp_NoArgs_ReturnsTrueForTUI(t *testing.T) {
	app := BuildApp("1.0.0", "")
	result := app.Execute(nil)
	if !result {
		t.Errorf("Execute(nil) returned %v, want true", result)
	}
}

func TestBuildApp_ContainerStart_RequiresArg(t *testing.T) {
	// Build app
	app := BuildApp("1.0.0", "")

	// The container group and start command should be registered
	// Just verify the basic structure to ensure no panics
	if _, ok := app.groups["container"]; !ok {
		t.Fatal("container group not registered")
	}

	// Test the behavior of the start command with no args
	// This is done in container_test.go with integration tests
}

func TestBuildApp_ListCommand_DelegatesToInstance(t *testing.T) {
	// Create a test HTTP server that responds to /api/projects and /api/health
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/projects":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"projects":[]}`))
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create a mock discoverer that returns the test server's URL
	mockDiscoverer := func(dataDir string) (string, error) {
		return server.URL, nil
	}

	// Capture stdout to verify the list command writes JSON
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()

	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runListCommandWithDiscovery("", mockDiscoverer)

	w.Close()
	buf := &bytes.Buffer{}
	buf.ReadFrom(r)
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("list command returned error: %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte(`"projects"`)) {
		t.Errorf("expected JSON output with 'projects' key, got: %s", output)
	}
}

func TestBuildApp_CleanupCommand_Registered(t *testing.T) {
	// Create a temporary directory for the config
	tmpDir := t.TempDir()
	app := BuildApp("1.0.0", tmpDir)

	// Find the cleanup command
	cleanupCmd, ok := app.commands["cleanup"]
	if !ok {
		t.Fatal("cleanup command not registered")
	}

	// Verify the command has the expected properties
	if cleanupCmd.Name != "cleanup" {
		t.Errorf("cleanup command name = %q, want \"cleanup\"", cleanupCmd.Name)
	}

	if cleanupCmd.Summary == "" {
		t.Error("cleanup command should have a summary")
	}

	if cleanupCmd.Usage == "" {
		t.Error("cleanup command should have usage documentation")
	}

	// Verify the command can be called
	// Capture stdout to verify the command runs
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	r, w, _ := os.Pipe()
	os.Stdout = w

	// Call cleanup with no instance running (in the temp directory)
	err := cleanupCmd.Run([]string{})

	w.Close()
	buf := &bytes.Buffer{}
	buf.ReadFrom(r)
	os.Stdout = oldStdout

	// The cleanup command should succeed when no instance is running
	if err != nil {
		t.Errorf("cleanup command returned error: %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("Cleaned up")) {
		t.Errorf("expected cleanup message in output, got: %s", output)
	}
}

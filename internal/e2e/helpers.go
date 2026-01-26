//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/config"
	"devagent/internal/tui"
)

// SkipIfRuntimeMissing skips the test if the specified runtime is not available.
func SkipIfRuntimeMissing(t *testing.T, runtime string) {
	t.Helper()
	if _, err := exec.LookPath(runtime); err != nil {
		t.Skipf("Skipping test: %s not found in PATH", runtime)
	}
}

// SkipIfDevcontainerMissing skips the test if devcontainer CLI is not available.
func SkipIfDevcontainerMissing(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("devcontainer"); err != nil {
		t.Skip("Skipping test: devcontainer CLI not found in PATH")
	}
}

// TestProject creates a temporary project directory with devcontainer config.
func TestProject(t *testing.T, templateName string) string {
	t.Helper()

	projectDir := t.TempDir()
	devcontainerDir := filepath.Join(projectDir, ".devcontainer")
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatalf("Failed to create .devcontainer dir: %v", err)
	}

	return projectDir
}

// TestConfig creates a test config with the specified runtime.
func TestConfig(runtime string) *config.Config {
	return &config.Config{
		Theme:   "mocha",
		Runtime: runtime,
		BaseImages: map[string]string{
			"default": "mcr.microsoft.com/devcontainers/base:ubuntu",
		},
	}
}

// TestTemplates returns minimal templates for E2E testing.
func TestTemplates() []config.Template {
	return []config.Template{
		{
			Name:        "default",
			Description: "Default container",
			BaseImage:   "default",
		},
	}
}

// CleanupContainer removes a container after test.
func CleanupContainer(t *testing.T, runtime, containerID string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Force remove the container
	cmd := exec.CommandContext(ctx, runtime, "rm", "-f", containerID)
	if err := cmd.Run(); err != nil {
		t.Logf("Warning: failed to cleanup container %s: %v", containerID, err)
	}
}

// TUITestRunner helps drive the TUI through Update() calls for testing.
type TUITestRunner struct {
	t     *testing.T
	model tui.Model
}

// NewTUITestRunner creates a new test runner with the given model.
func NewTUITestRunner(t *testing.T, model tui.Model) *TUITestRunner {
	return &TUITestRunner{
		t:     t,
		model: model,
	}
}

// Model returns the current model state.
func (r *TUITestRunner) Model() tui.Model {
	return r.model
}

// Init runs the Init command and processes results.
func (r *TUITestRunner) Init() {
	r.t.Helper()
	cmd := r.model.Init()
	r.runCmd(cmd)
}

// PressKey simulates pressing a regular key.
func (r *TUITestRunner) PressKey(key rune) {
	r.t.Helper()
	msg := tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{key},
	}
	model, cmd := r.model.Update(msg)
	r.model = model.(tui.Model)
	r.runCmd(cmd)
}

// PressSpecialKey simulates pressing a special key like Enter or Tab.
func (r *TUITestRunner) PressSpecialKey(keyType tea.KeyType) {
	r.t.Helper()
	msg := tea.KeyMsg{Type: keyType}
	model, cmd := r.model.Update(msg)
	r.model = model.(tui.Model)
	r.runCmd(cmd)
}

// TypeText types a string character by character.
func (r *TUITestRunner) TypeText(text string) {
	r.t.Helper()
	for _, ch := range text {
		r.PressKey(ch)
	}
}

// SendWindowSize sends a window size message.
func (r *TUITestRunner) SendWindowSize(width, height int) {
	r.t.Helper()
	msg := tea.WindowSizeMsg{Width: width, Height: height}
	model, cmd := r.model.Update(msg)
	r.model = model.(tui.Model)
	r.runCmd(cmd)
}

// WaitForContainerCount waits for the container count to reach the expected value.
func (r *TUITestRunner) WaitForContainerCount(expected int, timeout time.Duration) bool {
	r.t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if r.model.ContainerCount() == expected {
			return true
		}
		// Refresh
		r.PressKey('r')
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

// runCmd executes a Bubbletea command and processes its result.
func (r *TUITestRunner) runCmd(cmd tea.Cmd) {
	r.runCmdWithDepth(cmd, 0)
}

// runCmdWithDepth executes a command with depth tracking to prevent infinite recursion.
func (r *TUITestRunner) runCmdWithDepth(cmd tea.Cmd, depth int) {
	if cmd == nil || depth > 10 {
		return
	}

	// Execute the command
	msg := cmd()
	if msg == nil {
		return
	}

	// Handle batch messages (result of tea.Batch)
	if batchMsg, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batchMsg {
			if c != nil {
				r.runCmdWithDepth(c, depth+1)
			}
		}
		return
	}

	// Skip quit messages
	if _, ok := msg.(tea.QuitMsg); ok {
		return
	}

	// Update the model with the result
	model, nextCmd := r.model.Update(msg)
	r.model = model.(tui.Model)

	// Process any follow-up commands
	r.runCmdWithDepth(nextCmd, depth+1)
}

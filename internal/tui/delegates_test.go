package tui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"

	"devagent/internal/container"
)

func TestContainerDelegate_ShowsSpinnerForPending(t *testing.T) {
	styles := NewStyles("mocha")
	delegate := newContainerDelegate(styles)

	// Set up pending operation
	containerID := "abc123def456"
	pendingOps := map[string]string{containerID: "start"}
	delegate = delegate.WithSpinnerState("⠋", pendingOps)

	// Create a container item
	ctr := &container.Container{
		ID:    containerID,
		Name:  "test-container",
		State: container.StateStopped,
	}
	item := containerItem{container: ctr}

	// Create a minimal list model for testing
	items := []list.Item{item}
	l := list.New(items, delegate, 80, 10)

	// Render the item
	var buf bytes.Buffer
	delegate.Render(&buf, l, 0, item)

	output := buf.String()

	// Should contain spinner frame instead of bullet
	if !strings.Contains(output, "⠋") {
		t.Errorf("output should contain spinner frame, got: %q", output)
	}
	if strings.Contains(output, "●") {
		t.Error("output should not contain bullet when pending")
	}
}

func TestContainerDelegate_ShowsBulletWhenNotPending(t *testing.T) {
	styles := NewStyles("mocha")
	delegate := newContainerDelegate(styles)

	// No pending operations
	delegate = delegate.WithSpinnerState("⠋", map[string]string{})

	// Create a container item
	ctr := &container.Container{
		ID:    "abc123def456",
		Name:  "test-container",
		State: container.StateRunning,
	}
	item := containerItem{container: ctr}

	items := []list.Item{item}
	l := list.New(items, delegate, 80, 10)

	var buf bytes.Buffer
	delegate.Render(&buf, l, 0, item)

	output := buf.String()

	// Should contain bullet, not spinner
	if !strings.Contains(output, "●") {
		t.Errorf("output should contain bullet, got: %q", output)
	}
}

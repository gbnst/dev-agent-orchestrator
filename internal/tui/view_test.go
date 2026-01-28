package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"devagent/internal/logging"
)

// Sessions tab content tests removed - sessions now shown in tree view
// See tree_test.go for tree rendering tests

// Tab and session tab tests removed - tabs replaced by tree view
// See tree_test.go for TestRenderDetailPanel_Session tests

func TestRenderStatusBar_Info(t *testing.T) {
	m := newTestModel(t)
	m.statusLevel = StatusInfo
	m.statusMessage = "Ready"

	result := m.renderStatusBar(80)

	if !strings.Contains(result, "Ready") {
		t.Error("status bar should contain message")
	}
}

func TestRenderStatusBar_Success(t *testing.T) {
	m := newTestModel(t)
	m.statusLevel = StatusSuccess
	m.statusMessage = "Container started"

	result := m.renderStatusBar(80)

	if !strings.Contains(result, "✓") {
		t.Error("success status should contain checkmark")
	}
	if !strings.Contains(result, "Container started") {
		t.Error("status bar should contain message")
	}
}

func TestRenderStatusBar_Error(t *testing.T) {
	m := newTestModel(t)
	m.statusLevel = StatusError
	m.statusMessage = "Failed to start"
	m.err = fmt.Errorf("connection refused")

	result := m.renderStatusBar(80)

	if !strings.Contains(result, "✗") {
		t.Error("error status should contain X mark")
	}
	if !strings.Contains(result, "Failed to start") {
		t.Error("status bar should contain message")
	}
	if !strings.Contains(result, "esc to clear") {
		t.Error("error status should contain '(esc to clear)' hint")
	}
}

func TestRenderStatusBar_Loading(t *testing.T) {
	m := newTestModel(t)
	m.statusLevel = StatusLoading
	m.statusMessage = "Starting container..."

	result := m.renderStatusBar(80)

	// Spinner renders differently each frame, just check message
	if !strings.Contains(result, "Starting container...") {
		t.Error("status bar should contain loading message")
	}
}

func TestRenderLogEntry(t *testing.T) {
	m := newTestModel(t)

	entry := logging.LogEntry{
		Timestamp: time.Date(2025, 1, 27, 10, 30, 0, 0, time.UTC),
		Level:     "INFO",
		Scope:     "container.abc123",
		Message:   "container started",
	}

	result := m.renderLogEntry(entry)

	if !strings.Contains(result, "10:30:00") {
		t.Error("should contain timestamp")
	}
	if !strings.Contains(result, "INFO") {
		t.Error("should contain level")
	}
	if !strings.Contains(result, "container.abc123") {
		t.Error("should contain scope")
	}
	if !strings.Contains(result, "container started") {
		t.Error("should contain message")
	}
}

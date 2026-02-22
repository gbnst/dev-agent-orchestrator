package tui

import (
	"strings"
	"testing"

	"devagent/internal/config"
	"devagent/internal/logging"

	tea "github.com/charmbracelet/bubbletea"
)

func TestIntegration_LogPanelToggle(t *testing.T) {
	cfg := &config.Config{
		Theme: "mocha",
	}

	tmpDir := t.TempDir()
	logPath := tmpDir + "/test-integration-log-panel.log"
	lm, err := logging.NewManager(logging.Config{
		FilePath:       logPath,
		MaxSizeMB:      1,
		MaxBackups:     1,
		MaxAgeDays:     1,
		ChannelBufSize: 100,
		Level:          "debug",
	})
	if err != nil {
		t.Fatalf("failed to create LogManager: %v", err)
	}
	defer func() { _ = lm.Close() }()

	model := NewModelWithTemplates(cfg, []config.Template{}, lm)

	// Set window size
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model = updated.(Model)

	// Initially log panel should be hidden
	view := model.View()
	if strings.Contains(view, "Logs") {
		t.Error("log panel should be hidden initially")
	}

	// Press L to show log panel
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	model = updated.(Model)

	view = model.View()
	if !strings.Contains(view, "Logs") {
		t.Error("log panel should be visible after pressing L")
	}

	// Press L again to hide
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	model = updated.(Model)

	view = model.View()
	if strings.Contains(view, "Logs") {
		t.Error("log panel should be hidden after pressing L again")
	}
}

func TestIntegration_TreeNavigation(t *testing.T) {
	cfg := &config.Config{
		Theme:   "mocha",
		Runtime: "docker",
	}

	tmpDir := t.TempDir()
	logPath := tmpDir + "/test-integration-tree-nav.log"
	lm, err := logging.NewManager(logging.Config{
		FilePath:       logPath,
		MaxSizeMB:      1,
		MaxBackups:     1,
		MaxAgeDays:     1,
		ChannelBufSize: 100,
		Level:          "debug",
	})
	if err != nil {
		t.Fatalf("failed to create LogManager: %v", err)
	}
	defer func() { _ = lm.Close() }()

	model := NewModelWithTemplates(cfg, []config.Template{}, lm)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model = updated.(Model)

	// Initially shows title
	view := model.View()
	if !strings.Contains(view, "Dev Agent Orchestrator") {
		t.Error("should show title")
	}

	// Test detail panel toggle with right arrow
	model.treeItems = []TreeItem{{Type: TreeItemContainer, ContainerID: "test123"}}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRight})
	model = updated.(Model)

	if !model.detailPanelOpen {
		t.Error("detail panel should open after pressing right arrow")
	}

	// Test detail panel close with left arrow
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	model = updated.(Model)

	if model.detailPanelOpen {
		t.Error("detail panel should close after pressing left arrow")
	}
}

func TestIntegration_LogsAppearInPanel(t *testing.T) {
	cfg := &config.Config{
		Theme: "mocha",
	}

	tmpDir := t.TempDir()
	logPath := tmpDir + "/test-integration-logs-panel.log"
	lm, err := logging.NewManager(logging.Config{
		FilePath:       logPath,
		MaxSizeMB:      1,
		MaxBackups:     1,
		MaxAgeDays:     1,
		ChannelBufSize: 100,
		Level:          "debug",
	})
	if err != nil {
		t.Fatalf("failed to create LogManager: %v", err)
	}
	defer func() { _ = lm.Close() }()

	model := NewModelWithTemplates(cfg, []config.Template{}, lm)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model = updated.(Model)

	// Generate some log entries
	logger := lm.For("test")
	logger.Info("test log message")
	logger.Debug("test debug message")

	// Manually consume logs from the manager's channel into the model
	// This simulates what the Init() -> consumeLogEntries() command does
	entries := make([]logging.LogEntry, 0)
consumeLoop:
	for range 50 {
		select {
		case entry, ok := <-lm.Entries():
			if !ok {
				// Channel closed
				break consumeLoop
			}
			entries = append(entries, entry)
		default:
			// No more entries ready
			break consumeLoop
		}
	}

	if len(entries) == 0 {
		t.Fatalf("expected log entries to be available, got none")
	}

	// Consume logs into model
	updated, _ = model.Update(logEntriesMsg{entries: entries})
	model = updated.(Model)

	// Verify logs were added to the model
	if len(model.logEntries) == 0 {
		t.Error("log entries should be added to model")
	}

	// Open log panel
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	model = updated.(Model)

	view := model.View()

	// View should contain the log panel and some log content
	if !strings.Contains(view, "Logs") {
		t.Error("log panel should be visible")
	}

	// Verify logs flow from manager through the model
	if !model.logPanelOpen {
		t.Error("log panel should be open after pressing L")
	}
}

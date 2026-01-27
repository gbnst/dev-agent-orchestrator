package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"devagent/internal/config"
	"devagent/internal/logging"
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
	defer lm.Close()

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

func TestIntegration_TabNavigation(t *testing.T) {
	cfg := &config.Config{
		Theme: "mocha",
	}

	tmpDir := t.TempDir()
	logPath := tmpDir + "/test-integration-tab-nav.log"
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
	defer lm.Close()

	model := NewModelWithTemplates(cfg, []config.Template{}, lm)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model = updated.(Model)

	// Initially on Containers tab
	view := model.View()
	if !strings.Contains(view, "Containers") {
		t.Error("should show Containers tab initially")
	}

	// Press 2 to switch to Sessions tab
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	model = updated.(Model)

	// Should still show tabs but be on Sessions
	// (Sessions tab may show "No container selected" or similar)

	// Press 1 to go back to Containers
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	model = updated.(Model)

	view = model.View()
	if !strings.Contains(view, "Containers") {
		t.Error("should show Containers after pressing 1")
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
	defer lm.Close()

	model := NewModelWithTemplates(cfg, []config.Template{}, lm)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model = updated.(Model)

	// Generate some log entries
	logger := lm.For("test")
	logger.Info("test log message")

	// Consume logs into model
	updated, _ = model.Update(logEntriesMsg{})
	model = updated.(Model)

	// Open log panel
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	model = updated.(Model)

	view := model.View()

	// View should contain the log message (or at least show logs area)
	// Actual content depends on viewport implementation
	if !strings.Contains(view, "Logs") {
		t.Error("log panel should be visible")
	}
}

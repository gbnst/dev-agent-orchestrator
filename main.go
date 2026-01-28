// pattern: Imperative Shell
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/config"
	"devagent/internal/logging"
	"devagent/internal/tui"
)

func main() {
	configDir := flag.String("config-dir", "", "config directory (default: ~/.config/devagent)")
	flag.Parse()

	var cfg config.Config
	var err error

	if *configDir != "" {
		cfg, err = config.LoadFromDir(*configDir)
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
	}

	// Validate runtime configuration
	if err := cfg.ValidateRuntime(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Initialize logging
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get home directory: %v\n", err)
		os.Exit(1)
	}
	dataDir := filepath.Join(home, ".config", "devagent")
	logPath := filepath.Join(dataDir, "orchestrator.log")

	logManager, err := logging.NewManager(logging.Config{
		FilePath:       logPath,
		MaxSizeMB:      10,
		MaxBackups:     3,
		MaxAgeDays:     7,
		ChannelBufSize: 1000,
		Level:          cfg.LogLevel,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logging: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = logManager.Close() }()

	appLogger := logManager.For("app")
	appLogger.Info("application starting")

	model := tui.NewModel(&cfg, logManager)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		appLogger.Error("application exited with error", "error", err)
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}

	appLogger.Info("application stopped")
}

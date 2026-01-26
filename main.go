package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"devagent/internal/config"
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

	model := tui.NewModel(&cfg)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}

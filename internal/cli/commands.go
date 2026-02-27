// pattern: Imperative Shell
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"devagent/internal/instance"
)

// ResolveDataDir returns the data directory for lock/port files.
// If configDir is specified, uses that; otherwise uses ~/.config/devagent.
func ResolveDataDir(configDir string) string {
	if configDir != "" {
		return configDir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "devagent")
	}
	return filepath.Join(home, ".config", "devagent")
}

// BuildApp creates and configures the CLI application with all commands and groups.
func BuildApp(version string, configDir string) *App {
	app := NewApp(version)

	// Register ungrouped commands
	app.AddCommand(&Command{
		Name:    "list",
		Summary: "Output JSON data about all managed containers",
		Usage:   "Usage: devagent list",
		Run: func(args []string) error {
			return runListCommand(configDir)
		},
	})

	app.AddCommand(&Command{
		Name:    "cleanup",
		Summary: "Remove stale lock/port files from a crashed instance",
		Usage:   "Usage: devagent cleanup",
		Run: func(args []string) error {
			return runCleanupCommand(configDir)
		},
	})

	app.AddCommand(&Command{
		Name:    "version",
		Summary: "Print version and exit",
		Usage:   "Usage: devagent version",
		Run: func(args []string) error {
			fmt.Println(version)
			return nil
		},
	})

	// Register command groups
	worktreeGroup := app.AddGroup("worktree", "Manage git worktrees")
	RegisterWorktreeCommands(worktreeGroup, configDir)

	containerGroup := app.AddGroup("container", "Manage containers")
	RegisterContainerCommands(containerGroup, configDir)

	sessionGroup := app.AddGroup("session", "Manage tmux sessions")
	RegisterSessionCommands(sessionGroup, configDir)

	return app
}

// runListCommand delegates to the running devagent instance via HTTP.
// Requires a running TUI instance — outputs the same project hierarchy
// available at GET /api/projects.
func runListCommand(configDir string) error {
	return runListCommandWithDiscovery(configDir, instance.Discover)
}

// runListCommandWithDiscovery is the internal implementation that accepts
// a discoverer function for testing purposes.
func runListCommandWithDiscovery(configDir string, discoverer func(string) (string, error)) error {
	dataDir := ResolveDataDir(configDir)
	baseURL, err := discoverer(dataDir)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	client := instance.NewClient(baseURL)
	data, err := client.List()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	_, _ = os.Stdout.Write(data)
	return nil
}

// runCleanupCommand removes stale lock and port files from a crashed instance.
func runCleanupCommand(configDir string) error {
	dataDir := ResolveDataDir(configDir)

	// Try to acquire the lock to verify no instance is actually running
	fl, err := instance.Lock(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: a devagent instance appears to be running. Stop it first.\n")
		os.Exit(1)
	}
	// We got the lock — no instance is running. Clean up and release.
	instance.Cleanup(dataDir, fl)
	fmt.Println("Cleaned up stale lock and port files.")
	return nil
}

// pattern: Functional Core
package cli

import (
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
)

// Command represents a single CLI command with its metadata and handler.
type Command struct {
	Name             string
	Summary          string
	Usage            string
	RequiresInstance bool
	Run              func(args []string) error
}

// Group represents a group of related commands.
type Group struct {
	Name     string
	Summary  string
	Commands map[string]*Command
}

// App represents the top-level CLI application with groups and ungrouped commands.
type App struct {
	groups   map[string]*Group
	commands map[string]*Command
	version  string
}

// NewApp creates a new CLI application with the given version.
func NewApp(version string) *App {
	return &App{
		groups:   make(map[string]*Group),
		commands: make(map[string]*Command),
		version:  version,
	}
}

// AddGroup creates and registers a new command group.
func (a *App) AddGroup(name, summary string) *Group {
	g := &Group{
		Name:     name,
		Summary:  summary,
		Commands: make(map[string]*Command),
	}
	a.groups[name] = g
	return g
}

// AddCommand registers an ungrouped (top-level) command.
func (a *App) AddCommand(cmd *Command) {
	a.commands[cmd.Name] = cmd
}

// AddCommand registers a command in the group.
func (g *Group) AddCommand(cmd *Command) {
	g.Commands[cmd.Name] = cmd
}

// Execute dispatches the CLI arguments to the appropriate command.
// Returns true if TUI should be launched, false otherwise.
func (a *App) Execute(args []string) bool {
	// No args: launch TUI
	if len(args) == 0 {
		return true
	}

	cmdName := args[0]

	// Check for ungrouped command
	if cmd, ok := a.commands[cmdName]; ok {
		// Commands handle their own error reporting and exit codes.
		// Any errors are printed to stderr and os.Exit is called as needed.
		_ = cmd.Run(args[1:])
		return false
	}

	// Check for group
	if group, ok := a.groups[cmdName]; ok {
		// Group with no subcommand, "help", or --help/-h
		if len(args) < 2 || args[1] == "help" || args[1] == "--help" || args[1] == "-h" {
			group.PrintHelp(os.Stderr)
			return false
		}

		subCmd := args[1]
		if cmd, ok := group.Commands[subCmd]; ok {
			// Check for help flags
			for _, arg := range args[2:] {
				if arg == "--help" || arg == "-h" {
					fmt.Fprintf(os.Stderr, "%s\n", cmd.Usage)
					return false
				}
			}
			// Execute the command.
			// Commands handle their own error reporting and exit codes.
			_ = cmd.Run(args[2:])
			return false
		}

		// Unknown command in group
		group.PrintHelp(os.Stderr)
		os.Exit(1)
	}

	// Unknown command
	a.PrintHelp(os.Stderr)
	os.Exit(1)
	return false
}

// PrintHelp prints the top-level help text.
func (a *App) PrintHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: devagent [options] [command]\n\n")
	fmt.Fprintf(w, "Commands:\n")

	// Print ungrouped commands
	for _, name := range []string{"list", "cleanup", "version"} {
		if cmd, ok := a.commands[name]; ok {
			fmt.Fprintf(w, "  %-10s %s\n", cmd.Name, cmd.Summary)
		}
	}

	fmt.Fprintf(w, "  %-10s %s\n", "(none)", "Launch interactive TUI")

	// Print groups if any exist
	if len(a.groups) > 0 {
		fmt.Fprintf(w, "\nCommand Groups (requires running instance):\n")
		for _, name := range []string{"worktree", "container", "session"} {
			if group, ok := a.groups[name]; ok {
				fmt.Fprintf(w, "  %-10s %s\n", group.Name, group.Summary)
			}
		}
	}

	fmt.Fprintf(w, "\nUse \"devagent <group> help\" for group details.\n\n")
	fmt.Fprintf(w, "Options:\n")
}

// PrintHelp prints help for a specific group.
func (g *Group) PrintHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: devagent %s <command>\n\n", g.Name)
	fmt.Fprintf(w, "Commands:\n")
	// Sort command names for deterministic output
	names := slices.Sorted(maps.Keys(g.Commands))
	for _, name := range names {
		cmd := g.Commands[name]
		fmt.Fprintf(w, "  %-10s %s\n", cmd.Name, cmd.Summary)
	}
	fmt.Fprintf(w, "\nUse \"devagent %s <command> --help\" for command details.\n", g.Name)
}

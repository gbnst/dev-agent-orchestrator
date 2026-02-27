// pattern: Imperative Shell
package cli

import (
	"fmt"
	"os"
	"time"

	flag "github.com/spf13/pflag"

	"devagent/internal/instance"
)

// RegisterWorktreeCommands registers the worktree command group commands.
// Requires configDir for discovering the running devagent instance.
func RegisterWorktreeCommands(group *Group, configDir string) {
	group.AddCommand(&Command{
		Name:    "create",
		Summary: "Create a new git worktree",
		Usage:   "Usage: devagent worktree create <project-path> <name> [--no-start]",
		Run: func(args []string) error {
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "Usage: devagent worktree create <project-path> <name> [--no-start]\n")
				os.Exit(1)
			}

			projectPath := args[0]
			name := args[1]

			// Parse optional flags
			fs := flag.NewFlagSet("worktree create", flag.ContinueOnError)
			noStart := fs.Bool("no-start", false, "do not start container for the new worktree")
			if err := fs.Parse(args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "Usage: devagent worktree create <project-path> <name> [--no-start]\n")
				os.Exit(1)
			}

			// Create delegate with longer timeout for devcontainer builds
			delegate := Delegate{
				ConfigDir:     configDir,
				ClientTimeout: 120 * time.Second,
			}

			delegate.Run(func(client *instance.Client) error {
				data, err := client.CreateWorktree(projectPath, name, *noStart)
				if err != nil {
					return err
				}
				return PrintJSON(data)
			})

			return nil
		},
	})
}

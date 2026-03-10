// pattern: Imperative Shell
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"

	"devagent/internal/instance"
)

// RegisterSessionCommands registers the session command group commands.
// Requires configDir for discovering the running devagent instance.
// Uses an injectable ExitFunc (via Delegate) for testability.
func RegisterSessionCommands(group *Group, configDir string) {
	group.AddCommand(&Command{
		Name:    "create",
		Summary: "Create a tmux session",
		Usage:   "Usage: devagent session create <container-id-or-name> <session-name>",
		Run: func(args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("usage: devagent session create <container-id-or-name> <session-name>")
			}
			delegate := Delegate{ConfigDir: configDir}
			delegate.Run(func(client *instance.Client) error {
				_, err := client.CreateSession(args[0], args[1])
				if err != nil {
					return err
				}
				fmt.Println("Session created.")
				return nil
			})
			return nil
		},
	})

	group.AddCommand(&Command{
		Name:    "destroy",
		Summary: "Destroy a tmux session",
		Usage:   "Usage: devagent session destroy <container-id-or-name> <session-name>",
		Run: func(args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("usage: devagent session destroy <container-id-or-name> <session-name>")
			}
			delegate := Delegate{ConfigDir: configDir}
			delegate.Run(func(client *instance.Client) error {
				_, err := client.DestroySession(args[0], args[1])
				if err != nil {
					return err
				}
				fmt.Println("Session destroyed.")
				return nil
			})
			return nil
		},
	})

	group.AddCommand(&Command{
		Name:    "readlines",
		Summary: "Read last N lines from scrollback",
		Usage:   "Usage: devagent session readlines <container-id-or-name> <session-name> [N] (default: 20)",
		Run: func(args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("usage: devagent session readlines <container-id-or-name> <session-name> [N]")
			}

			lines := 20 // default
			if len(args) >= 3 {
				n, err := strconv.Atoi(args[2])
				if err != nil || n < 1 {
					return fmt.Errorf("invalid line count %q: must be a positive integer", args[2])
				}
				lines = n
			}

			delegate := Delegate{ConfigDir: configDir}
			delegate.Run(func(client *instance.Client) error {
				data, err := client.ReadLines(args[0], args[1], lines)
				if err != nil {
					return err
				}
				var result struct {
					Content string `json:"content"`
				}
				if err := json.Unmarshal(data, &result); err != nil {
					return fmt.Errorf("failed to parse response: %w", err)
				}
				fmt.Print(result.Content)
				return nil
			})
			return nil
		},
	})

	group.AddCommand(&Command{
		Name:    "send",
		Summary: "Send input to a session",
		Usage:   "Usage: devagent session send <container-id-or-name> <session-name> <text>",
		Run: func(args []string) error {
			if len(args) < 3 {
				return fmt.Errorf("usage: devagent session send <container-id-or-name> <session-name> <text>")
			}
			delegate := Delegate{ConfigDir: configDir}
			delegate.Run(func(client *instance.Client) error {
				if err := client.SendToSession(args[0], args[1], args[2]); err != nil {
					return err
				}
				fmt.Println("Sent.")
				return nil
			})
			return nil
		},
	})

	group.AddCommand(&Command{
		Name:    "tail",
		Summary: "Tail session output",
		Usage:   "Usage: devagent session tail <container-id-or-name> <session-name> [-i/--interval 1s] [--no-color]",
		Run: func(args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("usage: devagent session tail <container-id-or-name> <session-name> [-i/--interval 1s] [--no-color]")
			}

			// Parse flags
			fs := flag.NewFlagSet("session tail", flag.ContinueOnError)
			intervalStr := fs.StringP("interval", "i", "1s", "polling interval")
			noColor := fs.Bool("no-color", false, "strip ANSI color codes")
			if err := fs.Parse(args[2:]); err != nil {
				return fmt.Errorf("usage: devagent session tail <container-id-or-name> <session-name> [-i/--interval 1s] [--no-color]")
			}

			// Parse interval duration
			interval, err := time.ParseDuration(*intervalStr)
			if err != nil {
				return fmt.Errorf("invalid interval: %v", err)
			}

			// Discover and create client
			delegate := Delegate{ConfigDir: configDir}
			client := delegate.Client()
			if client == nil {
				return nil // ExitFunc already called by Client()
			}

			// Set up signal handling for SIGINT and SIGTERM
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-sigCh
				cancel()
			}()

			// Run tail
			err = TailSession(ctx, client, TailConfig{
				ContainerID: args[0],
				Session:     args[1],
				Interval:    interval,
				NoColor:     *noColor,
				Writer:      os.Stdout,
				ErrWriter:   os.Stderr,
			})

			if err != nil {
				return fmt.Errorf("tail failed: %v", err)
			}

			return nil
		},
	})
}

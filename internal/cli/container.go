// pattern: Imperative Shell
package cli

import (
	"fmt"

	"devagent/internal/instance"
)

// RegisterContainerCommands registers the container command group commands.
// Requires configDir for discovering the running devagent instance.
func RegisterContainerCommands(group *Group, configDir string) {
	group.AddCommand(&Command{
		Name:    "start",
		Summary: "Start a container",
		Usage:   "Usage: devagent container start <id-or-name>",
		Run: func(args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("usage: devagent container start <id-or-name>")
			}
			delegate := Delegate{ConfigDir: configDir}
			delegate.Run(func(client *instance.Client) error {
				_, err := client.StartContainer(args[0])
				if err != nil {
					return err
				}
				fmt.Println("Container started.")
				return nil
			})
			return nil
		},
	})

	group.AddCommand(&Command{
		Name:    "stop",
		Summary: "Stop a container",
		Usage:   "Usage: devagent container stop <id-or-name>",
		Run: func(args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("usage: devagent container stop <id-or-name>")
			}
			delegate := Delegate{ConfigDir: configDir}
			delegate.Run(func(client *instance.Client) error {
				_, err := client.StopContainer(args[0])
				if err != nil {
					return err
				}
				fmt.Println("Container stopped.")
				return nil
			})
			return nil
		},
	})

	group.AddCommand(&Command{
		Name:    "destroy",
		Summary: "Destroy a container",
		Usage:   "Usage: devagent container destroy <id-or-name>",
		Run: func(args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("usage: devagent container destroy <id-or-name>")
			}
			delegate := Delegate{ConfigDir: configDir}
			delegate.Run(func(client *instance.Client) error {
				_, err := client.DestroyContainer(args[0])
				if err != nil {
					return err
				}
				fmt.Println("Container destroyed.")
				return nil
			})
			return nil
		},
	})
}

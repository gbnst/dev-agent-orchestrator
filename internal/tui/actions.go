// pattern: Functional Core

package tui

import (
	"encoding/hex"
	"fmt"

	"devagent/internal/container"
)

// ActionCommand represents a command the user can copy/run for a container.
type ActionCommand struct {
	Label   string // Short description
	Command string // The actual command to copy
}

// GenerateContainerActions returns all available actions for a container.
func GenerateContainerActions(c *container.Container, runtimePath string) []ActionCommand {
	if c == nil {
		return nil
	}

	user := c.RemoteUser
	if user == "" {
		user = container.DefaultRemoteUser
	}

	// Read workspace folder from devcontainer.json (defaults to /workspaces)
	workspaceFolder := container.ReadWorkspaceFolder(c.ProjectPath)

	actions := []ActionCommand{
		{
			Label:   "Open in VS Code",
			Command: GenerateVSCodeCommand(c.ProjectPath, workspaceFolder),
		},
		{
			Label:   "Create tmux session (named)",
			Command: fmt.Sprintf("%s exec -it -u %s -w %s -e TERM=xterm-256color -e COLORTERM=truecolor %s tmux -u new-session -s mysession", runtimePath, user, workspaceFolder, c.Name),
		},
		{
			Label:   "Create tmux session (auto)",
			Command: fmt.Sprintf("%s exec -it -u %s -w %s -e TERM=xterm-256color -e COLORTERM=truecolor %s tmux -u new-session", runtimePath, user, workspaceFolder, c.Name),
		},
		{
			Label:   "Interactive shell",
			Command: fmt.Sprintf("%s exec -it -u %s -w %s %s /bin/bash", runtimePath, user, workspaceFolder, c.Name),
		},
	}

	return actions
}

// GenerateVSCodeCommand generates the VS Code command to open a devcontainer.
// projectPath is the host path to the project, workspacePath is the path inside the container.
func GenerateVSCodeCommand(projectPath, workspacePath string) string {
	hexPath := hex.EncodeToString([]byte(projectPath))
	uri := fmt.Sprintf("vscode-remote://dev-container+%s%s", hexPath, workspacePath)
	return fmt.Sprintf("code --folder-uri %s", uri)
}

// GetVSCodePaletteInstructions returns instructions for using VS Code command palette.
func GetVSCodePaletteInstructions() string {
	return "In VS Code: Cmd+Shift+P > \"Dev Containers: Attach to Running Container...\""
}

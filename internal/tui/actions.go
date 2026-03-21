// pattern: Functional Core

package tui

import (
	"encoding/hex"
	"encoding/json"
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
			Command: GenerateVSCodeCommand(c.ID, workspaceFolder),
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

// GenerateVSCodeURI builds the vscode-remote URI to attach to a running container.
// containerID is the full 64-character Docker/Podman container ID.
// workspacePath is the path inside the container (e.g. /workspaces).
func GenerateVSCodeURI(containerID, workspacePath string) string {
	payload, _ := json.Marshal(map[string]string{"containerName": containerID})
	hexPayload := hex.EncodeToString(payload)
	return fmt.Sprintf("vscode-remote://attached-container+%s%s", hexPayload, workspacePath)
}

// GenerateVSCodeCommand returns the full CLI command to open VS Code attached to a container.
func GenerateVSCodeCommand(containerID, workspacePath string) string {
	return fmt.Sprintf("code --folder-uri %s", GenerateVSCodeURI(containerID, workspacePath))
}

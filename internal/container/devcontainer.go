// pattern: Imperative Shell

package container

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"devagent/internal/config"
)

// processTemplate is imported from compose.go - declared here for documentation
// It processes a template file with the given data and returns the rendered content
// This is used by copyTemplateDir for .tmpl file processing

// getDataDir returns the XDG-compliant data directory for devagent.
// Uses $XDG_DATA_HOME/devagent or ~/.local/share/devagent
func getDataDir() string {
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		return filepath.Join(xdgData, "devagent")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "share", "devagent")
	}
	return filepath.Join(home, ".local", "share", "devagent")
}

// claudeSetupTokenFunc is the function used to run claude setup-token.
// It's a package-level variable so tests can override it.
var claudeSetupTokenFunc = func() (string, error) {
	cmd := exec.Command("claude", "setup-token")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// ensureClaudeToken ensures a Claude OAuth token exists at the given path.
// tokenPath must be an already-resolved absolute path.
// If empty, returns ("", ""). If file exists, reads it.
// If file doesn't exist, runs claude setup-token and saves to that path.
func ensureClaudeToken(tokenPath string) (string, string) {
	if tokenPath == "" {
		return "", ""
	}

	// Check if token file already exists
	if data, err := os.ReadFile(tokenPath); err == nil {
		return tokenPath, strings.TrimSpace(string(data))
	}

	// Token doesn't exist, try to create it via claude setup-token
	output, err := claudeSetupTokenFunc()
	if err != nil {
		return "", ""
	}

	// claude setup-token includes TUI rendering and ANSI codes in its output;
	// extract just the token which starts with "sk-ant-"
	re := regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]+`)
	match := re.FindString(output)
	if match == "" {
		return "", ""
	}
	token := strings.TrimSpace(match)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0755); err != nil {
		return "", ""
	}

	// Save token to file
	if err := os.WriteFile(tokenPath, []byte(token), 0600); err != nil {
		return "", ""
	}

	return tokenPath, token
}

// ensureGitHubToken reads a GitHub Personal Access Token from the given path.
// tokenPath must be an already-resolved absolute path.
// If empty or file doesn't exist, returns ("", "").
func ensureGitHubToken(tokenPath string) (string, string) {
	if tokenPath == "" {
		return "", ""
	}

	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", ""
	}

	return tokenPath, strings.TrimSpace(string(data))
}

// ReadWorkspaceFolder reads the workspaceFolder from a project's devcontainer.json.
// Returns the workspace folder path, or a default of "/workspaces" if not specified or on error.
func ReadWorkspaceFolder(projectPath string) string {
	defaultFolder := "/workspaces"
	if projectPath == "" {
		return defaultFolder
	}

	jsonPath := filepath.Join(projectPath, ".devcontainer", "devcontainer.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return defaultFolder
	}

	var dc DevcontainerJSON
	if err := json.Unmarshal(data, &dc); err != nil {
		return defaultFolder
	}

	if dc.WorkspaceFolder != "" {
		return dc.WorkspaceFolder
	}
	return defaultFolder
}

// DevcontainerGenerator creates devcontainer.json configurations.
type DevcontainerGenerator struct {
	cfg       *config.Config
	templates []config.Template
}

// NewDevcontainerGenerator creates a new generator with the given config and templates.
func NewDevcontainerGenerator(cfg *config.Config, templates []config.Template) *DevcontainerGenerator {
	return &DevcontainerGenerator{
		cfg:       cfg,
		templates: templates,
	}
}

// GenerateResult holds the generated devcontainer config and template metadata.
type GenerateResult struct {
	TemplatePath string // Path to template directory for copying additional files
}

// GetTemplate retrieves a template by name.
// Returns nil if template not found.
func (g *DevcontainerGenerator) GetTemplate(templateName string) *config.Template {
	for i := range g.templates {
		if g.templates[i].Name == templateName {
			return &g.templates[i]
		}
	}
	return nil
}

// Generate creates devcontainer.json from template files for compose-based orchestration.
// Templates must provide devcontainer.json.tmpl and docker-compose.yml.tmpl.
func (g *DevcontainerGenerator) Generate(opts CreateOptions) (*GenerateResult, error) {
	// Find template
	var tmpl *config.Template
	for i := range g.templates {
		if g.templates[i].Name == opts.Template {
			tmpl = &g.templates[i]
			break
		}
	}
	if tmpl == nil {
		return nil, fmt.Errorf("template not found: %s", opts.Template)
	}

	return &GenerateResult{TemplatePath: tmpl.Path}, nil
}

// copyTemplateDir copies a directory tree from src to dst, processing .tmpl files with templateData.
// Non-.tmpl files are copied as-is. Directories are created as needed.
// The .gitkeep files are copied to preserve empty directories.
func copyTemplateDir(src, dst string, data TemplateData) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dst, relPath)

		// Create directories
		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		// Process .tmpl files
		if strings.HasSuffix(relPath, ".tmpl") {
			content, err := processTemplate(path, data)
			if err != nil {
				return fmt.Errorf("failed to process template %s: %w", relPath, err)
			}
			// Write without .tmpl extension
			outputPath := strings.TrimSuffix(destPath, ".tmpl")
			return os.WriteFile(outputPath, []byte(content), 0644)
		}

		// Copy other files as-is
		return copyFile(path, destPath)
	})
}

// WriteToProject writes the devcontainer.json and any additional template files
// from the template's .devcontainer directory to the project's .devcontainer directory.
func (g *DevcontainerGenerator) WriteToProject(projectPath string, result *GenerateResult, templateData TemplateData) error {
	if result.TemplatePath == "" {
		return fmt.Errorf("no template path specified")
	}

	src := filepath.Join(result.TemplatePath, ".devcontainer")
	dst := filepath.Join(projectPath, ".devcontainer")

	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	return copyTemplateDir(src, dst, templateData)
}

// WriteAll writes all generated files to the project's .devcontainer directory.
func (g *DevcontainerGenerator) WriteAll(projectPath string, devResult *GenerateResult, composeResult *ComposeResult) error {
	return g.WriteToProject(projectPath, devResult, composeResult.TemplateData)
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// DevcontainerCLI wraps the devcontainer CLI.
type DevcontainerCLI struct {
	exec       CommandExecutor
	dockerPath string // path to runtime binary (docker/podman)
}

// NewDevcontainerCLI creates a new DevcontainerCLI.
func NewDevcontainerCLI() *DevcontainerCLI {
	return &DevcontainerCLI{
		exec: defaultExecutor,
	}
}

// NewDevcontainerCLIWithRuntime creates a new DevcontainerCLI with an explicit runtime.
func NewDevcontainerCLIWithRuntime(runtime string) *DevcontainerCLI {
	return &DevcontainerCLI{
		exec:       defaultExecutor,
		dockerPath: runtime,
	}
}

// NewDevcontainerCLIWithExecutor creates a new DevcontainerCLI with a custom executor for testing.
func NewDevcontainerCLIWithExecutor(exec CommandExecutor) *DevcontainerCLI {
	return &DevcontainerCLI{
		exec: exec,
	}
}

// NewDevcontainerCLIWithExecutorAndRuntime creates a new DevcontainerCLI with both custom executor and runtime.
func NewDevcontainerCLIWithExecutorAndRuntime(exec CommandExecutor, runtime string) *DevcontainerCLI {
	return &DevcontainerCLI{
		exec:       exec,
		dockerPath: runtime,
	}
}

// upResponse represents the JSON output from devcontainer up.
type upResponse struct {
	ContainerID string `json:"containerId"`
}

// Up starts a devcontainer from the project directory.
// Returns the container ID even if the command fails (e.g. postCreateCommand error),
// since the container may already be running. Callers should check both return values.
func (c *DevcontainerCLI) Up(ctx context.Context, projectPath string) (string, error) {
	args := []string{"up", "--workspace-folder", projectPath}
	if c.dockerPath != "" {
		args = append(args, "--docker-path", c.dockerPath)
	}

	output, execErr := c.exec(ctx, "devcontainer", args...)

	// devcontainer up writes JSON to stdout even on failure (e.g. postCreateCommand error).
	// Try to extract the container ID regardless of exit code.
	var resp upResponse
	if parseErr := json.Unmarshal([]byte(output), &resp); parseErr != nil {
		if execErr != nil {
			return "", execErr
		}
		return "", parseErr
	}

	return resp.ContainerID, execErr
}

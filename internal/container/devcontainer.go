// pattern: Imperative Shell

package container

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"devagent/internal/config"
)

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

// getContainerClaudeDir returns the host path for a container's .claude directory.
// Uses a hash of the project path to create a unique directory per container.
func getContainerClaudeDir(projectPath string) string {
	// Use SHA256 hash of project path for uniqueness
	hash := sha256.Sum256([]byte(projectPath))
	hashStr := hex.EncodeToString(hash[:])[:HashTruncLen]
	return filepath.Join(getDataDir(), "claude-configs", hashStr)
}

// ensureClaudeDir creates the host-side claude config directory for a container.
func ensureClaudeDir(projectPath string) (string, error) {
	dir := getContainerClaudeDir(projectPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create claude config directory: %w", err)
	}
	return dir, nil
}

// copyClaudeTemplateFiles copies template claude files to the container's claude config dir.
// Files from <templatePath>/home/vscode/.claude/ are copied to claudeDir.
// Existing files are not overwritten, preserving user modifications.
func copyClaudeTemplateFiles(templatePath, claudeDir string) error {
	srcDir := filepath.Join(templatePath, "home", "vscode", ".claude")

	info, err := os.Stat(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}

	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(claudeDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		// Skip existing files to preserve user modifications
		if _, err := os.Stat(destPath); err == nil {
			return nil
		}

		return copyFile(path, destPath)
	})
}

// getClaudeConfigDir returns the XDG-compliant Claude config directory.
// Uses $XDG_CONFIG_HOME/claude or ~/.claude as fallback.
func getClaudeConfigDir() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "claude")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".claude")
	}
	return filepath.Join(home, ".claude")
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

// ensureClaudeToken ensures a Claude OAuth token exists for devagent use.
// Returns the token file path and token content, or empty strings on error.
// Errors are non-blocking - logged but don't prevent container creation.
func ensureClaudeToken() (tokenPath string, token string) {
	claudeDir := getClaudeConfigDir()
	tokenPath = filepath.Join(claudeDir, ".devagent-claude-token")

	// Check if token file already exists
	if data, err := os.ReadFile(tokenPath); err == nil {
		return tokenPath, strings.TrimSpace(string(data))
	}

	// Token doesn't exist, try to create it via claude setup-token
	output, err := claudeSetupTokenFunc()
	if err != nil {
		// Non-blocking: log would go here, but we just return empty
		return "", ""
	}

	token = strings.TrimSpace(output)
	if token == "" {
		return "", ""
	}

	// Ensure claude config directory exists
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return "", ""
	}

	// Save token to file
	if err := os.WriteFile(tokenPath, []byte(token), 0600); err != nil {
		return "", ""
	}

	return tokenPath, token
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
	Config               *DevcontainerJSON
	TemplatePath         string // Path to template directory for copying additional files
	CopyDockerfile       string // Dockerfile to copy (relative to TemplatePath), independent of Config.Build
	DevcontainerTemplate string // Processed devcontainer.json.tmpl content (template-driven mode)
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

	// All templates have a Dockerfile in their template directory.
	// It is copied to .devcontainer/ separately because devcontainer CLI
	// ignores dockerComposeFile when build is present in devcontainer.json.
	copyDockerfile := "Dockerfile"

	// Ensure claude config dir exists and seed with template files
	claudeDir, err := ensureClaudeDir(opts.ProjectPath)
	if err != nil {
		return nil, err
	}
	if err := copyClaudeTemplateFiles(tmpl.Path, claudeDir); err != nil {
		return nil, fmt.Errorf("failed to copy claude template files: %w", err)
	}

	return g.generateFromTemplate(tmpl, opts, copyDockerfile)
}

// generateFromTemplate processes devcontainer.json.tmpl and returns the result.
func (g *DevcontainerGenerator) generateFromTemplate(tmpl *config.Template, opts CreateOptions, copyDockerfile string) (*GenerateResult, error) {
	// Build template data (same data used for docker-compose.yml.tmpl)
	hash := projectHash(opts.ProjectPath)
	projectName := filepath.Base(opts.ProjectPath)

	data := TemplateData{
		ProjectHash:        hash,
		ProjectPath:        opts.ProjectPath,
		ProjectName:        projectName,
		WorkspaceFolder:    fmt.Sprintf("/workspaces/%s", projectName),
		ClaudeConfigDir:    getContainerClaudeDir(opts.ProjectPath),
		TemplateName:       tmpl.Name,
		ContainerName:      opts.Name,
		CertInstallCommand: certInstallCommand,
		ProxyImage:         "mitmproxy/mitmproxy:latest",
		ProxyPort:          "8080",
		RemoteUser:         DefaultRemoteUser,
	}

	// Process the template
	content, err := ProcessDevcontainerTemplate(tmpl.Path, data)
	if err != nil {
		return nil, fmt.Errorf("failed to process devcontainer.json.tmpl: %w", err)
	}

	return &GenerateResult{
		Config:               nil, // No struct when using template
		TemplatePath:         tmpl.Path,
		CopyDockerfile:       copyDockerfile,
		DevcontainerTemplate: content, // New field for template output
	}, nil
}

// WriteToProject writes the devcontainer.json and any additional template files
// (like Dockerfile and home directory) to the project's .devcontainer directory.
func (g *DevcontainerGenerator) WriteToProject(projectPath string, result *GenerateResult) error {
	devcontainerDir := filepath.Join(projectPath, ".devcontainer")
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		return err
	}

	// Copy Dockerfile if specified
	// CopyDockerfile takes precedence (used for compose mode where Build shouldn't be serialized)
	// Falls back to Config.Build.Dockerfile for backward compatibility (non-compose mode)
	var dockerfileToCopy string
	if result.CopyDockerfile != "" {
		dockerfileToCopy = result.CopyDockerfile
	} else if result.Config != nil && result.Config.Build != nil && result.Config.Build.Dockerfile != "" {
		dockerfileToCopy = result.Config.Build.Dockerfile
	}

	if dockerfileToCopy != "" && result.TemplatePath != "" {
		srcDockerfile := filepath.Join(result.TemplatePath, dockerfileToCopy)
		dstDockerfile := filepath.Join(devcontainerDir, dockerfileToCopy)
		if err := copyFile(srcDockerfile, dstDockerfile); err != nil {
			return fmt.Errorf("failed to copy Dockerfile: %w", err)
		}
	}

	// Copy home directory if it exists (for Dockerfile COPY)
	if result.TemplatePath != "" {
		srcHome := filepath.Join(result.TemplatePath, "home")
		if info, err := os.Stat(srcHome); err == nil && info.IsDir() {
			dstHome := filepath.Join(devcontainerDir, "home")
			if err := copyDir(srcHome, dstHome); err != nil {
				return fmt.Errorf("failed to copy home directory: %w", err)
			}
		}
	}

	jsonPath := filepath.Join(devcontainerDir, "devcontainer.json")

	if result.DevcontainerTemplate == "" {
		return fmt.Errorf("no devcontainer template content generated")
	}

	return os.WriteFile(jsonPath, []byte(result.DevcontainerTemplate), 0644)
}

// WriteComposeFiles writes docker-compose.yml, Dockerfile.proxy, and filter.py
// to the project's .devcontainer directory alongside the devcontainer.json.
func (g *DevcontainerGenerator) WriteComposeFiles(projectPath string, composeResult *ComposeResult) error {
	devcontainerDir := filepath.Join(projectPath, ".devcontainer")
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		return fmt.Errorf("failed to create .devcontainer directory: %w", err)
	}

	// Write docker-compose.yml
	composePath := filepath.Join(devcontainerDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(composeResult.ComposeYAML), 0644); err != nil {
		return fmt.Errorf("failed to write docker-compose.yml: %w", err)
	}

	// Write Dockerfile.proxy
	dockerfileProxyPath := filepath.Join(devcontainerDir, "Dockerfile.proxy")
	if err := os.WriteFile(dockerfileProxyPath, []byte(composeResult.DockerfileProxy), 0644); err != nil {
		return fmt.Errorf("failed to write Dockerfile.proxy: %w", err)
	}

	// Write filter.py
	filterPath := filepath.Join(devcontainerDir, "filter.py")
	if err := os.WriteFile(filterPath, []byte(composeResult.FilterScript), 0644); err != nil {
		return fmt.Errorf("failed to write filter.py: %w", err)
	}

	return nil
}

// WriteAll writes all generated files to the project's .devcontainer directory.
// This includes devcontainer.json and optionally compose files if composeResult is provided.
func (g *DevcontainerGenerator) WriteAll(projectPath string, devcontainerResult *GenerateResult, composeResult *ComposeResult) error {
	// Write devcontainer.json and template files
	if err := g.WriteToProject(projectPath, devcontainerResult); err != nil {
		return fmt.Errorf("failed to write devcontainer files: %w", err)
	}

	// Write compose files if provided
	if composeResult != nil {
		if err := g.WriteComposeFiles(projectPath, composeResult); err != nil {
			return fmt.Errorf("failed to write compose files: %w", err)
		}
	}

	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// copyDir recursively copies a directory from src to dst.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		return copyFile(path, destPath)
	})
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
func (c *DevcontainerCLI) Up(ctx context.Context, projectPath string) (string, error) {
	args := []string{"up", "--workspace-folder", projectPath}
	if c.dockerPath != "" {
		args = append(args, "--docker-path", c.dockerPath)
	}

	output, err := c.exec(ctx, "devcontainer", args...)
	if err != nil {
		return "", err
	}

	var resp upResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return "", err
	}

	return resp.ContainerID, nil
}

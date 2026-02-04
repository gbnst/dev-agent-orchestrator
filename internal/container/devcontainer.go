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
	// Use SHA256 hash of project path for uniqueness (truncated to 12 chars)
	hash := sha256.Sum256([]byte(projectPath))
	hashStr := hex.EncodeToString(hash[:])[:12]
	return filepath.Join(getDataDir(), "claude-configs", hashStr)
}

// ensureClaudeDir creates the claude config directory for a container if it doesn't exist.
func ensureClaudeDir(projectPath string) (string, error) {
	dir := getContainerClaudeDir(projectPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create claude config directory: %w", err)
	}
	return dir, nil
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

// copyClaudeTemplateFiles copies files from template's home/vscode/.claude/ to the claude config dir.
// Only copies files that don't already exist (won't overwrite user changes).
func copyClaudeTemplateFiles(templatePath, claudeDir string) error {
	// Look for home/vscode/.claude/ in the template directory
	srcDir := filepath.Join(templatePath, "home", "vscode", ".claude")

	info, err := os.Stat(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			// No template files to copy, that's fine
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}

	// Walk the source directory and copy files
	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Get relative path from srcDir
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(claudeDir, relPath)

		if d.IsDir() {
			// Create directory if it doesn't exist
			return os.MkdirAll(destPath, 0755)
		}

		// Only copy files that don't already exist (preserve user modifications)
		if _, err := os.Stat(destPath); err == nil {
			// File exists, skip
			return nil
		}

		// Copy the file
		return copyFile(path, destPath)
	})
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

// buildIsolationRunArgs generates runtime arguments from isolation config
func buildIsolationRunArgs(iso *config.IsolationConfig) []string {
	if iso == nil {
		return nil
	}

	var args []string

	// Add capability drops
	if iso.Caps != nil {
		for _, cap := range iso.Caps.Drop {
			args = append(args, "--cap-drop", cap)
		}
	}

	// Add resource limits
	if iso.Resources != nil {
		if iso.Resources.Memory != "" {
			args = append(args, "--memory", iso.Resources.Memory)
		}
		if iso.Resources.CPUs != "" {
			args = append(args, "--cpus", iso.Resources.CPUs)
		}
		if iso.Resources.PidsLimit > 0 {
			args = append(args, "--pids-limit", fmt.Sprintf("%d", iso.Resources.PidsLimit))
		}
	}

	return args
}

// chainPostCreateCommand appends a command to an existing postCreateCommand.
// If the existing command is empty, returns just the new command.
// Commands are joined with " && " to run sequentially.
func chainPostCreateCommand(existing, additional string) string {
	if existing == "" {
		return additional
	}
	if additional == "" {
		return existing
	}
	return existing + " && " + additional
}

// GenerateResult holds the generated devcontainer config and template metadata.
type GenerateResult struct {
	Config       *DevcontainerJSON
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

// Generate creates a DevcontainerJSON from the given options.
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

	dc := &DevcontainerJSON{
		Name:              opts.Name,
		Image:             tmpl.Image,
		PostCreateCommand: tmpl.PostCreateCommand,
		ContainerEnv:      make(map[string]string),
		RunArgs:           []string{},
	}

	// Copy build config from template
	if tmpl.Build != nil {
		dc.Build = &BuildConfig{
			Dockerfile: tmpl.Build.Dockerfile,
			Context:    tmpl.Build.Context,
		}
	}

	// Copy features from template (shallow copy of map values is sufficient)
	if tmpl.Features != nil {
		dc.Features = make(map[string]map[string]interface{})
		for k, v := range tmpl.Features {
			dc.Features[k] = v
		}
	}

	// Copy customizations from template
	if tmpl.Customizations != nil {
		dc.Customizations = tmpl.Customizations
	}

	// Inject credentials
	for _, credName := range tmpl.InjectCredentials {
		if value, ok := g.cfg.GetCredentialValue(credName); ok {
			dc.ContainerEnv[credName] = value
		}
	}

	// Inject agent OTEL env
	agentName := opts.Agent
	if agentName == "" {
		agentName = tmpl.DefaultAgent
	}
	if agentName != "" {
		if agent, ok := g.cfg.Agents[agentName]; ok {
			for k, v := range agent.OTELEnv {
				dc.ContainerEnv[k] = v
			}
		}
	}

	// Set IS_DEMO to skip onboarding prompts
	dc.ContainerEnv["IS_DEMO"] = "1"

	// Add proxy environment variables if network isolation is enabled
	if opts.Proxy != nil {
		proxyURL := fmt.Sprintf("http://%s:%s", opts.Proxy.ProxyHost, opts.Proxy.ProxyPort)

		// Standard proxy variables (lowercase and uppercase for compatibility)
		dc.ContainerEnv["http_proxy"] = proxyURL
		dc.ContainerEnv["https_proxy"] = proxyURL
		dc.ContainerEnv["HTTP_PROXY"] = proxyURL
		dc.ContainerEnv["HTTPS_PROXY"] = proxyURL

		// Bypass proxy for localhost
		dc.ContainerEnv["no_proxy"] = "localhost,127.0.0.1"
		dc.ContainerEnv["NO_PROXY"] = "localhost,127.0.0.1"

		// Certificate bundle paths for various runtimes
		// These point to the system trust store that includes our CA cert
		dc.ContainerEnv["REQUESTS_CA_BUNDLE"] = "/etc/ssl/certs/ca-certificates.crt"
		dc.ContainerEnv["NODE_EXTRA_CA_CERTS"] = "/etc/ssl/certs/ca-certificates.crt"
		dc.ContainerEnv["SSL_CERT_FILE"] = "/etc/ssl/certs/ca-certificates.crt"
	}

	// Add devagent labels
	dc.RunArgs = append(dc.RunArgs,
		"--label", LabelManagedBy+"=true",
	)
	if opts.ProjectPath != "" {
		dc.RunArgs = append(dc.RunArgs,
			"--label", LabelProjectPath+"="+opts.ProjectPath,
		)
	}
	dc.RunArgs = append(dc.RunArgs,
		"--label", LabelTemplate+"="+opts.Template,
	)
	if agentName != "" {
		dc.RunArgs = append(dc.RunArgs,
			"--label", LabelAgent+"="+agentName,
		)
	}

	// Add remote user label (default to vscode per devcontainer spec)
	remoteUser := tmpl.RemoteUser
	if remoteUser == "" {
		remoteUser = DefaultRemoteUser
	}
	dc.RunArgs = append(dc.RunArgs,
		"--label", LabelRemoteUser+"="+remoteUser,
	)

	// Set Docker container name via runArgs
	if opts.Name != "" {
		dc.RunArgs = append(dc.RunArgs, "--name", opts.Name)
	}

	// Add network attachment if network isolation is enabled
	if opts.Proxy != nil && opts.Proxy.NetworkName != "" {
		dc.RunArgs = append(dc.RunArgs, "--network", opts.Proxy.NetworkName)
	}

	// Get effective isolation (merges template config with defaults)
	// This ensures templates without explicit isolation config get default isolation applied
	effectiveIsolation := tmpl.GetEffectiveIsolation()

	// Add isolation runtime arguments
	if effectiveIsolation != nil {
		dc.RunArgs = append(dc.RunArgs, buildIsolationRunArgs(effectiveIsolation)...)

		// Add capAdd to devcontainer.json native field
		if effectiveIsolation.Caps != nil && len(effectiveIsolation.Caps.Add) > 0 {
			dc.CapAdd = effectiveIsolation.Caps.Add
		}
	}

	// Add mount for Claude config directory
	// This maps a per-container directory on host to /home/vscode/.claude in container
	if opts.ProjectPath != "" {
		claudeDir, err := ensureClaudeDir(opts.ProjectPath)
		if err != nil {
			return nil, err
		}

		// Copy template files to claude config dir (only if they don't already exist)
		if tmpl.Path != "" {
			if err := copyClaudeTemplateFiles(tmpl.Path, claudeDir); err != nil {
				return nil, fmt.Errorf("failed to copy claude template files: %w", err)
			}
		}

		// Format: source=<host-path>,target=<container-path>,type=bind
		mount := fmt.Sprintf("source=%s,target=/home/%s/.claude,type=bind", claudeDir, remoteUser)
		dc.Mounts = append(dc.Mounts, mount)
	}

	// Add mount for Claude OAuth token (if available)
	// Token is provisioned via 'claude setup-token' if not already present
	if tokenPath, _ := ensureClaudeToken(); tokenPath != "" {
		tokenMount := fmt.Sprintf("source=%s,target=/run/secrets/claude-token,type=bind,readonly", tokenPath)
		dc.Mounts = append(dc.Mounts, tokenMount)
	}

	// Add proxy certificate mount if network isolation is enabled
	if opts.Proxy != nil && opts.Proxy.CertDir != "" {
		// Mount the CA cert to a location where postCreateCommand can access it
		certMount := fmt.Sprintf("source=%s,target=/tmp/mitmproxy-certs,type=bind,readonly",
			opts.Proxy.CertDir)
		dc.Mounts = append(dc.Mounts, certMount)

		// Chain cert installation to postCreateCommand
		// Copy cert with .crt extension and update trust store
		certInstallCmd := "sudo cp /tmp/mitmproxy-certs/mitmproxy-ca-cert.pem /usr/local/share/ca-certificates/mitmproxy-ca-cert.crt && sudo update-ca-certificates"
		dc.PostCreateCommand = chainPostCreateCommand(dc.PostCreateCommand, certInstallCmd)
	}

	return &GenerateResult{
		Config:       dc,
		TemplatePath: tmpl.Path,
	}, nil
}

// WriteToProject writes the devcontainer.json and any additional template files
// (like Dockerfile and home directory) to the project's .devcontainer directory.
func (g *DevcontainerGenerator) WriteToProject(projectPath string, result *GenerateResult) error {
	devcontainerDir := filepath.Join(projectPath, ".devcontainer")
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		return err
	}

	// If template uses a Dockerfile, copy it
	if result.Config.Build != nil && result.Config.Build.Dockerfile != "" && result.TemplatePath != "" {
		srcDockerfile := filepath.Join(result.TemplatePath, result.Config.Build.Dockerfile)
		dstDockerfile := filepath.Join(devcontainerDir, result.Config.Build.Dockerfile)
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

	data, err := json.MarshalIndent(result.Config, "", "  ")
	if err != nil {
		return err
	}

	jsonPath := filepath.Join(devcontainerDir, "devcontainer.json")
	return os.WriteFile(jsonPath, data, 0644)
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

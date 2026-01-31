// pattern: Imperative Shell

package container

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"devagent/internal/config"
)

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
	Config       *DevcontainerJSON
	TemplatePath string // Path to template directory for copying additional files
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

	return &GenerateResult{
		Config:       dc,
		TemplatePath: tmpl.Path,
	}, nil
}

// WriteToProject writes the devcontainer.json and any additional template files
// (like Dockerfile) to the project's .devcontainer directory.
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

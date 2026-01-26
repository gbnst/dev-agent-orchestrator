package container

import (
	"context"
	"encoding/json"
	"errors"
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

// Generate creates a DevcontainerJSON from the given options.
func (g *DevcontainerGenerator) Generate(opts CreateOptions) (*DevcontainerJSON, error) {
	// Find template
	var tmpl *config.Template
	for i := range g.templates {
		if g.templates[i].Name == opts.Template {
			tmpl = &g.templates[i]
			break
		}
	}
	if tmpl == nil {
		return nil, errors.New("template not found: " + opts.Template)
	}

	// Resolve base image
	image, ok := g.cfg.ResolveBaseImage(tmpl.BaseImage)
	if !ok {
		return nil, errors.New("base image not found: " + tmpl.BaseImage)
	}

	dc := &DevcontainerJSON{
		Name:           opts.Name,
		Image:          image,
		ContainerEnv:   make(map[string]string),
		RunArgs:        []string{},
	}

	// Copy features from template
	if tmpl.Devcontainer.Features != nil {
		dc.Features = make(map[string]map[string]interface{})
		for k, v := range tmpl.Devcontainer.Features {
			dc.Features[k] = v
		}
	}

	// Copy customizations from template
	if tmpl.Devcontainer.Customizations != nil {
		dc.Customizations = tmpl.Devcontainer.Customizations
	}

	// Copy post create command
	if tmpl.Devcontainer.PostCreateCommand != "" {
		dc.PostCreateCommand = tmpl.Devcontainer.PostCreateCommand
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

	return dc, nil
}

// WriteToProject writes the devcontainer.json to the project's .devcontainer directory.
func (g *DevcontainerGenerator) WriteToProject(projectPath string, dc *DevcontainerJSON) error {
	devcontainerDir := filepath.Join(projectPath, ".devcontainer")
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(dc, "", "  ")
	if err != nil {
		return err
	}

	jsonPath := filepath.Join(devcontainerDir, "devcontainer.json")
	return os.WriteFile(jsonPath, data, 0644)
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

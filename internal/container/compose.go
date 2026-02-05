// pattern: Functional Core

package container

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"devagent/internal/config"
)

// certInstallCommand is the shell command that waits for the mitmproxy CA cert
// to become available, then installs it into the system trust store.
const certInstallCommand = "timeout=30; while [ ! -f /tmp/mitmproxy-certs/mitmproxy-ca-cert.pem ] && [ $timeout -gt 0 ]; do sleep 1; timeout=$((timeout-1)); done && sudo cp /tmp/mitmproxy-certs/mitmproxy-ca-cert.pem /usr/local/share/ca-certificates/mitmproxy-ca-cert.crt && sudo update-ca-certificates"

// ComposeResult holds the generated compose configuration files.
type ComposeResult struct {
	ComposeYAML     string // docker-compose.yml content
	DockerfileProxy string // Dockerfile.proxy content
	FilterScript    string // filter.py content
}

// TemplateData holds all values for template placeholder substitution.
// Only instance-specific values are substituted - everything else is hardcoded in templates.
type TemplateData struct {
	ProjectHash        string // 12-char SHA256 of project path
	ProjectPath        string // Absolute path to project
	ProjectName        string // Base name of project directory
	WorkspaceFolder    string // /workspaces/{{.ProjectName}}
	ClaudeConfigDir    string // Host path for persistent .claude directory
	ClaudeTokenPath    string // Host path to Claude OAuth token file (absolute)
	TemplateName       string // Template name (e.g., "basic")
	ContainerName      string // Container name for devcontainer.json
	CertInstallCommand string // Command to wait for, copy, and trust mitmproxy CA cert
}

// ComposeGenerator creates docker-compose.yml and related files for container orchestration.
type ComposeGenerator struct {
	templates []config.Template
}

// NewComposeGenerator creates a new generator with the given templates.
func NewComposeGenerator(templates []config.Template) *ComposeGenerator {
	return &ComposeGenerator{
		templates: templates,
	}
}

// GetTemplate retrieves a template by name.
// Returns nil if template not found.
func (g *ComposeGenerator) GetTemplate(templateName string) *config.Template {
	for i := range g.templates {
		if g.templates[i].Name == templateName {
			return &g.templates[i]
		}
	}
	return nil
}

// ComposeOptions holds options for generating compose files.
// This is a subset of CreateOptions needed for compose generation.
type ComposeOptions struct {
	ProjectPath string
	Template    string
	Name        string // Container name (used for compose service naming)
}

// Generate creates docker-compose.yml, Dockerfile.proxy, and filter.py content.
// Returns ComposeResult with all generated file contents.
func (g *ComposeGenerator) Generate(opts ComposeOptions) (*ComposeResult, error) {
	// Find template
	tmpl := g.GetTemplate(opts.Template)
	if tmpl == nil {
		return nil, fmt.Errorf("template not found: %s", opts.Template)
	}

	// Build template data
	data := g.buildTemplateData(opts, tmpl)

	// Process docker-compose.yml.tmpl
	composeYAML, err := g.processComposeTemplate(tmpl.Path, data)
	if err != nil {
		return nil, fmt.Errorf("failed to process docker-compose.yml.tmpl: %w", err)
	}

	// Load Dockerfile.proxy from template (static file)
	dockerfileProxy, err := g.loadDockerfileProxy(tmpl.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to load Dockerfile.proxy: %w", err)
	}

	// Process filter.py.tmpl (currently static, but processed for consistency)
	filterScript, err := g.processFilterTemplate(tmpl.Path, data)
	if err != nil {
		return nil, fmt.Errorf("failed to process filter.py.tmpl: %w", err)
	}

	return &ComposeResult{
		ComposeYAML:     composeYAML,
		DockerfileProxy: dockerfileProxy,
		FilterScript:    filterScript,
	}, nil
}

// buildTemplateData constructs TemplateData from options and template.
func (g *ComposeGenerator) buildTemplateData(opts ComposeOptions, tmpl *config.Template) TemplateData {
	hash := projectHash(opts.ProjectPath)
	projectName := filepath.Base(opts.ProjectPath)

	// Ensure Claude token exists (non-blocking on error).
	// Falls back to /dev/null so Docker doesn't create an empty directory.
	tokenPath, _ := ensureClaudeToken()
	if tokenPath == "" {
		tokenPath = "/dev/null"
	}

	return TemplateData{
		ProjectHash:        hash,
		ProjectPath:        opts.ProjectPath,
		ProjectName:        projectName,
		WorkspaceFolder:    fmt.Sprintf("/workspaces/%s", projectName),
		ClaudeConfigDir:    getContainerClaudeDir(opts.ProjectPath),
		ClaudeTokenPath:    tokenPath,
		TemplateName:       tmpl.Name,
		ContainerName:      opts.Name,
		CertInstallCommand: certInstallCommand,
	}
}

// processComposeTemplate processes docker-compose.yml.tmpl with the given data.
func (g *ComposeGenerator) processComposeTemplate(templatePath string, data TemplateData) (string, error) {
	tmplFile := filepath.Join(templatePath, "docker-compose.yml.tmpl")
	return processTemplate(tmplFile, data)
}

// processFilterTemplate processes filter.py.tmpl with the given data.
func (g *ComposeGenerator) processFilterTemplate(templatePath string, data TemplateData) (string, error) {
	tmplFile := filepath.Join(templatePath, "filter.py.tmpl")
	return processTemplate(tmplFile, data)
}

// loadDockerfileProxy loads the static Dockerfile.proxy from the template directory.
func (g *ComposeGenerator) loadDockerfileProxy(templatePath string) (string, error) {
	dockerfilePath := filepath.Join(templatePath, "Dockerfile.proxy")
	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Fallback to generated content for backward compatibility
			return generateDockerfileProxy(), nil
		}
		return "", err
	}
	return string(content), nil
}

// generateDockerfileProxy creates the Dockerfile for the mitmproxy sidecar.
// Used as fallback when template doesn't have Dockerfile.proxy.
// NOTE: Do NOT set USER mitmproxy in this Dockerfile. The mitmproxy base image
// has an entrypoint script that runs usermod and gosu (both require root) before
// switching to the mitmproxy user. Setting USER breaks the entrypoint.
func generateDockerfileProxy() string {
	return `FROM mitmproxy/mitmproxy:latest

# Copy filter script into container
COPY filter.py /home/mitmproxy/filter.py

# Expose proxy port
EXPOSE 8080
`
}

// processTemplate reads a template file and processes it with the given data.
func processTemplate(tmplPath string, data any) (string, error) {
	content, err := os.ReadFile(tmplPath)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New(filepath.Base(tmplPath)).Parse(string(content))
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// ProcessDevcontainerTemplate processes devcontainer.json.tmpl with the given data.
// This is used by DevcontainerGenerator when writing compose mode files.
func ProcessDevcontainerTemplate(templatePath string, data TemplateData) (string, error) {
	tmplFile := filepath.Join(templatePath, "devcontainer.json.tmpl")
	return processTemplate(tmplFile, data)
}

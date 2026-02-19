// pattern: Functional Core

package container

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"devagent/internal/config"
	"devagent/internal/logging"
)

// ComposeResult holds the generated compose configuration files.
type ComposeResult struct {
	TemplateData TemplateData
}

// TemplateData holds all values for template placeholder substitution.
// Only instance-specific values are substituted - everything else is hardcoded in templates.
type TemplateData struct {
	ProjectPath     string // Absolute path to project
	ProjectName     string // Base name of project directory
	WorkspaceFolder string // /workspaces/{{.ProjectName}}
	ClaudeTokenPath string // Host path to Claude OAuth token file (absolute)
	GitHubTokenPath string // Host path to GitHub token file (absolute), /dev/null if missing
	TemplateName    string // Template name (e.g., "basic")
	ContainerName   string // Container name for devcontainer.json
	ProxyImage      string // Docker image for mitmproxy sidecar (default: mitmproxy/mitmproxy:latest)
	ProxyPort       string // Port mitmproxy listens on (default: 8080)
	RemoteUser      string // User for devcontainer exec commands (default: vscode)
	ProxyLogPath    string // Container path for proxy request logs (default: /opt/devagent-proxy/logs/requests.jsonl)
}

// ComposeGenerator creates docker-compose.yml and related files for container orchestration.
type ComposeGenerator struct {
	templates []config.Template
	logger    *logging.ScopedLogger
}

// NewComposeGenerator creates a new generator with the given templates.
func NewComposeGenerator(templates []config.Template, logger *logging.ScopedLogger) *ComposeGenerator {
	return &ComposeGenerator{
		templates: templates,
		logger:    logger,
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

// Generate creates docker-compose.yml content.
// Returns ComposeResult with template data for file writing.
func (g *ComposeGenerator) Generate(opts ComposeOptions) (*ComposeResult, error) {
	// Find template
	tmpl := g.GetTemplate(opts.Template)
	if tmpl == nil {
		return nil, fmt.Errorf("template not found: %s", opts.Template)
	}

	// Build and return template data
	data := g.buildTemplateData(opts, tmpl)
	return &ComposeResult{
		TemplateData: data,
	}, nil
}

// buildTemplateData constructs TemplateData from options and template.
func (g *ComposeGenerator) buildTemplateData(opts ComposeOptions, tmpl *config.Template) TemplateData {
	projectName := filepath.Base(opts.ProjectPath)

	// Ensure Claude token exists (non-blocking on error).
	// Falls back to /dev/null so Docker doesn't create an empty directory.
	tokenPath, _ := ensureClaudeToken()
	if tokenPath == "" {
		tokenPath = "/dev/null"
	}

	// Read GitHub token (non-blocking on error).
	// Falls back to /dev/null so Docker doesn't create an empty directory.
	ghTokenPath, _ := ensureGitHubToken()
	if ghTokenPath == "" {
		if g.logger != nil {
			g.logger.Warn("GitHub token not found, gh CLI will not be authenticated", "expected_path", "~/.config/github/token")
		}
		ghTokenPath = "/dev/null"
	}

	return TemplateData{
		ProjectPath:     opts.ProjectPath,
		ProjectName:     projectName,
		WorkspaceFolder: fmt.Sprintf("/workspaces/%s", projectName),
		ClaudeTokenPath: tokenPath,
		GitHubTokenPath: ghTokenPath,
		TemplateName:    tmpl.Name,
		ContainerName:   opts.Name,
		ProxyImage:      "mitmproxy/mitmproxy:latest",
		ProxyPort:       "8080",
		RemoteUser:      DefaultRemoteUser,
		ProxyLogPath:    "/opt/devagent-proxy/logs/requests.jsonl",
	}
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

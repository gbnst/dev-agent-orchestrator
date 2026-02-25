// pattern: Functional Core

package container

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	cfg       *config.Config
	templates []config.Template
	logger    *logging.ScopedLogger
}

// NewComposeGenerator creates a new generator with the given config and templates.
func NewComposeGenerator(cfg *config.Config, templates []config.Template, logger *logging.ScopedLogger) *ComposeGenerator {
	return &ComposeGenerator{
		cfg:       cfg,
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

	// Build and validate template data
	data := g.buildTemplateData(opts, tmpl)
	if err := validateTemplateData(data); err != nil {
		return nil, fmt.Errorf("invalid template data: %w", err)
	}
	return &ComposeResult{
		TemplateData: data,
	}, nil
}

// buildTemplateData constructs TemplateData from options and template.
func (g *ComposeGenerator) buildTemplateData(opts ComposeOptions, tmpl *config.Template) TemplateData {
	projectName := filepath.Base(opts.ProjectPath)

	// Resolve and ensure Claude token (non-blocking on error).
	// Falls back to /dev/null so Docker doesn't create an empty directory.
	tokenPath, _ := ensureClaudeToken(g.cfg.ResolveTokenPath(g.cfg.ClaudeTokenPath))
	if tokenPath == "" {
		tokenPath = "/dev/null"
	}

	// Resolve and read GitHub token (non-blocking on error).
	// Falls back to /dev/null so Docker doesn't create an empty directory.
	ghTokenPath, _ := ensureGitHubToken(g.cfg.ResolveTokenPath(g.cfg.GitHubTokenPath))
	if ghTokenPath == "" {
		if g.logger != nil && g.cfg.GitHubTokenPath != "" {
			g.logger.Warn("GitHub token not found", "path", g.cfg.GitHubTokenPath)
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

// validateTemplateData checks that template data values don't contain characters
// that could produce malformed YAML when substituted into templates.
func validateTemplateData(data TemplateData) error {
	// Characters that are meaningful in YAML values and could cause injection
	// when substituted unquoted into templates.
	const yamlSpecial = `:{}[]|>&*!%#@`
	check := func(name, value string) error {
		if strings.ContainsAny(value, yamlSpecial) {
			return fmt.Errorf("%s contains YAML-special characters: %q", name, value)
		}
		return nil
	}
	// ProjectPath is allowed to contain colons (Windows drive letters) and other
	// path chars — templates should quote it. Validate the names that appear
	// unquoted in YAML keys/values.
	if err := check("ContainerName", data.ContainerName); err != nil {
		return err
	}
	if err := check("ProjectName", data.ProjectName); err != nil {
		return err
	}
	return nil
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

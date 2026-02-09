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

// certInstallCommand is the shell command that waits for the mitmproxy CA cert
// to become available, then installs it into the system trust store.
const certInstallCommand = "timeout=30; while [ ! -f /tmp/mitmproxy-certs/mitmproxy-ca-cert.pem ] && [ $timeout -gt 0 ]; do sleep 1; timeout=$((timeout-1)); done && sudo cp /tmp/mitmproxy-certs/mitmproxy-ca-cert.pem /usr/local/share/ca-certificates/mitmproxy-ca-cert.crt && sudo update-ca-certificates"

// ComposeResult holds the generated compose configuration files.
type ComposeResult struct {
	ComposeYAML string // docker-compose.yml content
}

// TemplateData holds all values for template placeholder substitution.
// Only instance-specific values are substituted - everything else is hardcoded in templates.
type TemplateData struct {
	ProjectPath        string // Absolute path to project
	ProjectName        string // Base name of project directory
	WorkspaceFolder    string // /workspaces/{{.ProjectName}}
	ClaudeTokenPath    string // Host path to Claude OAuth token file (absolute)
	GitHubTokenPath    string // Host path to GitHub token file (absolute), /dev/null if missing
	TemplateName       string // Template name (e.g., "basic")
	ContainerName      string // Container name for devcontainer.json
	CertInstallCommand string // Command to wait for, copy, and trust mitmproxy CA cert
	ProxyImage         string // Docker image for mitmproxy sidecar (default: mitmproxy/mitmproxy:latest)
	ProxyPort          string // Port mitmproxy listens on (default: 8080)
	RemoteUser         string // User for devcontainer exec commands (default: vscode)
	ProxyLogPath       string // Container path for proxy request logs (default: /opt/devagent-proxy/logs/requests.jsonl)
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
// Returns ComposeResult with generated compose configuration.
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

	return &ComposeResult{
		ComposeYAML: composeYAML,
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
		ProjectPath:        opts.ProjectPath,
		ProjectName:        projectName,
		WorkspaceFolder:    fmt.Sprintf("/workspaces/%s", projectName),
		ClaudeTokenPath:    tokenPath,
		GitHubTokenPath:    ghTokenPath,
		TemplateName:       tmpl.Name,
		ContainerName:      opts.Name,
		CertInstallCommand: certInstallCommand,
		ProxyImage:         "mitmproxy/mitmproxy:latest",
		ProxyPort:          "8080",
		RemoteUser:         DefaultRemoteUser,
		ProxyLogPath:       "/opt/devagent-proxy/logs/requests.jsonl",
	}
}

// processComposeTemplate processes docker-compose.yml.tmpl with the given data.
func (g *ComposeGenerator) processComposeTemplate(templatePath string, data TemplateData) (string, error) {
	tmplFile := filepath.Join(templatePath, "docker-compose.yml.tmpl")
	return processTemplate(tmplFile, data)
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

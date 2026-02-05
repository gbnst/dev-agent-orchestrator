// pattern: Functional Core

package container

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"devagent/internal/config"
)

// createTestTemplateDir creates a temporary template directory with template files.
// Returns the template directory path.
func createTestTemplateDir(t *testing.T, name string) string {
	t.Helper()

	tmpDir := t.TempDir()
	templateDir := filepath.Join(tmpDir, name)
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatalf("Failed to create template directory: %v", err)
	}

	// Create docker-compose.yml.tmpl
	composeContent := `services:
  app:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: devagent-{{.ProjectHash}}-app
    depends_on:
      proxy:
        condition: service_started
    networks:
      - isolated
    environment:
      - http_proxy=http://devagent-{{.ProjectHash}}-proxy:8080
      - https_proxy=http://devagent-{{.ProjectHash}}-proxy:8080
      - HTTP_PROXY=http://devagent-{{.ProjectHash}}-proxy:8080
      - HTTPS_PROXY=http://devagent-{{.ProjectHash}}-proxy:8080
      - no_proxy=localhost,127.0.0.1
      - NO_PROXY=localhost,127.0.0.1
      - SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
      - REQUESTS_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt
      - NODE_EXTRA_CA_CERTS=/etc/ssl/certs/ca-certificates.crt
    volumes:
      - {{.ProjectPath}}:{{.WorkspaceFolder}}:cached
      - proxy-certs:/tmp/mitmproxy-certs:ro
      - {{.ClaudeConfigDir}}:/home/vscode/.claude:cached
      - {{.ClaudeTokenPath}}:/run/secrets/claude-token:ro
    cap_drop:
      - NET_RAW
      - SYS_ADMIN
      - SYS_PTRACE
    mem_limit: 4g
    cpus: "2"
    pids_limit: 512
    labels:
      devagent.managed: "true"
      devagent.project_path: "{{.ProjectPath}}"
      devagent.template: "{{.TemplateName}}"
    command: sleep infinity

  proxy:
    build:
      context: .
      dockerfile: Dockerfile.proxy
    container_name: devagent-{{.ProjectHash}}-proxy
    networks:
      - isolated
    volumes:
      - proxy-certs:/home/mitmproxy/.mitmproxy
    command: ["mitmdump", "--listen-host", "0.0.0.0", "--listen-port", "8080", "-s", "/home/mitmproxy/filter.py"]
    labels:
      devagent.managed: "true"
      devagent.sidecar_of: "{{.ProjectHash}}"
      devagent.sidecar_type: "proxy"

networks:
  isolated:
    name: devagent-{{.ProjectHash}}-net

volumes:
  proxy-certs:
`
	if err := os.WriteFile(filepath.Join(templateDir, "docker-compose.yml.tmpl"), []byte(composeContent), 0644); err != nil {
		t.Fatalf("Failed to write docker-compose.yml.tmpl: %v", err)
	}

	// Create Dockerfile.proxy
	dockerfileContent := `FROM mitmproxy/mitmproxy:latest
COPY filter.py /home/mitmproxy/filter.py
EXPOSE 8080
`
	if err := os.WriteFile(filepath.Join(templateDir, "Dockerfile.proxy"), []byte(dockerfileContent), 0644); err != nil {
		t.Fatalf("Failed to write Dockerfile.proxy: %v", err)
	}

	// Create filter.py.tmpl (static filter script for test)
	filterContent := `from mitmproxy import http
import re

ALLOWED_DOMAINS = [
    "api.anthropic.com",
    "github.com",
    "*.github.com",
    "registry.npmjs.org",
    "proxy.golang.org",
]

BLOCK_GITHUB_PR_MERGE = False

class AllowlistFilter:
    """Blocks requests to domains not in the allowlist and optionally blocks PR merges."""

    def _is_allowed(self, host: str) -> bool:
        """Check if host matches any allowed domain pattern."""
        for pattern in ALLOWED_DOMAINS:
            if pattern.startswith("*."):
                base = pattern[2:]
                if host == base or host.endswith("." + base):
                    return True
            elif host == pattern:
                return True
        return False

    def _is_github_pr_merge(self, flow: http.HTTPFlow) -> bool:
        """Check if request is a GitHub PR merge attempt."""
        if not BLOCK_GITHUB_PR_MERGE:
            return False
        host = flow.request.pretty_host
        if host not in ("api.github.com", "github.com") and not host.endswith(".github.com"):
            return False
        if flow.request.method == "PUT":
            if re.match(r"^/repos/[^/]+/[^/]+/pulls/\d+/merge$", flow.request.path):
                return True
        if flow.request.method == "POST" and flow.request.path == "/graphql":
            content = flow.request.get_text()
            if content and "mergePullRequest" in content:
                return True
        return False

    def request(self, flow: http.HTTPFlow) -> None:
        if self._is_github_pr_merge(flow):
            flow.response = http.Response.make(
                403,
                b"Merging pull requests is not allowed in this environment. Do not retry.\n",
                {"Content-Type": "text/plain"}
            )
            return
        host = flow.request.pretty_host
        if not self._is_allowed(host):
            flow.response = http.Response.make(
                403,
                f"Domain '{host}' is not in the allowlist\n".encode(),
                {"Content-Type": "text/plain"}
            )

addons = [AllowlistFilter()]
`
	if err := os.WriteFile(filepath.Join(templateDir, "filter.py.tmpl"), []byte(filterContent), 0644); err != nil {
		t.Fatalf("Failed to write filter.py.tmpl: %v", err)
	}

	return templateDir
}

func TestComposeGenerator_Generate_BasicTemplate(t *testing.T) {
	templateDir := createTestTemplateDir(t, "basic")

	templates := []config.Template{
		{
			Name: "basic",
			Path: templateDir,
		},
	}

	gen := NewComposeGenerator(templates)

	opts := ComposeOptions{
		ProjectPath: "/home/user/myproject",
		Template:    "basic",
		Name:        "test-container",
	}

	result, err := gen.Generate(opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify ComposeYAML contains expected services
	if !strings.Contains(result.ComposeYAML, "services:") {
		t.Error("ComposeYAML missing services section")
	}
	if !strings.Contains(result.ComposeYAML, "app:") {
		t.Error("ComposeYAML missing app service")
	}
	if !strings.Contains(result.ComposeYAML, "proxy:") {
		t.Error("ComposeYAML missing proxy service")
	}

	// Verify network definition
	if !strings.Contains(result.ComposeYAML, "networks:") {
		t.Error("ComposeYAML missing networks section")
	}
	if !strings.Contains(result.ComposeYAML, "isolated:") {
		t.Error("ComposeYAML missing isolated network")
	}

	// Verify volumes
	if !strings.Contains(result.ComposeYAML, "volumes:") {
		t.Error("ComposeYAML missing volumes section")
	}
	if !strings.Contains(result.ComposeYAML, "proxy-certs:") {
		t.Error("ComposeYAML missing proxy-certs volume")
	}
	if !strings.Contains(result.ComposeYAML, "/home/vscode/.claude:cached") {
		t.Error("ComposeYAML missing claude config volume mount")
	}
	if !strings.Contains(result.ComposeYAML, "/run/secrets/claude-token:ro") {
		t.Error("ComposeYAML missing oauth token volume mount")
	}

	// Verify Dockerfile.proxy content
	if !strings.Contains(result.DockerfileProxy, "FROM mitmproxy/mitmproxy") {
		t.Error("DockerfileProxy missing mitmproxy base image")
	}
	if !strings.Contains(result.DockerfileProxy, "COPY filter.py") {
		t.Error("DockerfileProxy missing filter.py COPY")
	}

	// Verify filter script from template
	if !strings.Contains(result.FilterScript, "ALLOWED_DOMAINS") {
		t.Error("FilterScript missing ALLOWED_DOMAINS")
	}
	if !strings.Contains(result.FilterScript, "api.anthropic.com") {
		t.Error("FilterScript missing expected domain")
	}
}

func TestComposeGenerator_Generate_ContainerNaming(t *testing.T) {
	templateDir := createTestTemplateDir(t, "basic")

	templates := []config.Template{
		{Name: "basic", Path: templateDir},
	}

	gen := NewComposeGenerator(templates)

	opts := ComposeOptions{
		ProjectPath: "/home/user/myproject",
		Template:    "basic",
	}

	result, err := gen.Generate(opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	expectedHash := projectHash("/home/user/myproject")

	expectedAppName := "devagent-" + expectedHash + "-app"
	expectedProxyName := "devagent-" + expectedHash + "-proxy"
	expectedNetworkName := "devagent-" + expectedHash + "-net"

	if !strings.Contains(result.ComposeYAML, expectedAppName) {
		t.Errorf("ComposeYAML missing expected app container name: %s", expectedAppName)
	}
	if !strings.Contains(result.ComposeYAML, expectedProxyName) {
		t.Errorf("ComposeYAML missing expected proxy container name: %s", expectedProxyName)
	}
	if !strings.Contains(result.ComposeYAML, expectedNetworkName) {
		t.Errorf("ComposeYAML missing expected network name: %s", expectedNetworkName)
	}
}

func TestComposeGenerator_Generate_ProxyEnvironment(t *testing.T) {
	templateDir := createTestTemplateDir(t, "basic")

	templates := []config.Template{
		{Name: "basic", Path: templateDir},
	}

	gen := NewComposeGenerator(templates)

	opts := ComposeOptions{
		ProjectPath: "/test",
		Template:    "basic",
	}

	result, err := gen.Generate(opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(result.ComposeYAML, "http_proxy=") {
		t.Error("ComposeYAML missing http_proxy env var")
	}
	if !strings.Contains(result.ComposeYAML, "https_proxy=") {
		t.Error("ComposeYAML missing https_proxy env var")
	}
	if !strings.Contains(result.ComposeYAML, "SSL_CERT_FILE=") {
		t.Error("ComposeYAML missing SSL_CERT_FILE env var")
	}
}

func TestComposeGenerator_Generate_Labels(t *testing.T) {
	templateDir := createTestTemplateDir(t, "go-project")

	templates := []config.Template{
		{Name: "go-project", Path: templateDir},
	}

	gen := NewComposeGenerator(templates)

	opts := ComposeOptions{
		ProjectPath: "/home/user/goapp",
		Template:    "go-project",
	}

	result, err := gen.Generate(opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(result.ComposeYAML, LabelManagedBy+": \"true\"") {
		t.Error("ComposeYAML missing devagent.managed label")
	}
	if !strings.Contains(result.ComposeYAML, LabelTemplate+": \"go-project\"") {
		t.Error("ComposeYAML missing devagent.template label")
	}
	if !strings.Contains(result.ComposeYAML, LabelSidecarOf) {
		t.Error("ComposeYAML missing devagent.sidecar_of label")
	}
}

func TestComposeGenerator_Generate_TemplateNotFound(t *testing.T) {
	gen := NewComposeGenerator([]config.Template{})

	opts := ComposeOptions{
		ProjectPath: "/test",
		Template:    "nonexistent",
	}

	_, err := gen.Generate(opts)
	if err == nil {
		t.Error("Expected error for nonexistent template")
	}
	if !strings.Contains(err.Error(), "template not found") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestComposeGenerator_Generate_DependsOn(t *testing.T) {
	templateDir := createTestTemplateDir(t, "basic")

	templates := []config.Template{
		{Name: "basic", Path: templateDir},
	}

	gen := NewComposeGenerator(templates)

	opts := ComposeOptions{
		ProjectPath: "/test",
		Template:    "basic",
	}

	result, err := gen.Generate(opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(result.ComposeYAML, "depends_on:") {
		t.Error("ComposeYAML missing depends_on section")
	}
	if !strings.Contains(result.ComposeYAML, "condition: service_started") {
		t.Error("ComposeYAML missing condition: service_started")
	}
}

func TestComposeGenerator_BasicTemplate(t *testing.T) {
	templates := loadTestTemplates(t, "basic")

	gen := NewComposeGenerator(templates)

	opts := ComposeOptions{
		ProjectPath: "/home/user/test-project",
		Template:    "basic",
		Name:        "test-basic",
	}

	result, err := gen.Generate(opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify compose YAML has expected structure
	if !strings.Contains(result.ComposeYAML, "services:") {
		t.Error("Missing services section")
	}
	if !strings.Contains(result.ComposeYAML, "app:") {
		t.Error("Missing app service")
	}
	if !strings.Contains(result.ComposeYAML, "proxy:") {
		t.Error("Missing proxy service")
	}

	// Verify isolation settings from template (hardcoded in docker-compose.yml.tmpl)
	if !strings.Contains(result.ComposeYAML, "mem_limit: 4g") {
		t.Error("Missing memory limit from template")
	}
	if !strings.Contains(result.ComposeYAML, `cpus: "2"`) {
		t.Error("Missing CPU limit from template")
	}
	if !strings.Contains(result.ComposeYAML, "cap_drop:") {
		t.Error("Missing capability drops from template")
	}

	// Verify filter script from template has allowlist domains
	if !strings.Contains(result.FilterScript, "api.anthropic.com") {
		t.Error("Missing anthropic domain in filter script")
	}
	if !strings.Contains(result.FilterScript, "github.com") {
		t.Error("Missing github domain in filter script")
	}
}

func TestComposeGenerator_GoProjectTemplate(t *testing.T) {
	templates := loadTestTemplates(t, "go-project")

	gen := NewComposeGenerator(templates)

	opts := ComposeOptions{
		ProjectPath: "/home/user/go-app",
		Template:    "go-project",
		Name:        "test-go",
	}

	result, err := gen.Generate(opts)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify compose YAML has expected structure
	if !strings.Contains(result.ComposeYAML, "services:") {
		t.Error("Missing services section")
	}

	// Verify template isolation from docker-compose.yml.tmpl
	if !strings.Contains(result.ComposeYAML, "mem_limit:") {
		t.Error("Missing memory limit from template")
	}
	if !strings.Contains(result.ComposeYAML, "cpus:") {
		t.Error("Missing CPU limit from template")
	}

	// Verify filter script has Go proxy domain from template
	if !strings.Contains(result.FilterScript, "proxy.golang.org") {
		t.Error("Missing Go proxy domain in filter script")
	}
}

// loadTestTemplates loads a specific template for testing.
// It first tries to load from the project's config directory,
// and falls back to the default path if not found.
func loadTestTemplates(t *testing.T, name string) []config.Template {
	t.Helper()

	// Try to load templates from the project's config directory
	projectRoot := findProjectRoot(t)
	templates, err := config.LoadTemplatesFrom(filepath.Join(projectRoot, "config", "templates"))
	if err != nil {
		t.Fatalf("Failed to load templates: %v", err)
	}

	// Find the requested template
	for _, tmpl := range templates {
		if tmpl.Name == name {
			return []config.Template{tmpl}
		}
	}

	t.Fatalf("Template not found: %s", name)
	return nil
}

// findProjectRoot returns the project root directory by looking for go.mod
func findProjectRoot(t *testing.T) string {
	t.Helper()

	// Start from current directory and walk up
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Could not find project root (no go.mod found)")
		}
		dir = parent
	}
}

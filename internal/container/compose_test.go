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

	// Create docker-compose.yml.tmpl (matches new template structure)
	composeContent := `services:
  app:
    build:
      context: .
      dockerfile: Dockerfile
    depends_on:
      proxy:
        condition: service_started
    networks:
      - isolated
    environment:
      - http_proxy=http://proxy:8080
      - https_proxy=http://proxy:8080
      - HTTP_PROXY=http://proxy:8080
      - HTTPS_PROXY=http://proxy:8080
      - no_proxy=localhost,127.0.0.1
      - NO_PROXY=localhost,127.0.0.1
      - SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
      - REQUESTS_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt
      - NODE_EXTRA_CA_CERTS=/etc/ssl/certs/ca-certificates.crt
    volumes:
      - {{.ProjectPath}}:{{.WorkspaceFolder}}:cached
      - proxy-certs:/tmp/mitmproxy-certs:ro
      - {{.ProjectPath}}/.devcontainer/home/vscode:/home/vscode:cached
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
    image: mitmproxy/mitmproxy:latest
    networks:
      - isolated
    volumes:
      - proxy-certs:/home/mitmproxy/.mitmproxy
      - {{.ProjectPath}}/.devcontainer/proxy:/opt/devagent-proxy
    command: ["mitmdump", "--listen-host", "0.0.0.0", "--listen-port", "8080", "-s", "/opt/devagent-proxy/filter.py"]
    labels:
      devagent.managed: "true"
      devagent.sidecar_type: "proxy"

networks:
  isolated:

volumes:
  proxy-certs:
`
	if err := os.WriteFile(filepath.Join(templateDir, "docker-compose.yml.tmpl"), []byte(composeContent), 0644); err != nil {
		t.Fatalf("Failed to write docker-compose.yml.tmpl: %v", err)
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

	// Verify volume mounts
	if !strings.Contains(result.ComposeYAML, "/home/vscode:cached") {
		t.Error("ComposeYAML missing home/vscode volume mount")
	}
	if !strings.Contains(result.ComposeYAML, "/run/secrets/claude-token:ro") {
		t.Error("ComposeYAML missing oauth token volume mount")
	}

	// Verify proxy uses image instead of build
	if !strings.Contains(result.ComposeYAML, "image: mitmproxy/mitmproxy:latest") {
		t.Error("ComposeYAML missing proxy image directive")
	}
	if strings.Contains(result.ComposeYAML, "dockerfile: Dockerfile.proxy") {
		t.Error("ComposeYAML should not reference Dockerfile.proxy")
	}
}

func TestComposeGenerator_Generate_NoHardcodedContainerNames(t *testing.T) {
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

	// Verify no hardcoded container_name directives (compose auto-generates names)
	if strings.Contains(result.ComposeYAML, "container_name:") {
		t.Error("ComposeYAML should not contain container_name (let compose auto-name)")
	}

	// Verify proxy env vars use service name "proxy" instead of hash-based names
	if !strings.Contains(result.ComposeYAML, "http_proxy=http://proxy:") {
		t.Error("ComposeYAML should use 'proxy' service name in proxy env vars")
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
	if !strings.Contains(result.ComposeYAML, LabelSidecarType+": \"proxy\"") {
		t.Error("ComposeYAML missing devagent.sidecar_type label")
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

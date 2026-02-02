package container

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateFilterScript(t *testing.T) {
	tests := []struct {
		name      string
		allowlist []string
		wantParts []string // Strings that should appear in the output
	}{
		{
			name:      "empty allowlist",
			allowlist: []string{},
			wantParts: []string{
				"ALLOWED_DOMAINS = [",
				"class AllowlistFilter:",
			},
		},
		{
			name:      "single domain",
			allowlist: []string{"github.com"},
			wantParts: []string{
				`"github.com",`,
			},
		},
		{
			name:      "multiple domains",
			allowlist: []string{"github.com", "api.anthropic.com", "*.npmjs.org"},
			wantParts: []string{
				`"github.com",`,
				`"api.anthropic.com",`,
				`"*.npmjs.org",`,
			},
		},
		{
			name:      "wildcard domain",
			allowlist: []string{"*.github.com"},
			wantParts: []string{
				`"*.github.com",`,
				`pattern.startswith("*.")`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateFilterScript(tt.allowlist)

			// Verify script structure
			if !strings.Contains(got, "from mitmproxy import http") {
				t.Error("script should import mitmproxy.http")
			}
			if !strings.Contains(got, "addons = [AllowlistFilter()]") {
				t.Error("script should export addons")
			}
			if !strings.Contains(got, "flow.response = http.Response.make") {
				t.Error("script should block with 403 response")
			}

			// Verify expected content
			for _, part := range tt.wantParts {
				if !strings.Contains(got, part) {
					t.Errorf("script should contain %q", part)
				}
			}
		})
	}
}

func TestGenerateIgnoreHostsPattern(t *testing.T) {
	tests := []struct {
		name        string
		passthrough []string
		want        string
	}{
		{
			name:        "empty passthrough",
			passthrough: []string{},
			want:        "",
		},
		{
			name:        "single domain",
			passthrough: []string{"example.com"},
			want:        `^(.+\.)?example\.com:`,
		},
		{
			name:        "multiple domains",
			passthrough: []string{"example.com", "test.org"},
			want:        `^(.+\.)?example\.com:|^(.+\.)?test\.org:`,
		},
		{
			name:        "wildcard domain",
			passthrough: []string{"*.example.com"},
			want:        `^(.+\.)?example\.com:`,
		},
		{
			name:        "domain with dots",
			passthrough: []string{"api.example.com"},
			want:        `^(.+\.)?api\.example\.com:`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateIgnoreHostsPattern(tt.passthrough)
			if got != tt.want {
				t.Errorf("GenerateIgnoreHostsPattern() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetProxyConfigDir(t *testing.T) {
	// Redirect data directory to temp for this test to avoid polluting real user data
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	projectPath := "/some/project/path"
	dir, err := GetProxyConfigDir(projectPath)
	if err != nil {
		t.Fatalf("GetProxyConfigDir() error = %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("directory was not created: %s", dir)
	}

	// Verify it's under proxy-configs
	if !strings.Contains(dir, "proxy-configs") {
		t.Errorf("directory should be under proxy-configs: %s", dir)
	}

	// Verify consistent hashing (same project = same dir)
	dir2, err := GetProxyConfigDir(projectPath)
	if err != nil {
		t.Fatalf("GetProxyConfigDir() second call error = %v", err)
	}
	if dir != dir2 {
		t.Errorf("same project path should return same dir: %s != %s", dir, dir2)
	}

	// Verify different projects get different dirs
	dir3, err := GetProxyConfigDir("/different/project")
	if err != nil {
		t.Fatalf("GetProxyConfigDir() different project error = %v", err)
	}
	if dir == dir3 {
		t.Errorf("different projects should get different dirs")
	}
}

func TestGetProxyCertDir(t *testing.T) {
	// Redirect data directory to temp for this test to avoid polluting real user data
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	projectPath := "/some/project/path"
	dir, err := GetProxyCertDir(projectPath)
	if err != nil {
		t.Fatalf("GetProxyCertDir() error = %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("directory was not created: %s", dir)
	}

	// Verify it's under proxy-certs
	if !strings.Contains(dir, "proxy-certs") {
		t.Errorf("directory should be under proxy-certs: %s", dir)
	}

	// Verify consistent hashing (same project = same dir)
	dir2, err := GetProxyCertDir(projectPath)
	if err != nil {
		t.Fatalf("GetProxyCertDir() second call error = %v", err)
	}
	if dir != dir2 {
		t.Errorf("same project path should return same dir: %s != %s", dir, dir2)
	}

	// Verify different projects get different dirs
	dir3, err := GetProxyCertDir("/different/project")
	if err != nil {
		t.Fatalf("GetProxyCertDir() different project error = %v", err)
	}
	if dir == dir3 {
		t.Errorf("different projects should get different dirs")
	}
}

func TestWriteFilterScript(t *testing.T) {
	// Redirect data directory to temp for this test to avoid polluting real user data
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	projectPath := "/some/project/path"  // Can use fixed path now since data dir is redirected
	allowlist := []string{"github.com", "api.anthropic.com"}

	scriptPath, err := WriteFilterScript(projectPath, allowlist)
	if err != nil {
		t.Fatalf("WriteFilterScript() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		t.Errorf("script file was not created: %s", scriptPath)
	}

	// Verify file name
	if filepath.Base(scriptPath) != "filter.py" {
		t.Errorf("script should be named filter.py, got: %s", filepath.Base(scriptPath))
	}

	// Verify content
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("failed to read script: %v", err)
	}

	if !strings.Contains(string(content), `"github.com",`) {
		t.Error("script should contain github.com")
	}
	if !strings.Contains(string(content), `"api.anthropic.com",`) {
		t.Error("script should contain api.anthropic.com")
	}
}

func TestGetProxyCACertPath(t *testing.T) {
	// Redirect data directory to temp for this test to avoid polluting real user data
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	projectPath := "/some/project/path"
	certPath, err := GetProxyCACertPath(projectPath)
	if err != nil {
		t.Fatalf("GetProxyCACertPath() error = %v", err)
	}

	if filepath.Base(certPath) != "mitmproxy-ca-cert.pem" {
		t.Errorf("cert filename = %q, want mitmproxy-ca-cert.pem", filepath.Base(certPath))
	}

	// Verify it's under proxy-certs
	if !strings.Contains(certPath, "proxy-certs") {
		t.Errorf("cert path should be under proxy-certs: %s", certPath)
	}
}

func TestProxyCertExists(t *testing.T) {
	// Redirect data directory to temp for this test to avoid polluting real user data
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	projectPath := "/some/project/path"

	// Initially should not exist
	exists, err := ProxyCertExists(projectPath)
	if err != nil {
		t.Fatalf("ProxyCertExists() error = %v", err)
	}
	if exists {
		t.Error("cert should not exist initially")
	}

	// Create the cert file
	certPath, _ := GetProxyCACertPath(projectPath)
	if err := os.WriteFile(certPath, []byte("fake cert"), 0644); err != nil {
		t.Fatalf("failed to create cert file: %v", err)
	}

	// Now should exist
	exists, err = ProxyCertExists(projectPath)
	if err != nil {
		t.Fatalf("ProxyCertExists() after create error = %v", err)
	}
	if !exists {
		t.Error("cert should exist after creation")
	}
}

func TestCleanupProxyConfigs(t *testing.T) {
	// Redirect data directory to temp for this test to avoid polluting real user data
	tmpDataDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDataDir)

	projectPath := "/some/project/path"

	// Create config and cert directories
	configDir, err := GetProxyConfigDir(projectPath)
	if err != nil {
		t.Fatalf("GetProxyConfigDir() error = %v", err)
	}
	certDir, err := GetProxyCertDir(projectPath)
	if err != nil {
		t.Fatalf("GetProxyCertDir() error = %v", err)
	}

	// Verify directories exist
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Fatal("config directory should exist")
	}
	if _, err := os.Stat(certDir); os.IsNotExist(err) {
		t.Fatal("cert directory should exist")
	}

	// Cleanup
	if err := CleanupProxyConfigs(projectPath); err != nil {
		t.Fatalf("CleanupProxyConfigs() error = %v", err)
	}

	// Verify directories are removed
	if _, err := os.Stat(configDir); !os.IsNotExist(err) {
		t.Error("config directory should be removed")
	}
	if _, err := os.Stat(certDir); !os.IsNotExist(err) {
		t.Error("cert directory should be removed")
	}

	// Cleanup again should not error (idempotent)
	if err := CleanupProxyConfigs(projectPath); err != nil {
		t.Errorf("CleanupProxyConfigs() second call error = %v", err)
	}
}

func TestParseAllowlistFromScript(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "empty content",
			content: "",
			want:    []string{},
		},
		{
			name: "single domain",
			content: `ALLOWED_DOMAINS = [
    "github.com",
]`,
			want: []string{"github.com"},
		},
		{
			name: "multiple domains",
			content: `ALLOWED_DOMAINS = [
    "github.com",
    "api.anthropic.com",
    "*.npmjs.org",
]`,
			want: []string{"github.com", "api.anthropic.com", "*.npmjs.org"},
		},
		{
			name: "full filter script",
			content: `from mitmproxy import http

# Allowlist of permitted domains
ALLOWED_DOMAINS = [
    "api.anthropic.com",
    "github.com",
    "*.github.com",
]

class AllowlistFilter:
    pass`,
			want: []string{"api.anthropic.com", "github.com", "*.github.com"},
		},
		{
			name: "no ALLOWED_DOMAINS marker",
			content: `some other content
without the marker`,
			want: []string{},
		},
		{
			name: "empty array",
			content: `ALLOWED_DOMAINS = [
]`,
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAllowlistFromScript(tt.content)
			if len(got) != len(tt.want) {
				t.Errorf("parseAllowlistFromScript() returned %d domains, want %d", len(got), len(tt.want))
				t.Errorf("got: %v, want: %v", got, tt.want)
				return
			}
			for i, domain := range got {
				if domain != tt.want[i] {
					t.Errorf("parseAllowlistFromScript()[%d] = %q, want %q", i, domain, tt.want[i])
				}
			}
		})
	}
}

func TestReadAllowlistFromFilterScript(t *testing.T) {
	// Redirect data directory to temp for this test
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	projectPath := "/test/project"

	t.Run("returns nil for non-existent file", func(t *testing.T) {
		domains, err := ReadAllowlistFromFilterScript("/nonexistent/project")
		if err != nil {
			t.Errorf("ReadAllowlistFromFilterScript() error = %v", err)
		}
		if domains != nil {
			t.Errorf("ReadAllowlistFromFilterScript() = %v, want nil", domains)
		}
	})

	t.Run("reads allowlist from existing file", func(t *testing.T) {
		// Write a filter script
		configDir, err := GetProxyConfigDir(projectPath)
		if err != nil {
			t.Fatalf("GetProxyConfigDir() error = %v", err)
		}

		scriptContent := `ALLOWED_DOMAINS = [
    "github.com",
    "api.anthropic.com",
]`
		scriptPath := filepath.Join(configDir, "filter.py")
		if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
			t.Fatalf("Failed to write filter script: %v", err)
		}

		domains, err := ReadAllowlistFromFilterScript(projectPath)
		if err != nil {
			t.Errorf("ReadAllowlistFromFilterScript() error = %v", err)
		}
		if len(domains) != 2 {
			t.Errorf("ReadAllowlistFromFilterScript() returned %d domains, want 2", len(domains))
		}
		if domains[0] != "github.com" {
			t.Errorf("ReadAllowlistFromFilterScript()[0] = %q, want %q", domains[0], "github.com")
		}
		if domains[1] != "api.anthropic.com" {
			t.Errorf("ReadAllowlistFromFilterScript()[1] = %q, want %q", domains[1], "api.anthropic.com")
		}
	})
}

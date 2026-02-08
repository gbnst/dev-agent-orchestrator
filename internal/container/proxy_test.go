package container

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

	// Create cert directory
	certDir, err := GetProxyCertDir(projectPath)
	if err != nil {
		t.Fatalf("GetProxyCertDir() error = %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(certDir); os.IsNotExist(err) {
		t.Fatal("cert directory should exist")
	}

	// Cleanup
	if err := CleanupProxyConfigs(projectPath); err != nil {
		t.Fatalf("CleanupProxyConfigs() error = %v", err)
	}

	// Verify cert directory is removed
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
	projectPath := t.TempDir()

	t.Run("returns nil for non-existent file", func(t *testing.T) {
		domains, err := ReadAllowlistFromFilterScript(projectPath)
		if err != nil {
			t.Errorf("ReadAllowlistFromFilterScript() error = %v", err)
		}
		if domains != nil {
			t.Errorf("ReadAllowlistFromFilterScript() = %v, want nil", domains)
		}
	})

	t.Run("reads allowlist from existing file", func(t *testing.T) {
		// Write a filter script to the new location
		proxyDir := filepath.Join(projectPath, ".devcontainer", "proxy")
		if err := os.MkdirAll(proxyDir, 0755); err != nil {
			t.Fatalf("Failed to create proxy dir: %v", err)
		}

		scriptContent := `ALLOWED_DOMAINS = [
    "github.com",
    "api.anthropic.com",
]`
		scriptPath := filepath.Join(proxyDir, "filter.py")
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


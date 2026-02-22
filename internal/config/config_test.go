package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadFullConfig(t *testing.T) {
	// Create temp config file with all fields
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `
theme: latte
runtime: podman
claude_token_path: ~/.claude/.devagent-claude-token
github_token_path: ~/.config/github/token
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}

	// Verify theme
	if cfg.Theme != "latte" {
		t.Errorf("Theme: got %q, want %q", cfg.Theme, "latte")
	}

	// Verify runtime
	if cfg.Runtime != "podman" {
		t.Errorf("Runtime: got %q, want %q", cfg.Runtime, "podman")
	}

	// Verify token paths
	if cfg.ClaudeTokenPath != "~/.claude/.devagent-claude-token" {
		t.Errorf("ClaudeTokenPath: got %q, want %q", cfg.ClaudeTokenPath, "~/.claude/.devagent-claude-token")
	}
	if cfg.GitHubTokenPath != "~/.config/github/token" {
		t.Errorf("GitHubTokenPath: got %q, want %q", cfg.GitHubTokenPath, "~/.config/github/token")
	}
}

func TestDetectedRuntime_ConfiguredValue(t *testing.T) {
	cfg := Config{Runtime: "podman"}
	got := cfg.DetectedRuntime()
	if got != "podman" {
		t.Errorf("DetectedRuntime: got %q, want %q", got, "podman")
	}
}

func TestDetectedRuntime_AutoDetect(t *testing.T) {
	cfg := Config{Runtime: ""}

	// DetectedRuntime should auto-detect; we test using a mock lookup function
	got := cfg.DetectedRuntimeWith(func(name string) (string, error) {
		if name == "docker" {
			return "/usr/bin/docker", nil
		}
		return "", os.ErrNotExist
	})
	if got != "docker" {
		t.Errorf("DetectedRuntime: got %q, want %q", got, "docker")
	}
}

func TestDetectedRuntime_AutoDetectPodman(t *testing.T) {
	cfg := Config{Runtime: ""}

	got := cfg.DetectedRuntimeWith(func(name string) (string, error) {
		if name == "podman" {
			return "/usr/bin/podman", nil
		}
		return "", os.ErrNotExist
	})
	if got != "podman" {
		t.Errorf("DetectedRuntime: got %q, want %q", got, "podman")
	}
}

func TestDetectedRuntime_AutoDetectFallback(t *testing.T) {
	cfg := Config{Runtime: ""}

	// Neither docker nor podman found, falls back to docker
	got := cfg.DetectedRuntimeWith(func(name string) (string, error) {
		return "", os.ErrNotExist
	})
	if got != "docker" {
		t.Errorf("DetectedRuntime fallback: got %q, want %q", got, "docker")
	}
}

func TestDetectedRuntimePath_ConfiguredValue(t *testing.T) {
	cfg := Config{Runtime: "podman"}
	got := cfg.DetectedRuntimePathWith(func(name string) (string, error) {
		if name == "podman" {
			return "/opt/homebrew/bin/podman", nil
		}
		return "", os.ErrNotExist
	})
	if got != "/opt/homebrew/bin/podman" {
		t.Errorf("DetectedRuntimePath: got %q, want %q", got, "/opt/homebrew/bin/podman")
	}
}

func TestDetectedRuntimePath_AutoDetect(t *testing.T) {
	cfg := Config{Runtime: ""}
	got := cfg.DetectedRuntimePathWith(func(name string) (string, error) {
		if name == "docker" {
			return "/usr/local/bin/docker", nil
		}
		return "", os.ErrNotExist
	})
	if got != "/usr/local/bin/docker" {
		t.Errorf("DetectedRuntimePath: got %q, want %q", got, "/usr/local/bin/docker")
	}
}

func TestDetectedRuntimePath_Fallback(t *testing.T) {
	cfg := Config{Runtime: ""}
	// Neither found, falls back to "docker" (without path)
	got := cfg.DetectedRuntimePathWith(func(name string) (string, error) {
		return "", os.ErrNotExist
	})
	if got != "docker" {
		t.Errorf("DetectedRuntimePath fallback: got %q, want %q", got, "docker")
	}
}

func TestValidateRuntime_EmptySkipsValidation(t *testing.T) {
	cfg := Config{Runtime: ""}

	err := cfg.ValidateRuntimeWith(func(name string) (string, error) {
		return "", os.ErrNotExist
	})
	if err != nil {
		t.Errorf("ValidateRuntime: expected nil for empty runtime, got %v", err)
	}
}

func TestValidateRuntime_InvalidRuntime(t *testing.T) {
	cfg := Config{Runtime: "containerd"}

	err := cfg.ValidateRuntimeWith(func(name string) (string, error) {
		return "/usr/bin/" + name, nil
	})
	if err == nil {
		t.Error("ValidateRuntime: expected error for invalid runtime")
	}
	if err.Error() != "runtime must be 'docker' or 'podman', got: containerd" {
		t.Errorf("ValidateRuntime: unexpected error message: %v", err)
	}
}

func TestValidateRuntime_DockerFound(t *testing.T) {
	cfg := Config{Runtime: "docker"}

	err := cfg.ValidateRuntimeWith(func(name string) (string, error) {
		if name == "docker" {
			return "/usr/bin/docker", nil
		}
		return "", os.ErrNotExist
	})
	if err != nil {
		t.Errorf("ValidateRuntime: expected nil for found docker, got %v", err)
	}
}

func TestValidateRuntime_PodmanFound(t *testing.T) {
	cfg := Config{Runtime: "podman"}

	err := cfg.ValidateRuntimeWith(func(name string) (string, error) {
		if name == "podman" {
			return "/usr/bin/podman", nil
		}
		return "", os.ErrNotExist
	})
	if err != nil {
		t.Errorf("ValidateRuntime: expected nil for found podman, got %v", err)
	}
}

func TestValidateRuntime_DockerNotFound(t *testing.T) {
	cfg := Config{Runtime: "docker"}

	err := cfg.ValidateRuntimeWith(func(name string) (string, error) {
		return "", os.ErrNotExist
	})
	if err == nil {
		t.Error("ValidateRuntime: expected error when docker not found")
	}
	if err.Error() != "runtime 'docker' not found in PATH" {
		t.Errorf("ValidateRuntime: unexpected error message: %v", err)
	}
}

func TestValidateRuntime_PodmanNotFound(t *testing.T) {
	cfg := Config{Runtime: "podman"}

	err := cfg.ValidateRuntimeWith(func(name string) (string, error) {
		return "", os.ErrNotExist
	})
	if err == nil {
		t.Error("ValidateRuntime: expected error when podman not found")
	}
	if err.Error() != "runtime 'podman' not found in PATH" {
		t.Errorf("ValidateRuntime: unexpected error message: %v", err)
	}
}

func TestDefaultConfig_LogLevel(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.LogLevel != "info" {
		t.Errorf("DefaultConfig().LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
}

func TestLoadFrom_LogLevel(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	content := []byte("log_level: debug\n")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("cfg.LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
}

func TestLoadFrom_LogLevel_EmptyUsesDefault(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	content := []byte("theme: latte\n") // no log_level
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("cfg.LogLevel = %q, want %q (default)", cfg.LogLevel, "info")
	}
}

func TestDefaultConfig_WebConfig(t *testing.T) {
	cfg := DefaultConfig()

	t.Run("bind defaults to 127.0.0.1", func(t *testing.T) {
		if cfg.Web.Bind != "127.0.0.1" {
			t.Errorf("Web.Bind = %q, want %q", cfg.Web.Bind, "127.0.0.1")
		}
	})

	t.Run("port defaults to 0 (disabled)", func(t *testing.T) {
		if cfg.Web.Port != 0 {
			t.Errorf("Web.Port = %d, want 0", cfg.Web.Port)
		}
	})
}

func TestWebConfig_UnmarshalYAML(t *testing.T) {
	t.Run("parses web section with port and bind", func(t *testing.T) {
		input := []byte(`
web:
  port: 8080
  bind: "0.0.0.0"
`)
		var cfg Config
		if err := yaml.Unmarshal(input, &cfg); err != nil {
			t.Fatalf("yaml.Unmarshal() error = %v", err)
		}
		if cfg.Web.Port != 8080 {
			t.Errorf("Web.Port = %d, want 8080", cfg.Web.Port)
		}
		if cfg.Web.Bind != "0.0.0.0" {
			t.Errorf("Web.Bind = %q, want %q", cfg.Web.Bind, "0.0.0.0")
		}
	})

	t.Run("missing web section leaves zero values", func(t *testing.T) {
		input := []byte("theme: latte\n")
		var cfg Config
		if err := yaml.Unmarshal(input, &cfg); err != nil {
			t.Fatalf("yaml.Unmarshal() error = %v", err)
		}
		if cfg.Web.Port != 0 {
			t.Errorf("Web.Port = %d, want 0 when web section absent", cfg.Web.Port)
		}
		if cfg.Web.Bind != "" {
			t.Errorf("Web.Bind = %q, want empty string when web section absent", cfg.Web.Bind)
		}
	})
}

func TestLoadFrom_WebConfig_ExplicitValues(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	content := []byte("web:\n  port: 8080\n  bind: \"0.0.0.0\"\n")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if cfg.Web.Port != 8080 {
		t.Errorf("Web.Port = %d, want 8080", cfg.Web.Port)
	}
	if cfg.Web.Bind != "0.0.0.0" {
		t.Errorf("Web.Bind = %q, want %q", cfg.Web.Bind, "0.0.0.0")
	}
}

func TestLoadFrom_WebConfig_NoSection_UsesDefaults(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	content := []byte("theme: latte\n") // no web section
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if cfg.Web.Bind != "127.0.0.1" {
		t.Errorf("Web.Bind = %q, want %q (default)", cfg.Web.Bind, "127.0.0.1")
	}
	if cfg.Web.Port != 0 {
		t.Errorf("Web.Port = %d, want 0 (default)", cfg.Web.Port)
	}
}

func TestDefaultConfig_TailscaleDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Tailscale.Enabled {
		t.Error("Tailscale should be disabled by default")
	}
	if cfg.Tailscale.Name != "devagent" {
		t.Errorf("Tailscale.Name = %q, want %q", cfg.Tailscale.Name, "devagent")
	}
	if !cfg.Tailscale.Ephemeral {
		t.Error("Tailscale.Ephemeral should default to true")
	}
	if cfg.Tailscale.AuthKeyPath != "~/.config/devagent/tailscale-authkey" {
		t.Errorf("Tailscale.AuthKeyPath = %q, want default", cfg.Tailscale.AuthKeyPath)
	}
	if cfg.Tailscale.StateDir != "~/.local/share/devagent/tsnsrv" {
		t.Errorf("Tailscale.StateDir = %q, want default", cfg.Tailscale.StateDir)
	}
}

func TestValidateTailscale_DisabledSkipsValidation(t *testing.T) {
	tc := TailscaleConfig{Enabled: false}
	err := tc.Validate(func(s string) string { return s })
	if err != nil {
		t.Errorf("expected nil for disabled tailscale, got %v", err)
	}
}

func TestValidateTailscale_EmptyName(t *testing.T) {
	tc := TailscaleConfig{Enabled: true, Name: "", AuthKeyPath: "/tmp/key"}
	err := tc.Validate(func(s string) string { return s })
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestValidateTailscale_FunnelOnlyRequiresFunnel(t *testing.T) {
	tc := TailscaleConfig{Enabled: true, Name: "test", FunnelOnly: true, Funnel: false, AuthKeyPath: "/tmp/key"}
	err := tc.Validate(func(s string) string { return s })
	if err == nil {
		t.Error("expected error when funnel_only=true but funnel=false")
	}
}

func TestValidateTailscale_AuthKeyMissing(t *testing.T) {
	tc := TailscaleConfig{Enabled: true, Name: "test", AuthKeyPath: "/nonexistent/path/key"}
	err := tc.Validate(func(s string) string { return s })
	if err == nil {
		t.Error("expected error for missing auth key file")
	}
}

func TestValidateTailscale_AuthKeyExists(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "authkey")
	if err := os.WriteFile(tmpFile, []byte("tskey-test"), 0600); err != nil {
		t.Fatal(err)
	}

	tc := TailscaleConfig{Enabled: true, Name: "test", AuthKeyPath: tmpFile}
	err := tc.Validate(func(s string) string { return s })
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestLoadFrom_TailscaleConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	content := []byte(`
tailscale:
  enabled: true
  name: myagent
  funnel: true
  tags:
    - tag:dev
`)
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if !cfg.Tailscale.Enabled {
		t.Error("Tailscale.Enabled should be true")
	}
	if cfg.Tailscale.Name != "myagent" {
		t.Errorf("Tailscale.Name = %q, want %q", cfg.Tailscale.Name, "myagent")
	}
	if !cfg.Tailscale.Funnel {
		t.Error("Tailscale.Funnel should be true")
	}
	if len(cfg.Tailscale.Tags) != 1 || cfg.Tailscale.Tags[0] != "tag:dev" {
		t.Errorf("Tailscale.Tags = %v, want [tag:dev]", cfg.Tailscale.Tags)
	}
}

func TestResolveTokenPath_Empty(t *testing.T) {
	cfg := Config{}
	if got := cfg.ResolveTokenPath(""); got != "" {
		t.Errorf("ResolveTokenPath(\"\") = %q, want empty", got)
	}
}

func TestResolveTokenPath_TildeExpansion(t *testing.T) {
	cfg := Config{}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	got := cfg.ResolveTokenPath("~/foo/bar")
	want := filepath.Join(home, "foo/bar")
	if got != want {
		t.Errorf("ResolveTokenPath(\"~/foo/bar\") = %q, want %q", got, want)
	}
}

func TestResolveTokenPath_AbsoluteUnchanged(t *testing.T) {
	cfg := Config{}
	got := cfg.ResolveTokenPath("/etc/tokens/test")
	if got != "/etc/tokens/test" {
		t.Errorf("ResolveTokenPath(\"/etc/tokens/test\") = %q, want %q", got, "/etc/tokens/test")
	}
}

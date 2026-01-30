package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFullConfig(t *testing.T) {
	// Create temp config file with all fields
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := `
theme: latte
runtime: podman
otel:
  grpc_port: 4317
credentials:
  OPENAI_API_KEY: OPENAI_API_KEY
  ANTHROPIC_API_KEY: ANTHROPIC_API_KEY
agents:
  claude-code:
    display_name: Claude Code
    otel_env:
      OTEL_SERVICE_NAME: claude-code
      OTEL_EXPORTER_OTLP_ENDPOINT: http://host.docker.internal:4317
    state_sources:
      - type: files
        events: [create, modify]
        patterns:
          todo: ["**/TODO.md"]
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

	// Verify OTEL config
	if cfg.OTEL.GRPCPort != 4317 {
		t.Errorf("OTEL.GRPCPort: got %d, want %d", cfg.OTEL.GRPCPort, 4317)
	}

	// Verify credentials
	if len(cfg.Credentials) != 2 {
		t.Errorf("Credentials length: got %d, want %d", len(cfg.Credentials), 2)
	}
	if cfg.Credentials["OPENAI_API_KEY"] != "OPENAI_API_KEY" {
		t.Errorf("Credentials[OPENAI_API_KEY]: got %q, want %q", cfg.Credentials["OPENAI_API_KEY"], "OPENAI_API_KEY")
	}

	// Verify agents
	if len(cfg.Agents) != 1 {
		t.Errorf("Agents length: got %d, want %d", len(cfg.Agents), 1)
	}
	agent, ok := cfg.Agents["claude-code"]
	if !ok {
		t.Fatal("Agents[claude-code] not found")
	}
	if agent.DisplayName != "Claude Code" {
		t.Errorf("Agent DisplayName: got %q, want %q", agent.DisplayName, "Claude Code")
	}
	if agent.OTELEnv["OTEL_SERVICE_NAME"] != "claude-code" {
		t.Errorf("Agent OTELEnv[OTEL_SERVICE_NAME]: got %q, want %q", agent.OTELEnv["OTEL_SERVICE_NAME"], "claude-code")
	}
	if len(agent.StateSources) != 1 {
		t.Errorf("Agent StateSources length: got %d, want %d", len(agent.StateSources), 1)
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

func TestGetCredentialValue_Found(t *testing.T) {
	cfg := Config{
		Credentials: map[string]string{
			"MY_API_KEY": "TEST_ENV_VAR",
		},
	}

	// Set the env var
	t.Setenv("TEST_ENV_VAR", "secret-value")

	value, ok := cfg.GetCredentialValue("MY_API_KEY")
	if !ok {
		t.Error("GetCredentialValue: expected ok=true")
	}
	if value != "secret-value" {
		t.Errorf("GetCredentialValue: got %q, want %q", value, "secret-value")
	}
}

func TestGetCredentialValue_NotFound(t *testing.T) {
	cfg := Config{
		Credentials: map[string]string{},
	}

	value, ok := cfg.GetCredentialValue("UNKNOWN_KEY")
	if ok {
		t.Error("GetCredentialValue: expected ok=false for unknown key")
	}
	if value != "" {
		t.Errorf("GetCredentialValue: got %q, want empty string", value)
	}
}

func TestGetCredentialValue_EnvVarNotSet(t *testing.T) {
	cfg := Config{
		Credentials: map[string]string{
			"MY_API_KEY": "UNSET_ENV_VAR_12345",
		},
	}

	value, ok := cfg.GetCredentialValue("MY_API_KEY")
	if ok {
		t.Error("GetCredentialValue: expected ok=false when env var not set")
	}
	if value != "" {
		t.Errorf("GetCredentialValue: got %q, want empty string", value)
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

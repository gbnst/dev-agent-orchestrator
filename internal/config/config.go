// pattern: Imperative Shell

package config

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Theme           string          `yaml:"theme"`
	Runtime         string          `yaml:"runtime"`
	LogLevel        string          `yaml:"log_level"`
	Web             WebConfig       `yaml:"web"`
	Tailscale       TailscaleConfig `yaml:"tailscale"`
	ClaudeTokenPath string          `yaml:"claude_token_path"`
	GitHubTokenPath string          `yaml:"github_token_path"`
}

type TailscaleConfig struct {
	Enabled     bool     `yaml:"enabled"`
	Name        string   `yaml:"name"`
	Funnel      bool     `yaml:"funnel"`
	FunnelOnly  bool     `yaml:"funnel_only"`
	Ephemeral   bool     `yaml:"ephemeral"`
	Plaintext   bool     `yaml:"plaintext"`
	AuthKeyPath string   `yaml:"auth_key_path"`
	StateDir    string   `yaml:"state_dir"`
	Tags        []string `yaml:"tags"`
}

type WebConfig struct {
	Bind string `yaml:"bind"`
	Port int    `yaml:"port"`
}

// LookPathFunc is the function signature for looking up executables.
type LookPathFunc func(name string) (string, error)

func DefaultConfig() Config {
	return Config{
		Theme:    "mocha",
		LogLevel: "info",
		Web: WebConfig{
			Bind: "127.0.0.1",
			Port: 0, // disabled by default
		},
		Tailscale: TailscaleConfig{
			Name:        "devagent",
			Ephemeral:   true,
			AuthKeyPath: "~/.config/devagent/tailscale-authkey",
			StateDir:    "~/.local/share/devagent/tsnsrv",
		},
	}
}

func Load() (Config, error) {
	return LoadFromDir(getConfigDir())
}

// LoadFromDir loads config and templates from a specified directory.
func LoadFromDir(configDir string) (Config, error) {
	configPath := filepath.Join(configDir, "config.yaml")
	templatesPath := filepath.Join(configDir, "templates")

	// Set the templates path for LoadTemplates to use
	SetTemplatesPath(templatesPath)

	return LoadFrom(configPath)
}

func LoadFrom(configPath string) (Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), err
	}

	if cfg.Theme == "" {
		cfg.Theme = "mocha"
	}

	return cfg, nil
}

// DetectedRuntime returns the configured runtime or auto-detects it.
func (c *Config) DetectedRuntime() string {
	return c.DetectedRuntimeWith(exec.LookPath)
}

// DetectedRuntimeWith returns the configured runtime or auto-detects it
// using the provided lookup function.
func (c *Config) DetectedRuntimeWith(lookPath LookPathFunc) string {
	if c.Runtime != "" {
		return c.Runtime
	}

	// Try docker first, then podman
	if _, err := lookPath("docker"); err == nil {
		return "docker"
	}
	if _, err := lookPath("podman"); err == nil {
		return "podman"
	}

	// Default to docker
	return "docker"
}

// DetectedRuntimePath returns the full path to the detected runtime binary.
// This is useful for generating commands that bypass shell aliases.
func (c *Config) DetectedRuntimePath() string {
	return c.DetectedRuntimePathWith(exec.LookPath)
}

// DetectedRuntimePathWith returns the full path to the detected runtime binary
// using the provided lookup function.
func (c *Config) DetectedRuntimePathWith(lookPath LookPathFunc) string {
	// If explicitly configured, look up that specific runtime
	if c.Runtime != "" {
		if path, err := lookPath(c.Runtime); err == nil {
			return path
		}
		// Fallback to just the name if lookup fails
		return c.Runtime
	}

	// Try docker first, then podman
	if path, err := lookPath("docker"); err == nil {
		return path
	}
	if path, err := lookPath("podman"); err == nil {
		return path
	}

	// Default to docker (without path)
	return "docker"
}

// ValidateRuntime validates the configured runtime.
// If Runtime is empty (auto-detect mode), validation is skipped.
// Otherwise, validates the runtime is "docker" or "podman" and the binary exists.
func (c *Config) ValidateRuntime() error {
	return c.ValidateRuntimeWith(exec.LookPath)
}

// ValidateRuntimeWith validates the configured runtime using the provided lookup function.
func (c *Config) ValidateRuntimeWith(lookPath LookPathFunc) error {
	if c.Runtime == "" {
		// Auto-detect mode - skip validation
		return nil
	}

	// Validate runtime is a known value
	if c.Runtime != "docker" && c.Runtime != "podman" {
		return errors.New("runtime must be 'docker' or 'podman', got: " + c.Runtime)
	}

	// Validate binary exists
	if _, err := lookPath(c.Runtime); err != nil {
		return errors.New("runtime '" + c.Runtime + "' not found in PATH")
	}

	return nil
}

// ResolveTokenPath expands a token path, resolving ~/... to the user's home directory.
// Returns empty string if path is empty.
func (c *Config) ResolveTokenPath(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// ResolvePathFunc is the function signature for resolving paths with ~ expansion.
type ResolvePathFunc func(string) string

// ValidateTailscale validates the TailscaleConfig.
// resolveTokenPath expands ~ in paths (use Config.ResolveTokenPath).
func (tc *TailscaleConfig) Validate(resolvePath ResolvePathFunc) error {
	if !tc.Enabled {
		return nil
	}
	if tc.Name == "" {
		return errors.New("tailscale.name must be non-empty when tailscale is enabled")
	}
	if tc.FunnelOnly && !tc.Funnel {
		return errors.New("tailscale.funnel_only requires tailscale.funnel to be enabled")
	}
	authPath := resolvePath(tc.AuthKeyPath)
	if authPath == "" {
		return errors.New("tailscale.auth_key_path must be set when tailscale is enabled")
	}
	if _, err := os.Stat(authPath); err != nil {
		return fmt.Errorf("tailscale auth key file not found: %s", authPath)
	}
	return nil
}

func getConfigDir() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "devagent")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "devagent")
	}

	return filepath.Join(home, ".config", "devagent")
}

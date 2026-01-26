package config

import (
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Theme       string                 `yaml:"theme"`
	Runtime     string                 `yaml:"runtime"`
	OTEL        OTELConfig             `yaml:"otel"`
	Credentials map[string]string      `yaml:"credentials"`
	BaseImages  map[string]string      `yaml:"base_images"`
	Agents      map[string]AgentConfig `yaml:"agents"`
}

type OTELConfig struct {
	GRPCPort int `yaml:"grpc_port"`
}

type AgentConfig struct {
	DisplayName  string            `yaml:"display_name"`
	OTELEnv      map[string]string `yaml:"otel_env"`
	StateSources []StateSource     `yaml:"state_sources"`
}

type StateSource struct {
	Type     string              `yaml:"type"`
	Events   []string            `yaml:"events"`
	Patterns map[string][]string `yaml:"patterns"`
	Paths    []string            `yaml:"paths"`
}

// LookPathFunc is the function signature for looking up executables.
type LookPathFunc func(name string) (string, error)

func DefaultConfig() Config {
	return Config{
		Theme: "mocha",
	}
}

func Load() (Config, error) {
	return LoadFrom(getConfigPath())
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

// GetCredentialValue looks up a credential by name and returns its value
// from the host environment.
func (c *Config) GetCredentialValue(name string) (string, bool) {
	envVar, ok := c.Credentials[name]
	if !ok {
		return "", false
	}

	value := os.Getenv(envVar)
	if value == "" {
		return "", false
	}

	return value, true
}

// ResolveBaseImage returns the full image reference for a named base image.
func (c *Config) ResolveBaseImage(name string) (string, bool) {
	image, ok := c.BaseImages[name]
	return image, ok
}

func getConfigPath() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "devagent", "config.yaml")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "devagent", "config.yaml")
	}

	return filepath.Join(home, ".config", "devagent", "config.yaml")
}

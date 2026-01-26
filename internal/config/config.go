package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Theme string `yaml:"theme"`
}

func DefaultConfig() Config {
	return Config{
		Theme: "mocha",
	}
}

func Load() (Config, error) {
	cfg := DefaultConfig()

	configPath := getConfigPath()
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

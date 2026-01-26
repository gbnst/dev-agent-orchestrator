package config

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Template represents a devcontainer template configuration.
type Template struct {
	Name              string           `yaml:"name"`
	Description       string           `yaml:"description"`
	BaseImage         string           `yaml:"base_image"`
	Devcontainer      DevcontainerSpec `yaml:"devcontainer"`
	InjectCredentials []string         `yaml:"inject_credentials"`
	DefaultAgent      string           `yaml:"default_agent"`
}

// DevcontainerSpec holds devcontainer.json settings from a template.
type DevcontainerSpec struct {
	Features          map[string]map[string]interface{} `yaml:"features"`
	Customizations    map[string]interface{}            `yaml:"customizations"`
	PostCreateCommand string                            `yaml:"postCreateCommand"`
}

// customTemplatesPath allows overriding the templates directory.
var customTemplatesPath string

// SetTemplatesPath sets a custom templates directory path.
func SetTemplatesPath(path string) {
	customTemplatesPath = path
}

// LoadTemplates loads all templates from the default templates directory.
func LoadTemplates() ([]Template, error) {
	if customTemplatesPath != "" {
		return LoadTemplatesFrom(customTemplatesPath)
	}
	return LoadTemplatesFrom(getTemplatesPath())
}

// LoadTemplatesFrom loads all .yaml and .yml files from the specified directory.
func LoadTemplatesFrom(dir string) ([]Template, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Template{}, nil
		}
		return nil, err
	}

	var templates []Template
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		templatePath := filepath.Join(dir, name)
		tmpl, err := loadTemplate(templatePath)
		if err != nil {
			log.Printf("Warning: failed to load template %s: %v", name, err)
			continue
		}
		templates = append(templates, tmpl)
	}

	return templates, nil
}

func loadTemplate(path string) (Template, error) {
	var tmpl Template

	data, err := os.ReadFile(path)
	if err != nil {
		return tmpl, err
	}

	if err := yaml.Unmarshal(data, &tmpl); err != nil {
		return tmpl, err
	}

	return tmpl, nil
}

func getTemplatesPath() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "devagent", "templates")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "devagent", "templates")
	}

	return filepath.Join(home, ".config", "devagent", "templates")
}

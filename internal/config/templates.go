// pattern: Imperative Shell

package config

import (
	"os"
	"path/filepath"
)

// Template represents a loaded devcontainer template.
// Templates are discovered by scanning directories for docker-compose.yml.tmpl marker files.
// All orchestration config (capabilities, resources, network allowlists) is hardcoded
// directly in template files (docker-compose.yml.tmpl, filter.py.tmpl, devcontainer.json.tmpl).
type Template struct {
	Name string // Template name (from directory name)
	Path string // Absolute path to template directory
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

// LoadTemplatesFrom loads all templates from the specified directory.
// Each subdirectory containing a docker-compose.yml.tmpl file is treated as a template.
// The directory name is used as the template name.
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
		if !entry.IsDir() {
			continue
		}

		templateDir := filepath.Join(dir, entry.Name())
		markerPath := filepath.Join(templateDir, "docker-compose.yml.tmpl")
		if _, err := os.Stat(markerPath); err != nil {
			if os.IsNotExist(err) {
				continue // Not a template directory
			}
			continue // Skip on stat errors
		}

		tmpl, err := loadTemplate(templateDir, entry.Name())
		if err != nil {
			continue
		}
		templates = append(templates, tmpl)
	}

	return templates, nil
}

// loadTemplate loads a single template from a directory.
// The dirName is used as the template name.
func loadTemplate(templateDir string, dirName string) (Template, error) {
	return Template{
		Name: dirName,
		Path: templateDir,
	}, nil
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

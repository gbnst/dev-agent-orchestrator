// pattern: Imperative Shell

package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// BuildConfig represents the build section of a devcontainer.json.
type BuildConfig struct {
	Dockerfile string `json:"dockerfile,omitempty"`
	Context    string `json:"context,omitempty"`
}

// Template represents a loaded devcontainer template with devagent extensions.
// Templates are loaded from directories containing a devcontainer.json file.
type Template struct {
	// Standard devcontainer.json fields
	Name              string                            `json:"name"`
	Image             string                            `json:"image,omitempty"`
	Build             *BuildConfig                      `json:"build,omitempty"`
	Features          map[string]map[string]interface{} `json:"features,omitempty"`
	Customizations    map[string]interface{}            `json:"customizations,omitempty"`
	PostCreateCommand string                            `json:"postCreateCommand,omitempty"`
	RemoteUser        string                            `json:"remoteUser,omitempty"`

	// Devagent-specific fields extracted from customizations.devagent
	InjectCredentials []string `json:"-"` // Populated from customizations.devagent.injectCredentials
	DefaultAgent      string   `json:"-"` // Populated from customizations.devagent.defaultAgent

	// Path to the template directory (for copying additional files like Dockerfile)
	Path string `json:"-"`
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
// Each subdirectory containing a devcontainer.json file is treated as a template.
// The directory name is used as the template name if not specified in the JSON.
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

		templatePath := filepath.Join(dir, entry.Name(), "devcontainer.json")
		tmpl, err := loadTemplate(templatePath, entry.Name())
		if err != nil {
			if os.IsNotExist(err) {
				// Directory doesn't contain devcontainer.json, skip it
				continue
			}
			// Log warning but continue loading other templates
			log.Printf("Warning: failed to load template from %s: %v", templatePath, err)
			continue
		}
		templates = append(templates, tmpl)
	}

	return templates, nil
}

// loadTemplate loads a single template from a devcontainer.json file.
// The dirName is used as the template name if not specified in the JSON.
func loadTemplate(path string, dirName string) (Template, error) {
	var tmpl Template

	data, err := os.ReadFile(path)
	if err != nil {
		return tmpl, err
	}

	if err := json.Unmarshal(data, &tmpl); err != nil {
		return tmpl, err
	}

	// Store template directory path for copying additional files
	tmpl.Path = filepath.Dir(path)

	// Use directory name as template name if not specified
	if tmpl.Name == "" {
		tmpl.Name = dirName
	}

	// Extract devagent-specific fields from customizations.devagent
	if tmpl.Customizations != nil {
		if devagent, ok := tmpl.Customizations["devagent"].(map[string]interface{}); ok {
			if creds, ok := devagent["injectCredentials"].([]interface{}); ok {
				for _, c := range creds {
					if s, ok := c.(string); ok {
						tmpl.InjectCredentials = append(tmpl.InjectCredentials, s)
					}
				}
			}
			if agent, ok := devagent["defaultAgent"].(string); ok {
				tmpl.DefaultAgent = agent
			}
		}
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

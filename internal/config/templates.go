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

// IsolationConfig defines security isolation settings for containers
type IsolationConfig struct {
	Enabled   *bool          `json:"enabled,omitempty"`   // nil means use default (true)
	Caps      *CapConfig     `json:"caps,omitempty"`
	Resources *ResourceConfig `json:"resources,omitempty"`
	Network   *NetworkConfig `json:"network,omitempty"`
}

// CapConfig defines Linux capability settings
type CapConfig struct {
	Drop []string `json:"drop,omitempty"` // Capabilities to drop (e.g., "NET_RAW", "SYS_ADMIN")
	Add  []string `json:"add,omitempty"`  // Capabilities to add (use sparingly)
}

// ResourceConfig defines container resource limits
type ResourceConfig struct {
	Memory   string `json:"memory,omitempty"`   // Memory limit (e.g., "2g", "512m")
	CPUs     string `json:"cpus,omitempty"`     // CPU limit (e.g., "2", "0.5")
	PidsLimit int   `json:"pidsLimit,omitempty"` // Process limit (0 means no limit)
}

// NetworkConfig defines network isolation settings
type NetworkConfig struct {
	Allowlist          []string `json:"allowlist,omitempty"`          // Allowed domains
	AllowlistExtend    []string `json:"allowlistExtend,omitempty"`    // Domains to add to defaults
	Passthrough        []string `json:"passthrough,omitempty"`        // Certificate-pinned domains (bypass TLS interception)
	BlockGitHubPRMerge bool     `json:"blockGitHubPRMerge,omitempty"` // Block GitHub PR merge API calls
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
	InjectCredentials []string         `json:"-"` // Populated from customizations.devagent.injectCredentials
	DefaultAgent      string           `json:"-"` // Populated from customizations.devagent.defaultAgent
	Isolation         *IsolationConfig `json:"-"` // Populated from customizations.devagent.isolation

	// Path to the template directory (for copying additional files like Dockerfile)
	Path string `json:"-"`
}

// parseIsolationConfig extracts isolation settings from the devagent customizations map
func parseIsolationConfig(devagent map[string]interface{}) *IsolationConfig {
	isolation, ok := devagent["isolation"].(map[string]interface{})
	if !ok {
		return nil
	}

	config := &IsolationConfig{}

	// Parse enabled field
	if enabled, ok := isolation["enabled"].(bool); ok {
		config.Enabled = &enabled
	}

	// Parse caps
	if caps, ok := isolation["caps"].(map[string]interface{}); ok {
		config.Caps = &CapConfig{}
		if drop, ok := caps["drop"].([]interface{}); ok {
			for _, d := range drop {
				if s, ok := d.(string); ok {
					config.Caps.Drop = append(config.Caps.Drop, s)
				}
			}
		}
		if add, ok := caps["add"].([]interface{}); ok {
			for _, a := range add {
				if s, ok := a.(string); ok {
					config.Caps.Add = append(config.Caps.Add, s)
				}
			}
		}
	}

	// Parse resources
	if resources, ok := isolation["resources"].(map[string]interface{}); ok {
		config.Resources = &ResourceConfig{}
		if memory, ok := resources["memory"].(string); ok {
			config.Resources.Memory = memory
		}
		if cpus, ok := resources["cpus"].(string); ok {
			config.Resources.CPUs = cpus
		}
		if pidsLimit, ok := resources["pidsLimit"].(float64); ok {
			config.Resources.PidsLimit = int(pidsLimit)
		}
	}

	// Parse network
	if network, ok := isolation["network"].(map[string]interface{}); ok {
		config.Network = &NetworkConfig{}
		if allowlist, ok := network["allowlist"].([]interface{}); ok {
			for _, a := range allowlist {
				if s, ok := a.(string); ok {
					config.Network.Allowlist = append(config.Network.Allowlist, s)
				}
			}
		}
		if allowlistExtend, ok := network["allowlistExtend"].([]interface{}); ok {
			for _, a := range allowlistExtend {
				if s, ok := a.(string); ok {
					config.Network.AllowlistExtend = append(config.Network.AllowlistExtend, s)
				}
			}
		}
		if passthrough, ok := network["passthrough"].([]interface{}); ok {
			for _, p := range passthrough {
				if s, ok := p.(string); ok {
					config.Network.Passthrough = append(config.Network.Passthrough, s)
				}
			}
		}
		if blockPRMerge, ok := network["blockGitHubPRMerge"].(bool); ok {
			config.Network.BlockGitHubPRMerge = blockPRMerge
		}
	}

	return config
}

// DefaultIsolation provides secure defaults for containers when no isolation config is specified.
// These defaults drop dangerous capabilities and set reasonable resource limits.
var DefaultIsolation = &IsolationConfig{
	Caps: &CapConfig{
		Drop: []string{
			"NET_RAW",      // Prevents raw socket access (mitigates network attacks)
			"SYS_ADMIN",    // Prevents mount namespace manipulation
			"SYS_PTRACE",   // Prevents process tracing
			"MKNOD",        // Prevents device node creation
			"NET_ADMIN",    // Prevents network configuration changes
			"SYS_MODULE",   // Prevents kernel module loading
			"SYS_RAWIO",    // Prevents raw I/O operations
			"SYS_BOOT",     // Prevents reboot
			"SYS_NICE",     // Prevents priority manipulation
			"SYS_RESOURCE", // Prevents resource limit manipulation
		},
	},
	Resources: &ResourceConfig{
		Memory:    "4g",
		CPUs:      "2",
		PidsLimit: 512,
	},
	Network: &NetworkConfig{
		Allowlist: []string{
			"api.anthropic.com",
			"github.com",
			"*.github.com",
			"api.github.com",
			"raw.githubusercontent.com",
			"objects.githubusercontent.com",
			"registry.npmjs.org",
			"pypi.org",
			"files.pythonhosted.org",
			"proxy.golang.org",
			"sum.golang.org",
			"storage.googleapis.com",
			"pkg.go.dev",
		},
		Passthrough: []string{
			// Domains known to use certificate pinning
		},
	},
}

// MergeIsolationConfig merges a template's isolation config with defaults.
// Rules:
//   - If template isolation is nil, use defaults
//   - If template isolation has enabled: false, return nil (disable isolation)
//   - Otherwise, merge template with defaults (template values override)
//   - AllowlistExtend is appended to default allowlist
func MergeIsolationConfig(template *IsolationConfig, defaults *IsolationConfig) *IsolationConfig {
	// No template isolation -> use defaults
	if template == nil {
		return copyIsolationConfig(defaults)
	}

	// Explicitly disabled -> no isolation
	if template.Enabled != nil && !*template.Enabled {
		return nil
	}

	// Start with a copy of defaults
	merged := copyIsolationConfig(defaults)
	if merged == nil {
		merged = &IsolationConfig{}
	}

	// Override with template values

	// Merge capabilities
	if template.Caps != nil {
		if merged.Caps == nil {
			merged.Caps = &CapConfig{}
		}
		if len(template.Caps.Drop) > 0 {
			merged.Caps.Drop = template.Caps.Drop
		}
		if len(template.Caps.Add) > 0 {
			merged.Caps.Add = template.Caps.Add
		}
	}

	// Merge resources
	if template.Resources != nil {
		if merged.Resources == nil {
			merged.Resources = &ResourceConfig{}
		}
		if template.Resources.Memory != "" {
			merged.Resources.Memory = template.Resources.Memory
		}
		if template.Resources.CPUs != "" {
			merged.Resources.CPUs = template.Resources.CPUs
		}
		if template.Resources.PidsLimit > 0 {
			merged.Resources.PidsLimit = template.Resources.PidsLimit
		}
	}

	// Merge network - allowlistExtend is appended, allowlist replaces
	if template.Network != nil {
		if merged.Network == nil {
			merged.Network = &NetworkConfig{}
		}
		if len(template.Network.Allowlist) > 0 {
			// Template provides explicit allowlist, use it
			merged.Network.Allowlist = template.Network.Allowlist
		}
		if len(template.Network.AllowlistExtend) > 0 {
			// Append extended domains to the allowlist
			merged.Network.Allowlist = append(merged.Network.Allowlist, template.Network.AllowlistExtend...)
		}
		if len(template.Network.Passthrough) > 0 {
			merged.Network.Passthrough = template.Network.Passthrough
		}
		// BlockGitHubPRMerge: template value overrides default
		if template.Network.BlockGitHubPRMerge {
			merged.Network.BlockGitHubPRMerge = true
		}
	}

	return merged
}

// copyIsolationConfig creates a deep copy of an IsolationConfig.
func copyIsolationConfig(src *IsolationConfig) *IsolationConfig {
	if src == nil {
		return nil
	}

	dst := &IsolationConfig{}

	if src.Enabled != nil {
		enabled := *src.Enabled
		dst.Enabled = &enabled
	}

	if src.Caps != nil {
		dst.Caps = &CapConfig{
			Drop: append([]string{}, src.Caps.Drop...),
			Add:  append([]string{}, src.Caps.Add...),
		}
	}

	if src.Resources != nil {
		dst.Resources = &ResourceConfig{
			Memory:    src.Resources.Memory,
			CPUs:      src.Resources.CPUs,
			PidsLimit: src.Resources.PidsLimit,
		}
	}

	if src.Network != nil {
		dst.Network = &NetworkConfig{
			Allowlist:          append([]string{}, src.Network.Allowlist...),
			AllowlistExtend:    append([]string{}, src.Network.AllowlistExtend...),
			Passthrough:        append([]string{}, src.Network.Passthrough...),
			BlockGitHubPRMerge: src.Network.BlockGitHubPRMerge,
		}
	}

	return dst
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
			// Parse isolation config
			tmpl.Isolation = parseIsolationConfig(devagent)
		}
	}

	return tmpl, nil
}

// GetEffectiveIsolation returns the merged isolation config for a template.
// Returns nil if isolation is explicitly disabled, or the merged config otherwise.
func (t *Template) GetEffectiveIsolation() *IsolationConfig {
	return MergeIsolationConfig(t.Isolation, DefaultIsolation)
}

// IsIsolationEnabled returns true if isolation is enabled for this template.
// Isolation is enabled by default unless explicitly disabled with enabled: false.
func (t *Template) IsIsolationEnabled() bool {
	if t.Isolation != nil && t.Isolation.Enabled != nil {
		return *t.Isolation.Enabled
	}
	// Default is enabled
	return true
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

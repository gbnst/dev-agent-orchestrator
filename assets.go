// pattern: Imperative Shell
package main

import (
	"embed"
	"fmt"
	"io/fs"

	"devagent/internal/config"
)

// builtinTemplatesFS holds the devcontainer templates shipped with the binary.
// The `all:` prefix is required so dot-prefixed entries (.devcontainer,
// .gitignore.tmpl) are embedded too. On first run (and after an upgrade) these
// are materialized into ~/.config/devagent/templates; see config.EnsureUserConfig.
//
//go:embed all:config/templates
var builtinTemplatesFS embed.FS

// defaultConfigYAML is the curated config.yaml seeded into the profile on first
// run (distinct from the dev config under ./config used by `make dev`).
//
//go:embed config/config.default.yaml
var defaultConfigYAML []byte

// builtinAssets bundles the embedded defaults for config.EnsureUserConfig.
// Templates is re-rooted at the templates directory so each child is a template.
func builtinAssets() (config.BuiltinAssets, error) {
	templates, err := fs.Sub(builtinTemplatesFS, "config/templates")
	if err != nil {
		return config.BuiltinAssets{}, fmt.Errorf("failed to root embedded templates: %w", err)
	}
	return config.BuiltinAssets{
		Templates:     templates,
		DefaultConfig: defaultConfigYAML,
		Version:       version,
	}, nil
}

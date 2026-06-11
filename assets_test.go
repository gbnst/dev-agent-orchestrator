package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"devagent/internal/config"
)

// TestBuiltinAssetsProvision exercises the real embedded template tree end to
// end: build assets from the embed.FS and materialize them into a temp profile.
func TestBuiltinAssetsProvision(t *testing.T) {
	assets, err := builtinAssets()
	if err != nil {
		t.Fatalf("builtinAssets: %v", err)
	}
	assets.Version = "test" // pin so the run is deterministic

	dir := t.TempDir()
	res, err := config.EnsureUserConfig(dir, assets, "20260101-000000")
	if err != nil {
		t.Fatalf("EnsureUserConfig: %v", err)
	}
	if !res.ConfigSeeded || !res.TemplatesSynced {
		t.Fatalf("expected fresh provision, got %+v", res)
	}

	// config.yaml seeded from the curated default.
	cfg := mustRead(t, filepath.Join(dir, "config.yaml"))
	if !strings.Contains(cfg, "theme:") {
		t.Errorf("seeded config.yaml missing theme key:\n%s", cfg)
	}

	// All three bundled templates materialized, including dot-prefixed entries
	// that require the `all:` embed prefix.
	for _, tmpl := range []string{"basic", "go-project", "python-fullstack"} {
		marker := filepath.Join(dir, "templates", tmpl, ".devcontainer", "docker-compose.yml.tmpl")
		if _, err := os.Stat(marker); err != nil {
			t.Errorf("template %s not materialized: %v", tmpl, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "templates", tmpl, ".devcontainer", ".gitignore.tmpl")); err != nil {
			t.Errorf("template %s missing .gitignore.tmpl (all: prefix?): %v", tmpl, err)
		}
	}

	// The security-relevant read-only proxy bind must be present in the
	// materialized compose template — this is what reaches users.
	compose := mustRead(t, filepath.Join(dir, "templates", "basic", ".devcontainer", "docker-compose.yml.tmpl"))
	if !strings.Contains(compose, "devagent-proxy") || !strings.Contains(compose, "read_only: true") {
		t.Errorf("materialized compose missing read-only proxy bind:\n%s", compose)
	}

	// No Python bytecode should ship in the embed.
	for rel := range func() map[string]struct{} {
		seen := map[string]struct{}{}
		_ = filepath.WalkDir(filepath.Join(dir, "templates"), func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				seen[p] = struct{}{}
			}
			return nil
		})
		return seen
	}() {
		if strings.Contains(rel, "__pycache__") || strings.HasSuffix(rel, ".pyc") {
			t.Errorf("unexpected python bytecode embedded: %s", rel)
		}
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

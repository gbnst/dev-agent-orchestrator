package config

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// builtinForTest returns a small embedded-template stand-in plus a default
// config, mirroring how main.go builds BuiltinAssets from embed.FS.
func builtinForTest(version string, files map[string]string) BuiltinAssets {
	mapfs := fstest.MapFS{}
	for path, content := range files {
		mapfs[path] = &fstest.MapFile{Data: []byte(content)}
	}
	return BuiltinAssets{
		Templates:     mapfs,
		DefaultConfig: []byte("theme: mocha\n"),
		Version:       version,
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestEnsureUserConfig_FirstRun(t *testing.T) {
	dir := t.TempDir()
	assets := builtinForTest("1.0.0", map[string]string{
		"basic/.devcontainer/docker-compose.yml.tmpl": "services: {}\n",
		"basic/.devcontainer/Dockerfile":              "FROM scratch\n",
	})

	res, err := EnsureUserConfig(dir, assets, "20260101-000000")
	if err != nil {
		t.Fatalf("EnsureUserConfig: %v", err)
	}

	if !res.ConfigSeeded {
		t.Error("expected ConfigSeeded=true on first run")
	}
	if !res.TemplatesSynced {
		t.Error("expected TemplatesSynced=true on first run")
	}
	if len(res.BackedUp) != 0 {
		t.Errorf("expected no backups on first run, got %v", res.BackedUp)
	}

	if got := readFile(t, filepath.Join(dir, "config.yaml")); got != "theme: mocha\n" {
		t.Errorf("config.yaml = %q", got)
	}
	if got := readFile(t, filepath.Join(dir, "templates", "basic", ".devcontainer", "docker-compose.yml.tmpl")); got != "services: {}\n" {
		t.Errorf("compose template = %q", got)
	}
	if got := readFile(t, filepath.Join(dir, templatesMarkerName)); got != "1.0.0" {
		t.Errorf("marker = %q, want 1.0.0", got)
	}
}

func TestEnsureUserConfig_Idempotent(t *testing.T) {
	dir := t.TempDir()
	assets := builtinForTest("1.0.0", map[string]string{
		"basic/.devcontainer/docker-compose.yml.tmpl": "services: {}\n",
	})

	if _, err := EnsureUserConfig(dir, assets, "20260101-000000"); err != nil {
		t.Fatalf("first run: %v", err)
	}
	res, err := EnsureUserConfig(dir, assets, "20260102-000000")
	if err != nil {
		t.Fatalf("second run: %v", err)
	}

	if res.ConfigSeeded {
		t.Error("expected ConfigSeeded=false on second run")
	}
	if res.TemplatesSynced {
		t.Error("expected TemplatesSynced=false when version unchanged")
	}
}

func TestEnsureUserConfig_PreservesExistingConfig(t *testing.T) {
	dir := t.TempDir()
	custom := "theme: latte\nlog_level: debug\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	assets := builtinForTest("1.0.0", map[string]string{
		"basic/.devcontainer/docker-compose.yml.tmpl": "services: {}\n",
	})

	res, err := EnsureUserConfig(dir, assets, "20260101-000000")
	if err != nil {
		t.Fatalf("EnsureUserConfig: %v", err)
	}
	if res.ConfigSeeded {
		t.Error("expected ConfigSeeded=false when config.yaml already exists")
	}
	if got := readFile(t, filepath.Join(dir, "config.yaml")); got != custom {
		t.Errorf("config.yaml was overwritten: %q", got)
	}
}

func TestEnsureUserConfig_BacksUpDivergentTemplateOnUpgrade(t *testing.T) {
	dir := t.TempDir()
	rel := "basic/.devcontainer/docker-compose.yml.tmpl"
	diskPath := filepath.Join(dir, "templates", filepath.FromSlash(rel))

	// v1 install.
	v1 := builtinForTest("1.0.0", map[string]string{rel: "FIXED-v1\n"})
	if _, err := EnsureUserConfig(dir, v1, "20260101-000000"); err != nil {
		t.Fatalf("v1: %v", err)
	}

	// User edits the materialized template (a customization / a stale pre-fix copy).
	if err := os.WriteFile(diskPath, []byte("USER-EDIT\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// v2 ships a new (fixed) version of the same file.
	v2 := builtinForTest("2.0.0", map[string]string{rel: "FIXED-v2\n"})
	res, err := EnsureUserConfig(dir, v2, "20260202-120000")
	if err != nil {
		t.Fatalf("v2: %v", err)
	}

	if !res.TemplatesSynced {
		t.Error("expected TemplatesSynced=true on version change")
	}
	if len(res.BackedUp) != 1 || res.BackedUp[0] != rel {
		t.Errorf("BackedUp = %v, want [%s]", res.BackedUp, rel)
	}

	// On-disk file now carries the fixed v2 content.
	if got := readFile(t, diskPath); got != "FIXED-v2\n" {
		t.Errorf("template after upgrade = %q, want FIXED-v2", got)
	}
	// The user's edit is preserved in the backup, not lost.
	backup := filepath.Join(res.BackupDir, filepath.FromSlash(rel))
	if got := readFile(t, backup); got != "USER-EDIT\n" {
		t.Errorf("backup = %q, want USER-EDIT", got)
	}
	if got := readFile(t, filepath.Join(dir, templatesMarkerName)); got != "2.0.0" {
		t.Errorf("marker = %q, want 2.0.0", got)
	}
}

func TestEnsureUserConfig_PreservesUserAddedTemplateOnUpgrade(t *testing.T) {
	dir := t.TempDir()
	shipped := "basic/.devcontainer/docker-compose.yml.tmpl"

	v1 := builtinForTest("1.0.0", map[string]string{shipped: "v1\n"})
	if _, err := EnsureUserConfig(dir, v1, "20260101-000000"); err != nil {
		t.Fatalf("v1: %v", err)
	}

	// User adds their own template, not part of the embedded set.
	mineDir := filepath.Join(dir, "templates", "mine", ".devcontainer")
	if err := os.MkdirAll(mineDir, 0o755); err != nil {
		t.Fatal(err)
	}
	minePath := filepath.Join(mineDir, "docker-compose.yml.tmpl")
	if err := os.WriteFile(minePath, []byte("custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	v2 := builtinForTest("2.0.0", map[string]string{shipped: "v2\n"})
	if _, err := EnsureUserConfig(dir, v2, "20260202-120000"); err != nil {
		t.Fatalf("v2: %v", err)
	}

	if got := readFile(t, minePath); got != "custom\n" {
		t.Errorf("user template was modified/removed: %q", got)
	}
}

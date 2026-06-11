// pattern: Imperative Shell

package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// templatesMarkerName is the file in the config directory recording the binary
// version whose templates are currently materialized in templates/.
const templatesMarkerName = ".templates-version"

// BuiltinAssets carries the defaults embedded in the binary. Templates is an
// fs.FS rooted at the templates directory (each immediate child is one
// template directory). Version is the running binary's version, used to detect
// when the materialized templates are stale.
type BuiltinAssets struct {
	Templates     fs.FS
	DefaultConfig []byte
	Version       string
}

// ProvisionResult reports what EnsureUserConfig changed, for the caller to log.
type ProvisionResult struct {
	ConfigSeeded    bool     // config.yaml was created
	TemplatesSynced bool     // templates were (re)written this run
	Written         []string // template files written (relative paths)
	BackedUp        []string // template files backed up before overwrite
	BackupDir       string   // where backups were written (empty if none)
}

// EnsureUserConfig materializes the embedded defaults into configDir on first
// run and refreshes the templates whenever the binary version changes.
//
//   - config.yaml is written only if absent (it holds user secrets/paths) and
//     is never overwritten on upgrade.
//   - templates are (re)written when the recorded version marker differs from
//     assets.Version. On-disk files that diverge from the embedded copy are
//     backed up under templates.backup-<now> before being overwritten. Files on
//     disk that are not part of the embedded set are left untouched, so
//     user-added templates survive upgrades.
//
// now is supplied by the caller (a timestamp label for the backup directory) to
// keep the backup location injectable for tests.
func EnsureUserConfig(configDir string, assets BuiltinAssets, now string) (ProvisionResult, error) {
	var res ProvisionResult

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return res, fmt.Errorf("failed to create config directory: %w", err)
	}

	seeded, err := seedConfig(configDir, assets.DefaultConfig)
	if err != nil {
		return res, err
	}
	res.ConfigSeeded = seeded

	if err := syncTemplates(configDir, assets, now, &res); err != nil {
		return res, err
	}
	return res, nil
}

// seedConfig writes the default config.yaml only when none exists.
func seedConfig(configDir string, defaultConfig []byte) (bool, error) {
	path := filepath.Join(configDir, "config.yaml")
	switch _, err := os.Stat(path); {
	case err == nil:
		return false, nil // present; never overwrite
	case !os.IsNotExist(err):
		return false, fmt.Errorf("failed to stat config file: %w", err)
	}
	if err := os.WriteFile(path, defaultConfig, 0o644); err != nil {
		return false, fmt.Errorf("failed to write default config: %w", err)
	}
	return true, nil
}

// syncTemplates refreshes templates/ when the version marker is stale.
func syncTemplates(configDir string, assets BuiltinAssets, now string, res *ProvisionResult) error {
	templatesDir := filepath.Join(configDir, "templates")
	markerPath := filepath.Join(configDir, templatesMarkerName)

	marker, err := readMarker(markerPath)
	if err != nil {
		return err
	}
	if !TemplatesNeedSync(marker, assets.Version) {
		return nil
	}

	embedded, err := readEmbeddedTemplates(assets.Templates)
	if err != nil {
		return err
	}
	onDisk, err := readDiskTemplates(templatesDir)
	if err != nil {
		return err
	}

	plan := PlanTemplateSync(embedded, onDisk)

	if len(plan.Backup) > 0 {
		res.BackupDir = filepath.Join(configDir, "templates.backup-"+now)
		for _, rel := range plan.Backup {
			if err := writeProfileFile(filepath.Join(res.BackupDir, filepath.FromSlash(rel)), onDisk[rel]); err != nil {
				return fmt.Errorf("failed to back up template %s: %w", rel, err)
			}
		}
	}
	for _, rel := range plan.Write {
		if err := writeProfileFile(filepath.Join(templatesDir, filepath.FromSlash(rel)), embedded[rel]); err != nil {
			return fmt.Errorf("failed to write template %s: %w", rel, err)
		}
	}

	if err := os.WriteFile(markerPath, []byte(assets.Version), 0o644); err != nil {
		return fmt.Errorf("failed to write templates version marker: %w", err)
	}

	res.TemplatesSynced = true
	res.Written = plan.Write
	res.BackedUp = plan.Backup
	return nil
}

// readMarker returns the recorded templates version, or "" when no marker
// exists (first run, or a profile created before markers existed).
func readMarker(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read templates version marker: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// readEmbeddedTemplates reads every embedded template file into a map keyed by
// forward-slash path relative to the templates root.
func readEmbeddedTemplates(fsys fs.FS) (map[string][]byte, error) {
	out := map[string][]byte{}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return fmt.Errorf("failed to read embedded template %s: %w", p, err)
		}
		out[p] = b
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk embedded templates: %w", err)
	}
	return out, nil
}

// readDiskTemplates reads the currently materialized templates into a map keyed
// by forward-slash path relative to dir. A missing dir yields an empty map.
func readDiskTemplates(dir string) (map[string][]byte, error) {
	out := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil // templates dir not created yet
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", rel, err)
		}
		out[filepath.ToSlash(rel)] = b
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk templates directory: %w", err)
	}
	return out, nil
}

// writeProfileFile writes content to path, creating parent directories.
func writeProfileFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

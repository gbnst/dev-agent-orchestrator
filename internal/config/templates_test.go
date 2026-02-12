package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTemplates_EmptyDir(t *testing.T) {
	tempDir := t.TempDir()
	templates, err := LoadTemplatesFrom(tempDir)
	if err != nil {
		t.Fatalf("LoadTemplatesFrom failed: %v", err)
	}
	if len(templates) != 0 {
		t.Errorf("Expected empty slice, got %d templates", len(templates))
	}
}

func TestLoadTemplates_SingleTemplate(t *testing.T) {
	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "basic")
	devcontainerDir := filepath.Join(templateDir, ".devcontainer")
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatalf("Failed to create .devcontainer directory: %v", err)
	}

	// Create marker file (docker-compose.yml.tmpl)
	composeContent := "services:\n  app:\n    build:\n      context: .\n"
	if err := os.WriteFile(filepath.Join(devcontainerDir, "docker-compose.yml.tmpl"), []byte(composeContent), 0644); err != nil {
		t.Fatalf("Failed to write docker-compose.yml.tmpl: %v", err)
	}

	templates, err := LoadTemplatesFrom(tempDir)
	if err != nil {
		t.Fatalf("LoadTemplatesFrom failed: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("Expected 1 template, got %d", len(templates))
	}

	tmpl := templates[0]
	if tmpl.Name != "basic" {
		t.Errorf("Name: got %q, want %q", tmpl.Name, "basic")
	}
	if tmpl.Path != templateDir {
		t.Errorf("Path: got %q, want %q", tmpl.Path, templateDir)
	}
}

func TestLoadTemplates_MultipleTemplates(t *testing.T) {
	tempDir := t.TempDir()

	for _, name := range []string{"basic", "go-project"} {
		dir := filepath.Join(tempDir, name)
		devcontainerDir := filepath.Join(dir, ".devcontainer")
		if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
			t.Fatalf("Failed to create .devcontainer directory for %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(devcontainerDir, "docker-compose.yml.tmpl"), []byte("services:\n  app:\n"), 0644); err != nil {
			t.Fatalf("Failed to write docker-compose.yml.tmpl for %s: %v", name, err)
		}
	}

	templates, err := LoadTemplatesFrom(tempDir)
	if err != nil {
		t.Fatalf("LoadTemplatesFrom failed: %v", err)
	}
	if len(templates) != 2 {
		t.Errorf("Expected 2 templates, got %d", len(templates))
	}

	names := make(map[string]bool)
	for _, tmpl := range templates {
		names[tmpl.Name] = true
	}
	if !names["basic"] || !names["go-project"] {
		t.Errorf("Expected basic and go-project templates, got: %v", names)
	}
}

func TestLoadTemplates_IgnoresFilesAndDirsWithoutMarker(t *testing.T) {
	tempDir := t.TempDir()

	// Create valid template directory (has docker-compose.yml.tmpl)
	validDir := filepath.Join(tempDir, "valid")
	validDevcontainerDir := filepath.Join(validDir, ".devcontainer")
	if err := os.MkdirAll(validDevcontainerDir, 0755); err != nil {
		t.Fatalf("Failed to create valid .devcontainer dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(validDevcontainerDir, "docker-compose.yml.tmpl"), []byte("services:\n  app:\n"), 0644); err != nil {
		t.Fatalf("Failed to write docker-compose.yml.tmpl: %v", err)
	}

	// Create directory without marker file
	emptyDir := filepath.Join(tempDir, "no-marker")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatalf("Failed to create empty dir: %v", err)
	}

	// Create a file at root level (should be ignored)
	if err := os.WriteFile(filepath.Join(tempDir, "readme.txt"), []byte("ignore me"), 0644); err != nil {
		t.Fatalf("Failed to write txt file: %v", err)
	}

	templates, err := LoadTemplatesFrom(tempDir)
	if err != nil {
		t.Fatalf("LoadTemplatesFrom failed: %v", err)
	}
	if len(templates) != 1 {
		t.Errorf("Expected 1 template (ignoring files and dirs without marker), got %d", len(templates))
	}
}

func TestLoadTemplates_DirNotExists(t *testing.T) {
	templates, err := LoadTemplatesFrom("/nonexistent/path/12345")
	if err != nil {
		t.Fatalf("LoadTemplatesFrom should not error for nonexistent dir: %v", err)
	}
	if len(templates) != 0 {
		t.Errorf("Expected empty slice for nonexistent dir, got %d", len(templates))
	}
}

func TestLoadTemplates_UsesDirectoryName(t *testing.T) {
	tempDir := t.TempDir()

	templateDir := filepath.Join(tempDir, "my-template")
	devcontainerDir := filepath.Join(templateDir, ".devcontainer")
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		t.Fatalf("Failed to create .devcontainer dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(devcontainerDir, "docker-compose.yml.tmpl"), []byte("services:\n  app:\n"), 0644); err != nil {
		t.Fatalf("Failed to write docker-compose.yml.tmpl: %v", err)
	}

	templates, err := LoadTemplatesFrom(tempDir)
	if err != nil {
		t.Fatalf("LoadTemplatesFrom failed: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("Expected 1 template, got %d", len(templates))
	}
	if templates[0].Name != "my-template" {
		t.Errorf("Name should be directory name: got %q, want %q", templates[0].Name, "my-template")
	}
}

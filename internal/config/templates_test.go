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
	templateContent := `
name: python-dev
description: Python development environment
base_image: python
devcontainer:
  features:
    ghcr.io/devcontainers/features/python:1:
      version: "3.11"
  customizations:
    vscode:
      extensions:
        - ms-python.python
  postCreateCommand: pip install -r requirements.txt
inject_credentials:
  - OPENAI_API_KEY
default_agent: claude-code
`
	templatePath := filepath.Join(tempDir, "python.yaml")
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	templates, err := LoadTemplatesFrom(tempDir)
	if err != nil {
		t.Fatalf("LoadTemplatesFrom failed: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("Expected 1 template, got %d", len(templates))
	}

	tmpl := templates[0]
	if tmpl.Name != "python-dev" {
		t.Errorf("Name: got %q, want %q", tmpl.Name, "python-dev")
	}
	if tmpl.Description != "Python development environment" {
		t.Errorf("Description: got %q, want %q", tmpl.Description, "Python development environment")
	}
	if tmpl.BaseImage != "python" {
		t.Errorf("BaseImage: got %q, want %q", tmpl.BaseImage, "python")
	}
	if tmpl.DefaultAgent != "claude-code" {
		t.Errorf("DefaultAgent: got %q, want %q", tmpl.DefaultAgent, "claude-code")
	}
	if len(tmpl.InjectCredentials) != 1 || tmpl.InjectCredentials[0] != "OPENAI_API_KEY" {
		t.Errorf("InjectCredentials: got %v, want [OPENAI_API_KEY]", tmpl.InjectCredentials)
	}
	if tmpl.Devcontainer.PostCreateCommand != "pip install -r requirements.txt" {
		t.Errorf("PostCreateCommand: got %q", tmpl.Devcontainer.PostCreateCommand)
	}
}

func TestLoadTemplates_MultipleTemplates(t *testing.T) {
	tempDir := t.TempDir()

	template1 := `
name: python-dev
description: Python development
base_image: python
`
	template2 := `
name: go-dev
description: Go development
base_image: go
`
	if err := os.WriteFile(filepath.Join(tempDir, "python.yaml"), []byte(template1), 0644); err != nil {
		t.Fatalf("Failed to write template1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "go.yaml"), []byte(template2), 0644); err != nil {
		t.Fatalf("Failed to write template2: %v", err)
	}

	templates, err := LoadTemplatesFrom(tempDir)
	if err != nil {
		t.Fatalf("LoadTemplatesFrom failed: %v", err)
	}
	if len(templates) != 2 {
		t.Errorf("Expected 2 templates, got %d", len(templates))
	}

	// Verify both templates loaded (order may vary)
	names := make(map[string]bool)
	for _, tmpl := range templates {
		names[tmpl.Name] = true
	}
	if !names["python-dev"] || !names["go-dev"] {
		t.Errorf("Expected python-dev and go-dev templates, got: %v", names)
	}
}

func TestLoadTemplates_IgnoresNonYaml(t *testing.T) {
	tempDir := t.TempDir()

	template := `
name: python-dev
description: Python development
base_image: python
`
	if err := os.WriteFile(filepath.Join(tempDir, "python.yaml"), []byte(template), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "readme.txt"), []byte("ignore me"), 0644); err != nil {
		t.Fatalf("Failed to write txt file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "notes.md"), []byte("# notes"), 0644); err != nil {
		t.Fatalf("Failed to write md file: %v", err)
	}

	templates, err := LoadTemplatesFrom(tempDir)
	if err != nil {
		t.Fatalf("LoadTemplatesFrom failed: %v", err)
	}
	if len(templates) != 1 {
		t.Errorf("Expected 1 template (ignoring non-yaml), got %d", len(templates))
	}
}

func TestLoadTemplates_InvalidYaml(t *testing.T) {
	tempDir := t.TempDir()

	validTemplate := `
name: python-dev
description: Python development
base_image: python
`
	invalidYaml := `
name: broken
  description: this is malformed yaml
  base_image: [
`
	if err := os.WriteFile(filepath.Join(tempDir, "valid.yaml"), []byte(validTemplate), 0644); err != nil {
		t.Fatalf("Failed to write valid template: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "invalid.yaml"), []byte(invalidYaml), 0644); err != nil {
		t.Fatalf("Failed to write invalid template: %v", err)
	}

	// Should still load valid templates, logging warning for invalid
	templates, err := LoadTemplatesFrom(tempDir)
	if err != nil {
		t.Fatalf("LoadTemplatesFrom failed: %v", err)
	}
	if len(templates) != 1 {
		t.Errorf("Expected 1 valid template, got %d", len(templates))
	}
	if templates[0].Name != "python-dev" {
		t.Errorf("Expected python-dev template, got %q", templates[0].Name)
	}
}

func TestLoadTemplates_YmlExtension(t *testing.T) {
	tempDir := t.TempDir()

	template := `
name: python-dev
description: Python development
base_image: python
`
	if err := os.WriteFile(filepath.Join(tempDir, "python.yml"), []byte(template), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	templates, err := LoadTemplatesFrom(tempDir)
	if err != nil {
		t.Fatalf("LoadTemplatesFrom failed: %v", err)
	}
	if len(templates) != 1 {
		t.Errorf("Expected 1 template for .yml extension, got %d", len(templates))
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

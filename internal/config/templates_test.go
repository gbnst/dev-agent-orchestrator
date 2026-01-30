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
	pythonDir := filepath.Join(tempDir, "python-dev")
	if err := os.Mkdir(pythonDir, 0755); err != nil {
		t.Fatalf("Failed to create template directory: %v", err)
	}

	templateContent := `{
  "name": "python-dev",
  "image": "python",
  "features": {
    "ghcr.io/devcontainers/features/python:1": {
      "version": "3.11"
    }
  },
  "customizations": {
    "vscode": {
      "extensions": ["ms-python.python"]
    },
    "devagent": {
      "injectCredentials": ["OPENAI_API_KEY"],
      "defaultAgent": "claude-code"
    }
  },
  "postCreateCommand": "pip install -r requirements.txt"
}`
	templatePath := filepath.Join(pythonDir, "devcontainer.json")
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
	if tmpl.Image != "python" {
		t.Errorf("Image: got %q, want %q", tmpl.Image, "python")
	}
	if tmpl.DefaultAgent != "claude-code" {
		t.Errorf("DefaultAgent: got %q, want %q", tmpl.DefaultAgent, "claude-code")
	}
	if len(tmpl.InjectCredentials) != 1 || tmpl.InjectCredentials[0] != "OPENAI_API_KEY" {
		t.Errorf("InjectCredentials: got %v, want [OPENAI_API_KEY]", tmpl.InjectCredentials)
	}
	if tmpl.PostCreateCommand != "pip install -r requirements.txt" {
		t.Errorf("PostCreateCommand: got %q", tmpl.PostCreateCommand)
	}
}

func TestLoadTemplates_MultipleTemplates(t *testing.T) {
	tempDir := t.TempDir()

	// Create python-dev template
	pythonDir := filepath.Join(tempDir, "python-dev")
	if err := os.Mkdir(pythonDir, 0755); err != nil {
		t.Fatalf("Failed to create python-dev directory: %v", err)
	}
	template1 := `{"name": "python-dev", "image": "python"}`
	if err := os.WriteFile(filepath.Join(pythonDir, "devcontainer.json"), []byte(template1), 0644); err != nil {
		t.Fatalf("Failed to write template1: %v", err)
	}

	// Create go-dev template
	goDir := filepath.Join(tempDir, "go-dev")
	if err := os.Mkdir(goDir, 0755); err != nil {
		t.Fatalf("Failed to create go-dev directory: %v", err)
	}
	template2 := `{"name": "go-dev", "image": "go"}`
	if err := os.WriteFile(filepath.Join(goDir, "devcontainer.json"), []byte(template2), 0644); err != nil {
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

func TestLoadTemplates_IgnoresFilesAndEmptyDirs(t *testing.T) {
	tempDir := t.TempDir()

	// Create valid template directory
	validDir := filepath.Join(tempDir, "python-dev")
	if err := os.MkdirAll(validDir, 0755); err != nil {
		t.Fatalf("Failed to create valid dir: %v", err)
	}
	validJSON := `{"name": "python-dev", "image": "python:3.11"}`
	if err := os.WriteFile(filepath.Join(validDir, "devcontainer.json"), []byte(validJSON), 0644); err != nil {
		t.Fatalf("Failed to write devcontainer.json: %v", err)
	}

	// Create empty directory (no devcontainer.json)
	emptyDir := filepath.Join(tempDir, "empty-template")
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
		t.Errorf("Expected 1 template (ignoring files and empty dirs), got %d", len(templates))
	}
}

func TestLoadTemplates_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()

	// Create valid template directory
	validDir := filepath.Join(tempDir, "valid")
	if err := os.MkdirAll(validDir, 0755); err != nil {
		t.Fatalf("Failed to create valid dir: %v", err)
	}
	validJSON := `{"name": "valid", "image": "python:3.11"}`
	if err := os.WriteFile(filepath.Join(validDir, "devcontainer.json"), []byte(validJSON), 0644); err != nil {
		t.Fatalf("Failed to write valid devcontainer.json: %v", err)
	}

	// Create invalid template directory
	invalidDir := filepath.Join(tempDir, "invalid")
	if err := os.MkdirAll(invalidDir, 0755); err != nil {
		t.Fatalf("Failed to create invalid dir: %v", err)
	}
	invalidJSON := `{"name": "broken", "image": [` // malformed JSON
	if err := os.WriteFile(filepath.Join(invalidDir, "devcontainer.json"), []byte(invalidJSON), 0644); err != nil {
		t.Fatalf("Failed to write invalid devcontainer.json: %v", err)
	}

	// Should still load valid templates, logging warning for invalid
	templates, err := LoadTemplatesFrom(tempDir)
	if err != nil {
		t.Fatalf("LoadTemplatesFrom failed: %v", err)
	}
	if len(templates) != 1 {
		t.Errorf("Expected 1 valid template, got %d", len(templates))
	}
	if templates[0].Name != "valid" {
		t.Errorf("Expected valid template, got %q", templates[0].Name)
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

func TestLoadTemplates_UsesDirectoryNameWhenNameOmitted(t *testing.T) {
	tempDir := t.TempDir()

	// Create template directory with JSON that omits name
	templateDir := filepath.Join(tempDir, "my-template")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatalf("Failed to create template dir: %v", err)
	}
	// Note: no "name" field in JSON
	templateJSON := `{"image": "python:3.11"}`
	if err := os.WriteFile(filepath.Join(templateDir, "devcontainer.json"), []byte(templateJSON), 0644); err != nil {
		t.Fatalf("Failed to write devcontainer.json: %v", err)
	}

	templates, err := LoadTemplatesFrom(tempDir)
	if err != nil {
		t.Fatalf("LoadTemplatesFrom failed: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("Expected 1 template, got %d", len(templates))
	}
	if templates[0].Name != "my-template" {
		t.Errorf("Name should default to directory name: got %q, want %q", templates[0].Name, "my-template")
	}
}

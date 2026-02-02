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

func TestLoadTemplate_IsolationConfig(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantNil  bool
		validate func(t *testing.T, iso *IsolationConfig)
	}{
		{
			name:    "no isolation config returns nil",
			json:    `{"name": "test"}`,
			wantNil: true,
		},
		{
			name:    "no devagent customizations returns nil",
			json:    `{"name": "test", "customizations": {}}`,
			wantNil: true,
		},
		{
			name: "enabled false",
			json: `{
				"name": "test",
				"customizations": {
					"devagent": {
						"isolation": {
							"enabled": false
						}
					}
				}
			}`,
			wantNil: false,
			validate: func(t *testing.T, iso *IsolationConfig) {
				if iso.Enabled == nil {
					t.Error("Enabled should not be nil")
					return
				}
				if *iso.Enabled != false {
					t.Errorf("Enabled = %v, want false", *iso.Enabled)
				}
			},
		},
		{
			name: "caps drop and add",
			json: `{
				"name": "test",
				"customizations": {
					"devagent": {
						"isolation": {
							"caps": {
								"drop": ["NET_RAW", "SYS_ADMIN"],
								"add": ["NET_BIND_SERVICE"]
							}
						}
					}
				}
			}`,
			wantNil: false,
			validate: func(t *testing.T, iso *IsolationConfig) {
				if iso.Caps == nil {
					t.Fatal("Caps should not be nil")
				}
				if len(iso.Caps.Drop) != 2 {
					t.Errorf("Caps.Drop len = %d, want 2", len(iso.Caps.Drop))
				}
				if iso.Caps.Drop[0] != "NET_RAW" {
					t.Errorf("Caps.Drop[0] = %q, want %q", iso.Caps.Drop[0], "NET_RAW")
				}
				if len(iso.Caps.Add) != 1 {
					t.Errorf("Caps.Add len = %d, want 1", len(iso.Caps.Add))
				}
				if iso.Caps.Add[0] != "NET_BIND_SERVICE" {
					t.Errorf("Caps.Add[0] = %q, want %q", iso.Caps.Add[0], "NET_BIND_SERVICE")
				}
			},
		},
		{
			name: "resource limits",
			json: `{
				"name": "test",
				"customizations": {
					"devagent": {
						"isolation": {
							"resources": {
								"memory": "2g",
								"cpus": "1.5",
								"pidsLimit": 256
							}
						}
					}
				}
			}`,
			wantNil: false,
			validate: func(t *testing.T, iso *IsolationConfig) {
				if iso.Resources == nil {
					t.Fatal("Resources should not be nil")
				}
				if iso.Resources.Memory != "2g" {
					t.Errorf("Resources.Memory = %q, want %q", iso.Resources.Memory, "2g")
				}
				if iso.Resources.CPUs != "1.5" {
					t.Errorf("Resources.CPUs = %q, want %q", iso.Resources.CPUs, "1.5")
				}
				if iso.Resources.PidsLimit != 256 {
					t.Errorf("Resources.PidsLimit = %d, want %d", iso.Resources.PidsLimit, 256)
				}
			},
		},
		{
			name: "network allowlist and passthrough",
			json: `{
				"name": "test",
				"customizations": {
					"devagent": {
						"isolation": {
							"network": {
								"allowlist": ["github.com", "api.anthropic.com"],
								"allowlistExtend": ["custom.example.com"],
								"passthrough": ["pinned.example.com"]
							}
						}
					}
				}
			}`,
			wantNil: false,
			validate: func(t *testing.T, iso *IsolationConfig) {
				if iso.Network == nil {
					t.Fatal("Network should not be nil")
				}
				if len(iso.Network.Allowlist) != 2 {
					t.Errorf("Network.Allowlist len = %d, want 2", len(iso.Network.Allowlist))
				}
				if len(iso.Network.AllowlistExtend) != 1 {
					t.Errorf("Network.AllowlistExtend len = %d, want 1", len(iso.Network.AllowlistExtend))
				}
				if len(iso.Network.Passthrough) != 1 {
					t.Errorf("Network.Passthrough len = %d, want 1", len(iso.Network.Passthrough))
				}
			},
		},
		{
			name: "full isolation config",
			json: `{
				"name": "test",
				"customizations": {
					"devagent": {
						"isolation": {
							"enabled": true,
							"caps": {"drop": ["NET_RAW"]},
							"resources": {"memory": "4g"},
							"network": {"allowlist": ["github.com"]}
						}
					}
				}
			}`,
			wantNil: false,
			validate: func(t *testing.T, iso *IsolationConfig) {
				if iso.Enabled == nil || *iso.Enabled != true {
					t.Error("Enabled should be true")
				}
				if iso.Caps == nil || len(iso.Caps.Drop) != 1 {
					t.Error("Caps.Drop should have 1 element")
				}
				if iso.Resources == nil || iso.Resources.Memory != "4g" {
					t.Error("Resources.Memory should be 4g")
				}
				if iso.Network == nil || len(iso.Network.Allowlist) != 1 {
					t.Error("Network.Allowlist should have 1 element")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			templateDir := filepath.Join(tmpDir, "test-template")
			if err := os.MkdirAll(templateDir, 0755); err != nil {
				t.Fatalf("failed to create template dir: %v", err)
			}

			devcontainerPath := filepath.Join(templateDir, "devcontainer.json")
			if err := os.WriteFile(devcontainerPath, []byte(tt.json), 0644); err != nil {
				t.Fatalf("failed to write devcontainer.json: %v", err)
			}

			templates, err := LoadTemplatesFrom(tmpDir)
			if err != nil {
				t.Fatalf("LoadTemplatesFrom() error = %v", err)
			}
			if len(templates) != 1 {
				t.Fatalf("LoadTemplatesFrom() returned %d templates, want 1", len(templates))
			}
			tmpl := templates[0]

			if tt.wantNil {
				if tmpl.Isolation != nil {
					t.Errorf("Isolation = %+v, want nil", tmpl.Isolation)
				}
				return
			}

			if tmpl.Isolation == nil {
				t.Fatal("Isolation is nil, want non-nil")
			}

			if tt.validate != nil {
				tt.validate(t, tmpl.Isolation)
			}
		})
	}
}

func TestDefaultIsolation(t *testing.T) {
	// Verify DefaultIsolation has expected secure defaults
	if DefaultIsolation == nil {
		t.Fatal("DefaultIsolation is nil")
	}

	// Check caps defaults
	if DefaultIsolation.Caps == nil {
		t.Fatal("DefaultIsolation.Caps is nil")
	}
	if len(DefaultIsolation.Caps.Drop) == 0 {
		t.Error("DefaultIsolation.Caps.Drop should not be empty")
	}

	// Check for critical capability drops
	wantDropped := []string{"NET_RAW", "SYS_ADMIN", "SYS_PTRACE"}
	for _, cap := range wantDropped {
		found := false
		for _, d := range DefaultIsolation.Caps.Drop {
			if d == cap {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DefaultIsolation.Caps.Drop should contain %q", cap)
		}
	}

	// Check resource defaults
	if DefaultIsolation.Resources == nil {
		t.Fatal("DefaultIsolation.Resources is nil")
	}
	if DefaultIsolation.Resources.Memory == "" {
		t.Error("DefaultIsolation.Resources.Memory should not be empty")
	}
	if DefaultIsolation.Resources.CPUs == "" {
		t.Error("DefaultIsolation.Resources.CPUs should not be empty")
	}
	if DefaultIsolation.Resources.PidsLimit == 0 {
		t.Error("DefaultIsolation.Resources.PidsLimit should not be 0")
	}

	// Check network defaults
	if DefaultIsolation.Network == nil {
		t.Fatal("DefaultIsolation.Network is nil")
	}
	if len(DefaultIsolation.Network.Allowlist) == 0 {
		t.Error("DefaultIsolation.Network.Allowlist should not be empty")
	}

	// Check for critical domains in allowlist
	wantAllowed := []string{"api.anthropic.com", "github.com"}
	for _, domain := range wantAllowed {
		found := false
		for _, a := range DefaultIsolation.Network.Allowlist {
			if a == domain {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DefaultIsolation.Network.Allowlist should contain %q", domain)
		}
	}
}

func TestMergeIsolationConfig(t *testing.T) {
	defaults := &IsolationConfig{
		Caps: &CapConfig{
			Drop: []string{"NET_RAW", "SYS_ADMIN"},
		},
		Resources: &ResourceConfig{
			Memory:    "4g",
			CPUs:      "2",
			PidsLimit: 512,
		},
		Network: &NetworkConfig{
			Allowlist: []string{"github.com", "api.anthropic.com"},
		},
	}

	tests := []struct {
		name     string
		template *IsolationConfig
		validate func(t *testing.T, merged *IsolationConfig)
	}{
		{
			name:     "nil template returns copy of defaults",
			template: nil,
			validate: func(t *testing.T, merged *IsolationConfig) {
				if merged == nil {
					t.Fatal("merged should not be nil")
				}
				if len(merged.Caps.Drop) != 2 {
					t.Errorf("should have 2 cap drops, got %d", len(merged.Caps.Drop))
				}
				if merged.Resources.Memory != "4g" {
					t.Errorf("memory = %q, want %q", merged.Resources.Memory, "4g")
				}
			},
		},
		{
			name: "enabled false returns nil",
			template: func() *IsolationConfig {
				enabled := false
				return &IsolationConfig{Enabled: &enabled}
			}(),
			validate: func(t *testing.T, merged *IsolationConfig) {
				if merged != nil {
					t.Errorf("enabled=false should return nil, got %+v", merged)
				}
			},
		},
		{
			name: "template caps override defaults",
			template: &IsolationConfig{
				Caps: &CapConfig{
					Drop: []string{"MKNOD"},
				},
			},
			validate: func(t *testing.T, merged *IsolationConfig) {
				if len(merged.Caps.Drop) != 1 || merged.Caps.Drop[0] != "MKNOD" {
					t.Errorf("caps.drop should be [MKNOD], got %v", merged.Caps.Drop)
				}
				// Resources should still come from defaults
				if merged.Resources.Memory != "4g" {
					t.Errorf("memory should be 4g from defaults, got %q", merged.Resources.Memory)
				}
			},
		},
		{
			name: "template resources override defaults",
			template: &IsolationConfig{
				Resources: &ResourceConfig{
					Memory: "8g",
				},
			},
			validate: func(t *testing.T, merged *IsolationConfig) {
				if merged.Resources.Memory != "8g" {
					t.Errorf("memory = %q, want %q", merged.Resources.Memory, "8g")
				}
				// CPUs and PidsLimit should come from defaults
				if merged.Resources.CPUs != "2" {
					t.Errorf("cpus should be 2 from defaults, got %q", merged.Resources.CPUs)
				}
			},
		},
		{
			name: "allowlist replaces default allowlist",
			template: &IsolationConfig{
				Network: &NetworkConfig{
					Allowlist: []string{"custom.example.com"},
				},
			},
			validate: func(t *testing.T, merged *IsolationConfig) {
				if len(merged.Network.Allowlist) != 1 || merged.Network.Allowlist[0] != "custom.example.com" {
					t.Errorf("allowlist should be [custom.example.com], got %v", merged.Network.Allowlist)
				}
			},
		},
		{
			name: "allowlistExtend appends to default allowlist",
			template: &IsolationConfig{
				Network: &NetworkConfig{
					AllowlistExtend: []string{"extra.example.com"},
				},
			},
			validate: func(t *testing.T, merged *IsolationConfig) {
				// Should have defaults + extended
				if len(merged.Network.Allowlist) != 3 {
					t.Errorf("allowlist should have 3 entries, got %d: %v", len(merged.Network.Allowlist), merged.Network.Allowlist)
				}
				// Check that extra.example.com is included
				found := false
				for _, domain := range merged.Network.Allowlist {
					if domain == "extra.example.com" {
						found = true
						break
					}
				}
				if !found {
					t.Error("allowlist should include extra.example.com")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := MergeIsolationConfig(tt.template, defaults)
			tt.validate(t, merged)
		})
	}
}

func TestCopyIsolationConfig(t *testing.T) {
	original := &IsolationConfig{
		Caps: &CapConfig{
			Drop: []string{"NET_RAW"},
			Add:  []string{"NET_BIND_SERVICE"},
		},
		Resources: &ResourceConfig{
			Memory:    "4g",
			CPUs:      "2",
			PidsLimit: 512,
		},
		Network: &NetworkConfig{
			Allowlist:   []string{"github.com"},
			Passthrough: []string{"pinned.example.com"},
		},
	}

	copied := copyIsolationConfig(original)

	// Modify original
	original.Caps.Drop[0] = "MODIFIED"
	original.Resources.Memory = "MODIFIED"
	original.Network.Allowlist[0] = "MODIFIED"

	// Copy should be unchanged
	if copied.Caps.Drop[0] == "MODIFIED" {
		t.Error("copy should be independent of original")
	}
	if copied.Resources.Memory == "MODIFIED" {
		t.Error("copy resources should be independent")
	}
	if copied.Network.Allowlist[0] == "MODIFIED" {
		t.Error("copy network allowlist should be independent")
	}
}

func TestTemplate_GetEffectiveIsolation(t *testing.T) {
	tests := []struct {
		name     string
		template Template
		wantNil  bool
	}{
		{
			name:     "no isolation uses defaults",
			template: Template{Name: "test", Isolation: nil},
			wantNil:  false,
		},
		{
			name: "explicit enabled true uses merged",
			template: Template{
				Name: "test",
				Isolation: func() *IsolationConfig {
					enabled := true
					return &IsolationConfig{Enabled: &enabled}
				}(),
			},
			wantNil: false,
		},
		{
			name: "explicit enabled false returns nil",
			template: Template{
				Name: "test",
				Isolation: func() *IsolationConfig {
					enabled := false
					return &IsolationConfig{Enabled: &enabled}
				}(),
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.template.GetEffectiveIsolation()
			if tt.wantNil && result != nil {
				t.Errorf("GetEffectiveIsolation() = %+v, want nil", result)
			}
			if !tt.wantNil && result == nil {
				t.Error("GetEffectiveIsolation() = nil, want non-nil")
			}
		})
	}
}

func TestTemplate_IsIsolationEnabled(t *testing.T) {
	tests := []struct {
		name     string
		template Template
		want     bool
	}{
		{
			name:     "nil isolation is enabled by default",
			template: Template{Name: "test", Isolation: nil},
			want:     true,
		},
		{
			name: "nil enabled field is enabled by default",
			template: Template{
				Name:      "test",
				Isolation: &IsolationConfig{Enabled: nil},
			},
			want: true,
		},
		{
			name: "explicit enabled true",
			template: Template{
				Name: "test",
				Isolation: func() *IsolationConfig {
					enabled := true
					return &IsolationConfig{Enabled: &enabled}
				}(),
			},
			want: true,
		},
		{
			name: "explicit enabled false",
			template: Template{
				Name: "test",
				Isolation: func() *IsolationConfig {
					enabled := false
					return &IsolationConfig{Enabled: &enabled}
				}(),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.template.IsIsolationEnabled()
			if got != tt.want {
				t.Errorf("IsIsolationEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

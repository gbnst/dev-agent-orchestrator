// pattern: Functional Core

package container

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// TestParsePortEnvVarsFromContent tests the pure parsing function with various inputs
func TestParsePortEnvVarsFromContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected map[string]string
	}{
		{
			name:     "single variable",
			content:  `ports:\n  - "${APP_PORT:-8000}:8000"`,
			expected: map[string]string{"APP_PORT": "8000"},
		},
		{
			name:     "multiple variables",
			content:  `ports:\n  - "${APP_PORT:-8000}:8000"\n  - "${WEB_PORT:-3000}:3000"`,
			expected: map[string]string{"APP_PORT": "8000", "WEB_PORT": "3000"},
		},
		{
			name:     "mixed hardcoded and variables",
			content:  `ports:\n  - "${APP_PORT:-8000}:8000"\n  - "5432:5432"`,
			expected: map[string]string{"APP_PORT": "8000"},
		},
		{
			name:     "no variables",
			content:  `ports:\n  - "8000:8000"\n  - "3000:3000"`,
			expected: map[string]string{},
		},
		{
			name:     "empty content",
			content:  "",
			expected: map[string]string{},
		},
		{
			name:     "variable with different defaults",
			content:  `ports:\n  - "${SERVER_PORT:-9999}:9999"`,
			expected: map[string]string{"SERVER_PORT": "9999"},
		},
		{
			name:     "multiple occurrences same variable",
			content:  `ports:\n  - "${APP_PORT:-8000}:8000"\nenvironment:\n  - APP_PORT=${APP_PORT:-8000}`,
			expected: map[string]string{"APP_PORT": "8000"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsePortEnvVarsFromContent(tt.content)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d vars, got %d", len(tt.expected), len(result))
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("expected %s=%s, got %s=%s", k, v, k, result[k])
				}
			}
		})
	}
}

// TestParsePortEnvVars tests the file I/O path
func TestParsePortEnvVars(t *testing.T) {
	t.Run("with real temp file", func(t *testing.T) {
		tmpDir := t.TempDir()
		composeFile := filepath.Join(tmpDir, "docker-compose.yml")
		content := `version: '3'
services:
  app:
    ports:
      - "${APP_PORT:-8000}:8000"
      - "${WEB_PORT:-3000}:3000"
`
		if err := os.WriteFile(composeFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write temp compose file: %v", err)
		}

		result, err := ParsePortEnvVars(composeFile)
		if err != nil {
			t.Fatalf("ParsePortEnvVars failed: %v", err)
		}

		if len(result) != 2 {
			t.Errorf("expected 2 vars, got %d", len(result))
		}
		if result["APP_PORT"] != "8000" {
			t.Errorf("expected APP_PORT=8000, got %s", result["APP_PORT"])
		}
		if result["WEB_PORT"] != "3000" {
			t.Errorf("expected WEB_PORT=3000, got %s", result["WEB_PORT"])
		}
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		_, err := ParsePortEnvVars("/nonexistent/path/docker-compose.yml")
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})
}

// TestAllocateFreePorts tests port allocation with various scenarios
func TestAllocateFreePorts(t *testing.T) {
	t.Run("allocates valid ports", func(t *testing.T) {
		portVars := map[string]string{
			"APP_PORT": "8000",
			"WEB_PORT": "3000",
		}

		result, err := AllocateFreePorts(portVars)
		if err != nil {
			t.Fatalf("AllocateFreePorts failed: %v", err)
		}

		if len(result) != 2 {
			t.Errorf("expected 2 allocated ports, got %d", len(result))
		}

		for varName, portStr := range result {
			port, err := strconv.Atoi(portStr)
			if err != nil {
				t.Errorf("allocated port for %s is not a valid integer: %s", varName, portStr)
			}
			if port <= 0 || port > 65535 {
				t.Errorf("allocated port for %s is out of valid range: %d", varName, port)
			}
			defaultPort, _ := strconv.Atoi(portVars[varName])
			// Port should differ from default (not strictly required, but highly likely)
			// We just verify it's a valid port number
			_ = defaultPort
		}
	})

	t.Run("all allocated ports are distinct", func(t *testing.T) {
		portVars := map[string]string{
			"APP_PORT":   "8000",
			"WEB_PORT":   "3000",
			"DEBUG_PORT": "9000",
		}

		result, err := AllocateFreePorts(portVars)
		if err != nil {
			t.Fatalf("AllocateFreePorts failed: %v", err)
		}

		if len(result) != 3 {
			t.Errorf("expected 3 allocated ports, got %d", len(result))
		}

		// Check all ports are unique
		ports := make(map[string]bool)
		for _, portStr := range result {
			if ports[portStr] {
				t.Errorf("duplicate port allocated: %s", portStr)
			}
			ports[portStr] = true
		}
	})

	t.Run("empty input returns empty map", func(t *testing.T) {
		result, err := AllocateFreePorts(map[string]string{})
		if err != nil {
			t.Fatalf("AllocateFreePorts failed: %v", err)
		}

		if len(result) != 0 {
			t.Errorf("expected empty map, got %d entries", len(result))
		}
	})
}

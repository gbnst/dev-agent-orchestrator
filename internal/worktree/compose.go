// pattern: Imperative Shell

package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteComposeOverride writes a docker-compose.worktree.yml override file that adds:
// 1. A unique top-level `name:` for compose project isolation
// 2. A volume mount for the host .git directory (so worktree gitdir resolves inside container)
//
// This replaces the previous approach of patching docker-compose.yml in-place,
// leaving the original compose file untouched.
func WriteComposeOverride(projectPath, wtDir, name string) error {
	projectName := filepath.Base(projectPath)
	composeName := sanitizeComposeName(projectName + "-" + name)
	gitMount := fmt.Sprintf("%s/.git:%s/.git:cached", projectPath, projectPath)

	content := fmt.Sprintf(`name: %s
services:
  app:
    volumes:
      - %s
`, composeName, gitMount)

	overridePath := filepath.Join(wtDir, ".devcontainer", "docker-compose.worktree.yml")
	if err := os.WriteFile(overridePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing compose override: %w", err)
	}

	return nil
}

// sanitizeComposeName converts a name to a valid Docker Compose project name.
// Compose names must be lowercase, alphanumeric with hyphens.
func sanitizeComposeName(name string) string {
	name = strings.ToLower(name)
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, name)
	// Remove leading/trailing hyphens
	name = strings.Trim(name, "-")
	return name
}

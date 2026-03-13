// pattern: Functional Core

package container

import (
	"fmt"
	"os"
	"regexp"
)

// portEnvVarPattern matches Docker Compose variable interpolation in port mappings.
// Example: ${APP_PORT:-8000} captures var name "APP_PORT" and default "8000".
var portEnvVarPattern = regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*):-(\d+)\}`)

// ParsePortEnvVars reads a docker-compose.yml file and extracts environment
// variable port patterns from port mapping strings. Returns a map of env var
// name to default port value.
func ParsePortEnvVars(composeFilePath string) (map[string]string, error) {
	content, err := os.ReadFile(composeFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read compose file: %w", err)
	}
	return parsePortEnvVarsFromContent(string(content)), nil
}

// parsePortEnvVarsFromContent extracts port env var patterns from compose file content.
// Pure function for testability.
func parsePortEnvVarsFromContent(content string) map[string]string {
	result := make(map[string]string)
	matches := portEnvVarPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		varName := match[1]
		defaultVal := match[2]
		result[varName] = defaultVal
	}
	return result
}

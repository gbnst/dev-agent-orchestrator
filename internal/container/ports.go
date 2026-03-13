// pattern: Functional Core

package container

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
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

// AllocateFreePorts finds free TCP ports on the host for each port variable.
// Takes a map of env var name to default value (from ParsePortEnvVars) and returns
// a map of env var name to allocated free port as string.
func AllocateFreePorts(portVars map[string]string) (map[string]string, error) {
	result := make(map[string]string, len(portVars))
	for varName := range portVars {
		port, err := findFreePort()
		if err != nil {
			return nil, fmt.Errorf("failed to allocate port for %s: %w", varName, err)
		}
		result[varName] = strconv.Itoa(port)
	}
	return result, nil
}

// findFreePort asks the OS for a free TCP port by binding to :0.
func findFreePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, fmt.Errorf("failed to find free port: %w", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// SanitizeComposeName normalises a name into a valid Docker Compose project name.
// Lowercase, non-alphanumeric characters replaced with hyphens, trimmed.
func SanitizeComposeName(name string) string {
	name = strings.ToLower(name)
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, name)
	name = strings.Trim(name, "-")
	return name
}

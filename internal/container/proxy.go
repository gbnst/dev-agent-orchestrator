// pattern: Functional Core

package container

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// projectHash returns a truncated SHA256 hash of the project path (12 chars).
func projectHash(projectPath string) string {
	hash := sha256.Sum256([]byte(projectPath))
	return hex.EncodeToString(hash[:])[:12]
}

// GetProxyConfigDir returns the directory for proxy configuration files.
// Creates the directory if it doesn't exist.
// Uses a hash of the project path for uniqueness.
func GetProxyConfigDir(projectPath string) (string, error) {
	hashStr := projectHash(projectPath)
	configDir := filepath.Join(getDataDir(), "proxy-configs", hashStr)

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create proxy config directory: %w", err)
	}

	return configDir, nil
}

// GetProxyCertDir returns the directory for proxy certificate files.
// Creates the directory if it doesn't exist.
// Uses a hash of the project path for uniqueness.
func GetProxyCertDir(projectPath string) (string, error) {
	hashStr := projectHash(projectPath)
	certDir := filepath.Join(getDataDir(), "proxy-certs", hashStr)

	if err := os.MkdirAll(certDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create proxy cert directory: %w", err)
	}

	return certDir, nil
}

// GetProxyCACertPath returns the path to the mitmproxy CA certificate file.
// This is where mitmproxy will generate the cert on first run.
func GetProxyCACertPath(projectPath string) (string, error) {
	certDir, err := GetProxyCertDir(projectPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(certDir, "mitmproxy-ca-cert.pem"), nil
}

// ProxyCertExists checks if the mitmproxy CA certificate has been generated.
func ProxyCertExists(projectPath string) (bool, error) {
	certPath, err := GetProxyCACertPath(projectPath)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(certPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ReadAllowlistFromFilterScript reads the allowlist domains from an existing filter script.
// Returns nil if the file doesn't exist or can't be parsed.
func ReadAllowlistFromFilterScript(projectPath string) ([]string, error) {
	configDir, err := GetProxyConfigDir(projectPath)
	if err != nil {
		return nil, err
	}

	scriptPath := filepath.Join(configDir, "filter.py")
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No filter script exists
		}
		return nil, err
	}

	return parseAllowlistFromScript(string(content)), nil
}

// parseAllowlistFromScript extracts domain strings from the ALLOWED_DOMAINS array.
// The format is:    "domain.com", (one per line, with quotes and comma)
func parseAllowlistFromScript(content string) []string {
	var domains []string

	// Find the ALLOWED_DOMAINS = [ ... ] section
	startMarker := "ALLOWED_DOMAINS = ["
	startIdx := strings.Index(content, startMarker)
	if startIdx == -1 {
		return domains
	}

	// Find the closing bracket
	endIdx := strings.Index(content[startIdx:], "]")
	if endIdx == -1 {
		return domains
	}

	// Extract the content between [ and ]
	arrayContent := content[startIdx+len(startMarker) : startIdx+endIdx]

	// Parse each line looking for quoted strings
	for _, line := range strings.Split(arrayContent, "\n") {
		line = strings.TrimSpace(line)
		// Look for quoted strings like "domain.com",
		if strings.HasPrefix(line, "\"") {
			// Find the closing quote
			endQuote := strings.Index(line[1:], "\"")
			if endQuote > 0 {
				domain := line[1 : endQuote+1]
				domains = append(domains, domain)
			}
		}
	}

	return domains
}

// CleanupProxyConfigs removes the proxy configuration directories for a project.
// Called when a container is destroyed to clean up associated resources.
func CleanupProxyConfigs(projectPath string) error {
	hashStr := projectHash(projectPath)

	// Remove proxy-configs directory
	configDir := filepath.Join(getDataDir(), "proxy-configs", hashStr)
	if err := os.RemoveAll(configDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove proxy config directory: %w", err)
	}

	// Remove proxy-certs directory
	certDir := filepath.Join(getDataDir(), "proxy-certs", hashStr)
	if err := os.RemoveAll(certDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove proxy cert directory: %w", err)
	}

	return nil
}

// pattern: Functional Core

package container

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const filterScriptTemplate = `from mitmproxy import http
import re

# Allowlist of permitted domains
# Wildcards are supported: "*.github.com" matches "api.github.com"
ALLOWED_DOMAINS = [
%s]

# Block GitHub PR merge API calls
BLOCK_GITHUB_PR_MERGE = %s

class AllowlistFilter:
    """Blocks requests to domains not in the allowlist and optionally blocks PR merges."""

    def _is_allowed(self, host: str) -> bool:
        """Check if host matches any allowed domain pattern."""
        for pattern in ALLOWED_DOMAINS:
            if pattern.startswith("*."):
                # Wildcard pattern: matches exact or any subdomain
                base = pattern[2:]  # Remove "*."
                if host == base or host.endswith("." + base):
                    return True
            elif host == pattern:
                return True
        return False

    def _is_github_pr_merge(self, flow: http.HTTPFlow) -> bool:
        """Check if request is a GitHub PR merge attempt."""
        if not BLOCK_GITHUB_PR_MERGE:
            return False

        host = flow.request.pretty_host
        if host not in ("api.github.com", "github.com") and not host.endswith(".github.com"):
            return False

        # REST API: PUT /repos/{owner}/{repo}/pulls/{number}/merge
        if flow.request.method == "PUT":
            if re.match(r"^/repos/[^/]+/[^/]+/pulls/\d+/merge$", flow.request.path):
                return True

        # GraphQL API: POST /graphql with mergePullRequest mutation
        if flow.request.method == "POST" and flow.request.path == "/graphql":
            content = flow.request.get_text()
            if content and "mergePullRequest" in content:
                return True

        return False

    def request(self, flow: http.HTTPFlow) -> None:
        # Check PR merge block first
        if self._is_github_pr_merge(flow):
            flow.response = http.Response.make(
                403,
                b"Merging pull requests is not allowed in this environment. Do not retry.\n",
                {"Content-Type": "text/plain"}
            )
            return

        host = flow.request.pretty_host
        if not self._is_allowed(host):
            flow.response = http.Response.make(
                403,
                f"Domain '{host}' is not in the allowlist\n".encode(),
                {"Content-Type": "text/plain"}
            )

addons = [AllowlistFilter()]
`

// GenerateFilterScript generates a mitmproxy Python filter script from an allowlist.
// The script blocks all HTTP/HTTPS requests to domains not in the allowlist.
// If blockGitHubPRMerge is true, it also blocks GitHub PR merge API calls.
func GenerateFilterScript(allowlist []string, blockGitHubPRMerge bool) string {
	var domains strings.Builder
	for _, domain := range allowlist {
		domains.WriteString(fmt.Sprintf("    %q,\n", domain))
	}
	blockMerge := "False"
	if blockGitHubPRMerge {
		blockMerge = "True"
	}
	return fmt.Sprintf(filterScriptTemplate, domains.String(), blockMerge)
}

// GenerateIgnoreHostsPattern generates a regex pattern for mitmproxy --ignore-hosts flag.
// Passthrough domains bypass TLS interception (for certificate-pinned services).
func GenerateIgnoreHostsPattern(passthrough []string) string {
	if len(passthrough) == 0 {
		return ""
	}

	var patterns []string
	for _, domain := range passthrough {
		// Escape dots for regex and anchor the pattern
		escaped := regexp.QuoteMeta(domain)
		// Match the domain and any subdomain
		if strings.HasPrefix(domain, "*.") {
			// Wildcard already specified - convert to regex
			base := escaped[4:] // Remove `\*\.`
			patterns = append(patterns, fmt.Sprintf("^(.+\\.)?%s:", base))
		} else {
			patterns = append(patterns, fmt.Sprintf("^(.+\\.)?%s:", escaped))
		}
	}

	return strings.Join(patterns, "|")
}

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

// WriteFilterScript writes a mitmproxy filter script to the project's proxy config directory.
// Returns the path to the written script.
func WriteFilterScript(projectPath string, allowlist []string, blockGitHubPRMerge bool) (string, error) {
	configDir, err := GetProxyConfigDir(projectPath)
	if err != nil {
		return "", err
	}

	scriptContent := GenerateFilterScript(allowlist, blockGitHubPRMerge)
	scriptPath := filepath.Join(configDir, "filter.py")

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write filter script: %w", err)
	}

	return scriptPath, nil
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

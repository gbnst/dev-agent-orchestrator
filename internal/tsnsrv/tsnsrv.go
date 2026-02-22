// pattern: Functional Core (arg building) + Imperative Shell (LookPath)

package tsnsrv

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"devagent/internal/config"
	"devagent/internal/process"
)

// ReadServiceURL reads the service FQDN from tsnsrv's state directory.
// Returns the URL and true if the FQDN was successfully parsed,
// or a fallback URL and false if the state can't be read.
func ReadServiceURL(stateDir string, tc config.TailscaleConfig) (string, bool) {
	scheme := "https"
	if tc.Plaintext {
		scheme = "http"
	}
	fallback := fmt.Sprintf("%s://%s.<tailnet>.ts.net", scheme, tc.Name)

	data, err := os.ReadFile(filepath.Join(stateDir, "tailscaled.state"))
	if err != nil {
		return fallback, false
	}

	var state map[string]json.RawMessage
	if err := json.Unmarshal(data, &state); err != nil {
		return fallback, false
	}

	// _current-profile is base64-encoded profile ID
	currentProfileRaw, ok := state["_current-profile"]
	if !ok {
		return fallback, false
	}
	var currentProfileB64 string
	if err := json.Unmarshal(currentProfileRaw, &currentProfileB64); err != nil {
		return fallback, false
	}
	profileID, err := base64.StdEncoding.DecodeString(currentProfileB64)
	if err != nil {
		return fallback, false
	}

	// _profiles is base64-encoded JSON map of id → profile
	profilesRaw, ok := state["_profiles"]
	if !ok {
		return fallback, false
	}
	var profilesB64 string
	if err := json.Unmarshal(profilesRaw, &profilesB64); err != nil {
		return fallback, false
	}
	profilesJSON, err := base64.StdEncoding.DecodeString(profilesB64)
	if err != nil {
		return fallback, false
	}

	var profiles map[string]struct {
		Name string `json:"Name"`
		Key  string `json:"Key"`
	}
	if err := json.Unmarshal(profilesJSON, &profiles); err != nil {
		return fallback, false
	}

	profileKey := string(profileID)

	// Try direct lookup first (key might match map key)
	if profile, ok := profiles[profileKey]; ok && profile.Name != "" {
		return fmt.Sprintf("%s://%s", scheme, profile.Name), true
	}

	// Fall back to matching by Key field (e.g. _current-profile="profile-7213", map key="7213", Key="profile-7213")
	for _, profile := range profiles {
		if profile.Key == profileKey && profile.Name != "" {
			return fmt.Sprintf("%s://%s", scheme, profile.Name), true
		}
	}

	return fallback, false
}

// BuildProcessConfig builds a process.Config for tsnsrv from the given TailscaleConfig.
// upstreamAddr is the address the web server is listening on (e.g. "127.0.0.1:9001").
// resolvePath expands ~ in paths (use Config.ResolveTokenPath).
func BuildProcessConfig(tc config.TailscaleConfig, upstreamAddr string, resolvePath config.ResolvePathFunc) (process.Config, error) {
	binary, err := exec.LookPath("tsnsrv")
	if err != nil {
		return process.Config{}, fmt.Errorf("tsnsrv binary not found in PATH: %w", err)
	}

	return BuildProcessConfigWith(tc, upstreamAddr, resolvePath, binary)
}

// BuildProcessConfigWith builds a process.Config using the given binary path.
// This is the pure/testable core — no exec.LookPath call.
func BuildProcessConfigWith(tc config.TailscaleConfig, upstreamAddr string, resolvePath config.ResolvePathFunc, binary string) (process.Config, error) {
	args := []string{}

	if tc.Name != "" {
		args = append(args, "-name", tc.Name)
	}

	if tc.Ephemeral {
		args = append(args, "-ephemeral")
	}

	if tc.Funnel {
		args = append(args, "-funnel")
	}

	if tc.FunnelOnly {
		args = append(args, "-funnelOnly")
	}

	if tc.Plaintext {
		args = append(args, "-plaintext")
	}

	authPath := resolvePath(tc.AuthKeyPath)
	if authPath != "" {
		args = append(args, "-authkeyPath", authPath)
	}

	stateDir := resolvePath(tc.StateDir)
	if stateDir != "" {
		args = append(args, "-stateDir", stateDir)
	}

	for _, tag := range tc.Tags {
		args = append(args, "-tag", tag)
	}

	// The upstream URL is the final positional argument
	args = append(args, fmt.Sprintf("http://%s", upstreamAddr))

	return process.Config{
		Name:       "tsnsrv",
		Binary:     binary,
		Args:       args,
		RestartOn:  process.OnFailure,
		MaxRetries: 5,
		RetryDelay: 3 * time.Second,
	}, nil
}

// pattern: Imperative Shell
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"devagent/internal/instance"
)

// Delegate coordinates discovering a running devagent instance and delegating
// a CLI command to it via HTTP. It handles error classification (no instance vs
// other errors) and exit code logic.
type Delegate struct {
	// ConfigDir is the config directory for lock/port file discovery.
	ConfigDir string

	// ExitFunc is called to exit the process. Defaults to os.Exit.
	// Overridable for testing.
	ExitFunc func(int)

	// Stderr is where error messages are written. Defaults to os.Stderr.
	// Overridable for testing.
	Stderr io.Writer

	// ClientTimeout is the HTTP client timeout. Defaults to 10 seconds.
	// Set to longer durations for operations like worktree creation that
	// may involve devcontainer builds.
	ClientTimeout time.Duration
}

// discover initializes defaults, discovers the running instance, and returns an HTTP client.
// On discovery error, prints error message, calls ExitFunc, and returns nil.
// This is the common discovery logic used by both Run and Client methods.
func (d *Delegate) discover() *instance.Client {
	if d.ExitFunc == nil {
		d.ExitFunc = os.Exit
	}
	if d.Stderr == nil {
		d.Stderr = os.Stderr
	}
	if d.ClientTimeout == 0 {
		d.ClientTimeout = 10 * time.Second
	}

	dataDir := ResolveDataDir(d.ConfigDir)

	// Discover the running instance
	baseURL, err := instance.Discover(dataDir)
	if err != nil {
		errMsg := err.Error()
		fmt.Fprintf(d.Stderr, "error: %v\n", err)

		// Check if this is a "no instance" error
		if strings.Contains(errMsg, "no running devagent instance found") {
			d.ExitFunc(2)
		} else {
			d.ExitFunc(1)
		}
		return nil
	}

	// Create a client with the configured timeout
	var client *instance.Client
	if d.ClientTimeout != 10*time.Second {
		client = instance.NewClientWithTimeout(baseURL, d.ClientTimeout)
	} else {
		client = instance.NewClient(baseURL)
	}
	return client
}

// Run executes a delegated command by discovering the running instance and
// invoking fn with an HTTP client targeting it.
//
// Exit codes:
// - 2: no running devagent instance found
// - 1: any other error (connection, client method failed, etc.)
// - 0: success (fn returned nil)
func (d *Delegate) Run(fn func(*instance.Client) error) {
	client := d.discover()
	if client == nil {
		return
	}

	// Invoke the command function
	err := fn(client)
	if err != nil {
		errMsg := err.Error()
		// Extract the message portion if this is a formatted server error
		if strings.Contains(errMsg, "devagent returned status") {
			// Error message is in format: "devagent returned status %d: %s"
			// Try to extract the message part
			parts := strings.SplitN(errMsg, ": ", 2)
			if len(parts) > 1 {
				fmt.Fprintf(d.Stderr, "error: %s\n", parts[1])
			} else {
				fmt.Fprintf(d.Stderr, "error: %s\n", errMsg)
			}
		} else {
			fmt.Fprintf(d.Stderr, "error: %s\n", errMsg)
		}
		d.ExitFunc(1)
		return
	}
}

// Client discovers the running instance and returns an HTTP client for it.
// Handles error classification and calls ExitFunc on failure.
// Returns nil if instance discovery fails (caller should handle nil check).
func (d *Delegate) Client() *instance.Client {
	return d.discover()
}

// PrintJSON pretty-prints JSON data to stdout.
// If stdout is a terminal, uses indentation for readability.
// Otherwise outputs raw bytes.
func PrintJSON(data []byte) error {
	// Check if stdout is a terminal
	fi, _ := os.Stdout.Stat()
	isTerm := (fi.Mode() & os.ModeCharDevice) != 0

	if isTerm {
		// Pretty-print with indentation
		var obj any
		err := json.Unmarshal(data, &obj)
		if err != nil {
			// If JSON parsing fails, just write raw
			_, err := os.Stdout.Write(data)
			return err
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(obj)
	}

	// Write raw bytes
	_, err := os.Stdout.Write(data)
	return err
}

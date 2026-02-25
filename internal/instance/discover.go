// pattern: Imperative Shell
package instance

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

const healthTimeout = 2 * time.Second

// Discover checks whether a running devagent instance exists and returns
// its base URL (e.g. "http://127.0.0.1:12345"). Returns an error if no
// instance is running, the port file is missing, or the health check fails.
func Discover(dataDir string) (string, error) {
	// Try to acquire the lock — if we succeed, no instance is running.
	lockPath := filepath.Join(dataDir, lockFileName)
	fl := flock.New(lockPath)
	locked, err := fl.TryLock()
	if err != nil {
		return "", fmt.Errorf("failed to check lock: %w", err)
	}
	if locked {
		// No instance running — release the lock we just acquired.
		_ = fl.Unlock()
		return "", fmt.Errorf("no running devagent instance found (start devagent first)")
	}

	// Lock is held — read the port file.
	portPath := filepath.Join(dataDir, portFileName)
	data, err := os.ReadFile(portPath)
	if err != nil {
		return "", fmt.Errorf("devagent instance detected but port file missing (try 'devagent cleanup'): %w", err)
	}

	addr := strings.TrimSpace(string(data))
	if addr == "" {
		return "", fmt.Errorf("devagent port file is empty (try 'devagent cleanup')")
	}

	baseURL := fmt.Sprintf("http://%s", addr)

	// Health check to verify the instance is responsive.
	client := &http.Client{Timeout: healthTimeout}
	resp, err := client.Get(baseURL + "/api/health")
	if err != nil {
		return "", fmt.Errorf("devagent instance not responding (try 'devagent cleanup'): %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("devagent health check failed (status %d)", resp.StatusCode)
	}

	return baseURL, nil
}

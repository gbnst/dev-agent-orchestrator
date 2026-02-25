// pattern: Imperative Shell
package instance

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

const (
	lockFileName = "devagent.lock"
	portFileName = "devagent.port"
)

// Lock acquires an exclusive file lock for single-instance enforcement.
// Returns the flock handle (caller must defer Cleanup) or an error if
// another instance already holds the lock.
func Lock(dataDir string) (*flock.Flock, error) {
	lockPath := filepath.Join(dataDir, lockFileName)
	fl := flock.New(lockPath)
	locked, err := fl.TryLock()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("another devagent instance is already running")
	}
	return fl, nil
}

// WritePort writes the web server's listener address to the port file.
func WritePort(dataDir, addr string) error {
	portPath := filepath.Join(dataDir, portFileName)
	return os.WriteFile(portPath, []byte(addr), 0600)
}

// Cleanup removes the port file and releases the file lock.
func Cleanup(dataDir string, fl *flock.Flock) {
	portPath := filepath.Join(dataDir, portFileName)
	_ = os.Remove(portPath)
	if fl != nil {
		_ = fl.Unlock()
	}
}

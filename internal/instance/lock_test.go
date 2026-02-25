package instance

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLockAndCleanup(t *testing.T) {
	dir := t.TempDir()

	// First lock should succeed
	fl, err := Lock(dir)
	if err != nil {
		t.Fatalf("Lock() failed: %v", err)
	}
	if fl == nil {
		t.Fatal("Lock() returned nil flock")
	}

	// Second lock should fail
	_, err = Lock(dir)
	if err == nil {
		t.Fatal("second Lock() should have failed")
	}

	// Write a port file
	if err := WritePort(dir, "127.0.0.1:8080"); err != nil {
		t.Fatalf("WritePort() failed: %v", err)
	}

	// Verify port file exists
	portPath := filepath.Join(dir, portFileName)
	data, err := os.ReadFile(portPath)
	if err != nil {
		t.Fatalf("port file not found: %v", err)
	}
	if string(data) != "127.0.0.1:8080" {
		t.Fatalf("port file content = %q, want %q", string(data), "127.0.0.1:8080")
	}

	// Cleanup should remove port file and release lock
	Cleanup(dir, fl)

	// Port file should be gone
	if _, err := os.Stat(portPath); !os.IsNotExist(err) {
		t.Fatal("port file should have been removed after Cleanup")
	}

	// Lock should be available again
	fl2, err := Lock(dir)
	if err != nil {
		t.Fatalf("Lock() after Cleanup should succeed: %v", err)
	}
	Cleanup(dir, fl2)
}

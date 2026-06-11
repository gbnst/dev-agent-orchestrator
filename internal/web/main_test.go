package web_test

import (
	"fmt"
	"os"
	"testing"
)

// TestMain isolates the devagent data directory for the whole package. Several
// API tests exercise container/worktree creation, which reaches getDataDir()
// via GetProxyCertDir to materialize the proxy cert directory. Pointing
// XDG_DATA_HOME at a temp dir keeps those writes hermetic, so the tests don't
// depend on a writable ambient $HOME — without this they fail in sandboxed
// builds where HOME is unwritable (e.g. nix's /homeless-shelter), which is what
// broke the v0.1.0 release build.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "devagent-web-test")
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to create temp data dir:", err)
		os.Exit(1)
	}
	if err := os.Setenv("XDG_DATA_HOME", dir); err != nil {
		fmt.Fprintln(os.Stderr, "failed to set XDG_DATA_HOME:", err)
		os.Exit(1)
	}

	code := m.Run()

	_ = os.RemoveAll(dir)
	os.Exit(code)
}

package instance

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscover_NoInstance(t *testing.T) {
	dir := t.TempDir()

	_, err := Discover(dir)
	if err == nil {
		t.Fatal("Discover() should fail when no instance is running")
	}
}

func TestDiscover_WithInstance(t *testing.T) {
	dir := t.TempDir()

	// Simulate a running instance: hold the lock + write portfile + serve health
	fl, err := Lock(dir)
	if err != nil {
		t.Fatalf("Lock() failed: %v", err)
	}
	defer Cleanup(dir, fl)

	// Start a real HTTP server with a health endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	// Write the test server's address (strip "http://")
	addr := srv.Listener.Addr().String()
	if err := WritePort(dir, addr); err != nil {
		t.Fatalf("WritePort() failed: %v", err)
	}

	// Discover should succeed
	baseURL, err := Discover(dir)
	if err != nil {
		t.Fatalf("Discover() failed: %v", err)
	}
	if baseURL != "http://"+addr {
		t.Fatalf("Discover() = %q, want %q", baseURL, "http://"+addr)
	}
}

func TestDiscover_StalePortFile(t *testing.T) {
	dir := t.TempDir()

	// Hold the lock but write a portfile pointing to a dead server
	fl, err := Lock(dir)
	if err != nil {
		t.Fatalf("Lock() failed: %v", err)
	}
	defer Cleanup(dir, fl)

	if err := WritePort(dir, "127.0.0.1:1"); err != nil {
		t.Fatalf("WritePort() failed: %v", err)
	}

	_, err = Discover(dir)
	if err == nil {
		t.Fatal("Discover() should fail with stale port file")
	}
}

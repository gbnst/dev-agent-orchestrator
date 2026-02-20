package web_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"devagent/internal/logging"
	"devagent/internal/web"
)

func newTestServer(t *testing.T) *web.Server {
	t.Helper()
	lm := logging.NewTestLogManager(10)
	t.Cleanup(func() { _ = lm.Close() })
	return web.New(
		web.Config{Bind: "127.0.0.1", Port: 0},
		nil, // *container.Manager not needed for health endpoint
		nil, // notifyTUI not needed for health endpoint
		lm,
	)
}

func TestNew_ReturnsNonNil(t *testing.T) {
	s := newTestServer(t)
	if s == nil {
		t.Fatal("New() returned nil")
	}
}

func TestHandleHealth(t *testing.T) {
	s := newTestServer(t)

	ln, err := s.Listen()
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- s.Serve(ln)
	}()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
		<-done
	})

	baseURL := "http://" + s.Addr()

	t.Run("returns 200 with JSON body", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/health")
		if err != nil {
			t.Fatalf("GET /api/health error = %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("reading body error = %v", err)
		}

		want := `{"status":"ok"}`
		if string(body) != want {
			t.Errorf("body = %q, want %q", string(body), want)
		}
	})

	t.Run("content-type is application/json", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/health")
		if err != nil {
			t.Fatalf("GET /api/health error = %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		ct := resp.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("Content-Type = %q, want %q", ct, "application/json")
		}
	})

}

func TestServer_Lifecycle(t *testing.T) {
	lm := logging.NewTestLogManager(10)
	t.Cleanup(func() { _ = lm.Close() })

	s := web.New(
		web.Config{Bind: "127.0.0.1", Port: 0},
		nil,
		nil,
		lm,
	)

	t.Run("Start makes server accept connections", func(t *testing.T) {
		ln, err := s.Listen()
		if err != nil {
			t.Fatalf("Listen() error = %v", err)
		}

		done := make(chan error, 1)
		go func() {
			done <- s.Serve(ln)
		}()

		// Give server a moment to start
		time.Sleep(10 * time.Millisecond)

		addr := s.Addr()
		if addr == "" {
			t.Fatal("Addr() returned empty string after Listen()")
		}

		resp, err := http.Get("http://" + addr + "/api/health")
		if err != nil {
			t.Fatalf("GET health error = %v", err)
		}
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := s.Shutdown(ctx); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}

		select {
		case err := <-done:
			if err != nil && err != http.ErrServerClosed {
				t.Errorf("Serve() returned unexpected error: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("server did not stop after Shutdown()")
		}
	})
}

func TestServer_AddrBeforeListen(t *testing.T) {
	lm := logging.NewTestLogManager(10)
	t.Cleanup(func() { _ = lm.Close() })

	s := web.New(
		web.Config{Bind: "127.0.0.1", Port: 8765},
		nil,
		nil,
		lm,
	)

	// Before Listen(), Addr() returns the configured address
	addr := s.Addr()
	if addr != "127.0.0.1:8765" {
		t.Errorf("Addr() before Listen() = %q, want %q", addr, "127.0.0.1:8765")
	}
}

// TestServer_GracefulShutdown verifies AC5.2: after Shutdown() the server no
// longer accepts new connections.
func TestServer_GracefulShutdown(t *testing.T) {
	lm := logging.NewTestLogManager(10)
	t.Cleanup(func() { _ = lm.Close() })

	s := web.New(
		web.Config{Bind: "127.0.0.1", Port: 0},
		nil,
		nil,
		lm,
	)

	ln, err := s.Listen()
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- s.Serve(ln)
	}()

	addr := s.Addr()

	// Verify server accepts connections before shutdown.
	resp, err := http.Get("http://" + addr + "/api/health")
	if err != nil {
		t.Fatalf("pre-shutdown GET: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pre-shutdown status = %d, want 200", resp.StatusCode)
	}

	// Shutdown gracefully.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	if err := s.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	// Wait for Serve() to return.
	select {
	case err := <-done:
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("Serve() returned unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("server did not stop after Shutdown()")
	}

	// Verify subsequent connections are refused.
	// Use a short client timeout so the test does not block.
	client := &http.Client{Timeout: 2 * time.Second}
	_, err = client.Get("http://" + addr + "/api/health")
	if err == nil {
		t.Error("expected connection refused after Shutdown(), but GET succeeded")
	}
}

// TestServer_BindFailure verifies AC5.3: Start() returns an error when the
// configured port is already in use.
func TestServer_BindFailure(t *testing.T) {
	lm := logging.NewTestLogManager(10)
	t.Cleanup(func() { _ = lm.Close() })

	// Occupy a port so the web server cannot bind to it.
	occupier, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not open occupier listener: %v", err)
	}
	defer func() { _ = occupier.Close() }()

	occupiedAddr := occupier.Addr().String()
	// Extract port from "127.0.0.1:PORT".
	portStr := occupiedAddr[strings.LastIndex(occupiedAddr, ":")+1:]
	port := 0
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("parse port from %q: %v", occupiedAddr, err)
	}

	s := web.New(
		web.Config{Bind: "127.0.0.1", Port: port},
		nil,
		nil,
		lm,
	)

	bindErr := s.Start()
	if bindErr == nil {
		t.Fatal("Start() returned nil error, expected bind error")
	}

	// Verify the error is a bind / address-in-use error.
	errStr := bindErr.Error()
	if !strings.Contains(errStr, "address already in use") &&
		!strings.Contains(errStr, "bind") &&
		!strings.Contains(errStr, "listen") {
		t.Errorf("Start() error = %q; expected address-in-use or bind error", errStr)
	}
}

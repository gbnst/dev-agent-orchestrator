package process

import (
	"context"
	"testing"
	"time"

	"devagent/internal/logging"
)

func testLogger(t *testing.T) *logging.ScopedLogger {
	t.Helper()
	lm := logging.NewTestLogManager(100)
	t.Cleanup(func() { _ = lm.Close() })
	return lm.For("test")
}

func TestSupervisor_StartAndStop(t *testing.T) {
	s := NewSupervisor(Config{
		Name:   "sleeper",
		Binary: "sleep",
		Args:   []string{"60"},
	}, testLogger(t))

	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Give the process a moment to start
	time.Sleep(100 * time.Millisecond)

	if !s.Running() {
		t.Error("expected Running() to be true")
	}

	if err := s.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Done channel should be closed
	select {
	case <-s.Done():
	case <-time.After(time.Second):
		t.Error("Done() not closed after Stop()")
	}

	if s.Running() {
		t.Error("expected Running() to be false after Stop()")
	}
}

func TestSupervisor_ProcessExits(t *testing.T) {
	s := NewSupervisor(Config{
		Name:      "echo",
		Binary:    "true",
		RestartOn: Never,
	}, testLogger(t))

	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	select {
	case <-s.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("process did not exit in time")
	}
}

func TestSupervisor_RestartOnFailure(t *testing.T) {
	s := NewSupervisor(Config{
		Name:       "failer",
		Binary:     "false",
		RestartOn:  OnFailure,
		MaxRetries: 2,
		RetryDelay: 50 * time.Millisecond,
	}, testLogger(t))

	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	select {
	case <-s.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("supervisor did not stop after max retries")
	}
}

func TestSupervisor_NoRestartOnSuccess(t *testing.T) {
	s := NewSupervisor(Config{
		Name:       "succeeder",
		Binary:     "true",
		RestartOn:  OnFailure,
		MaxRetries: 3,
		RetryDelay: 50 * time.Millisecond,
	}, testLogger(t))

	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	select {
	case <-s.Done():
		// Good - should exit without retrying since exit code is 0
	case <-time.After(2 * time.Second):
		t.Fatal("supervisor should have stopped after successful exit")
	}
}

func TestSupervisor_DoubleStartFails(t *testing.T) {
	s := NewSupervisor(Config{
		Name:   "sleeper",
		Binary: "sleep",
		Args:   []string{"60"},
	}, testLogger(t))

	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("First Start() error = %v", err)
	}
	defer s.Stop()

	time.Sleep(50 * time.Millisecond)

	if err := s.Start(ctx); err == nil {
		t.Error("expected error on double Start()")
	}
}

func TestSupervisor_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	s := NewSupervisor(Config{
		Name:   "sleeper",
		Binary: "sleep",
		Args:   []string{"60"},
	}, testLogger(t))

	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-s.Done():
	case <-time.After(3 * time.Second):
		t.Error("Done() not closed after context cancellation")
	}
}

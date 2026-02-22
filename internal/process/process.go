// pattern: Imperative Shell

package process

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"devagent/internal/logging"
)

// RestartPolicy controls when a process is restarted after exit.
type RestartPolicy int

const (
	Never     RestartPolicy = iota // Never restart
	OnFailure                      // Restart only on non-zero exit
	Always                         // Always restart (unless Stop is called)
)

// Config describes a child process to supervise.
type Config struct {
	Name       string
	Binary     string
	Args       []string
	RestartOn  RestartPolicy
	MaxRetries int
	RetryDelay time.Duration
}

// Supervisor manages the lifecycle of a child process.
type Supervisor struct {
	cfg    Config
	logger *logging.ScopedLogger

	mu      sync.Mutex
	cmd     *exec.Cmd
	running bool
	stopped bool
	done    chan struct{}
}

// NewSupervisor creates a new child process supervisor.
func NewSupervisor(cfg Config, logger *logging.ScopedLogger) *Supervisor {
	return &Supervisor{
		cfg:    cfg,
		logger: logger,
		done:   make(chan struct{}),
	}
}

// Start launches the child process in a goroutine. Non-blocking.
func (s *Supervisor) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New("supervisor: already running")
	}
	s.running = true
	s.mu.Unlock()

	go s.run(ctx)
	return nil
}

// Stop sends SIGTERM and waits up to 5 seconds, then SIGKILL.
func (s *Supervisor) Stop() error {
	s.mu.Lock()
	s.stopped = true
	cmd := s.cmd
	s.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		// Not running or already exited
		<-s.done
		return nil
	}

	// Send SIGTERM
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		// Process may already be gone
		<-s.done
		return nil
	}

	// Wait up to 5 seconds for graceful exit
	select {
	case <-s.done:
		return nil
	case <-time.After(5 * time.Second):
	}

	// Force kill
	s.mu.Lock()
	cmd = s.cmd
	s.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}

	<-s.done
	return nil
}

// Running returns whether the child process is currently running.
func (s *Supervisor) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Done returns a channel that is closed when the supervisor exits
// (either the process exited without restart or Stop was called).
func (s *Supervisor) Done() <-chan struct{} {
	return s.done
}

func (s *Supervisor) run(ctx context.Context) {
	defer close(s.done)
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	retries := 0
	for {
		s.mu.Lock()
		if s.stopped {
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()

		exitCode := s.runOnce(ctx)

		s.mu.Lock()
		if s.stopped {
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()

		shouldRestart := false
		switch s.cfg.RestartOn {
		case Always:
			shouldRestart = true
		case OnFailure:
			shouldRestart = exitCode != 0
		case Never:
			shouldRestart = false
		}

		if !shouldRestart {
			return
		}

		retries++
		if s.cfg.MaxRetries > 0 && retries > s.cfg.MaxRetries {
			s.logger.Error("max retries exceeded", "retries", retries-1, "process", s.cfg.Name)
			return
		}

		delay := s.cfg.RetryDelay
		if delay == 0 {
			delay = time.Second
		}

		s.logger.Info("restarting process", "process", s.cfg.Name, "attempt", retries, "delay", delay)

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return
		}
	}
}

func (s *Supervisor) runOnce(ctx context.Context) int {
	cmd := exec.CommandContext(ctx, s.cfg.Binary, s.cfg.Args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.logger.Error("failed to create stdout pipe", "error", err, "process", s.cfg.Name)
		return -1
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.logger.Error("failed to create stderr pipe", "error", err, "process", s.cfg.Name)
		return -1
	}

	s.logger.Info("starting process", "process", s.cfg.Name, "binary", s.cfg.Binary, "args", fmt.Sprintf("%v", s.cfg.Args))

	if err := cmd.Start(); err != nil {
		s.logger.Error("failed to start process", "error", err, "process", s.cfg.Name)
		return -1
	}

	s.mu.Lock()
	s.cmd = cmd
	s.mu.Unlock()

	// Capture stdout and stderr into logger
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			s.logger.Info(scanner.Text(), "stream", "stdout", "process", s.cfg.Name)
		}
	}()
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			s.logger.Info(scanner.Text(), "stream", "stderr", "process", s.cfg.Name)
		}
	}()

	wg.Wait()
	err = cmd.Wait()

	s.mu.Lock()
	s.cmd = nil
	s.mu.Unlock()

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code := exitErr.ExitCode()
			s.logger.Warn("process exited", "process", s.cfg.Name, "exit_code", code)
			return code
		}
		// Context cancellation or other error
		s.logger.Info("process stopped", "process", s.cfg.Name, "error", err)
		return -1
	}

	s.logger.Info("process exited cleanly", "process", s.cfg.Name)
	return 0
}

/*
Copyright 2025 The Scion Authors.
*/

// Package supervisor provides process lifecycle management for sciontool init.
// It handles spawning child processes, signal forwarding, and graceful shutdown.
package supervisor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// ErrNoCommand is returned when no command is specified for the supervisor to run.
var ErrNoCommand = errors.New("no command specified")

// Config holds configuration for the Supervisor.
type Config struct {
	// GracePeriod is the time to wait after SIGTERM before sending SIGKILL.
	GracePeriod time.Duration
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		GracePeriod: 10 * time.Second,
	}
}

// Supervisor manages child process lifecycle including signal forwarding
// and graceful shutdown.
type Supervisor struct {
	config Config
	cmd    *exec.Cmd

	// mu protects the process state
	mu        sync.Mutex
	started   bool
	exited    bool
	exitCode  int
	exitError error

	// done is closed when the child process exits
	done chan struct{}
}

// New creates a new Supervisor with the given configuration.
func New(config Config) *Supervisor {
	return &Supervisor{
		config: config,
		done:   make(chan struct{}),
	}
}

// Run starts and supervises the given command until it exits or the context
// is cancelled. It returns the exit code of the child process.
func (s *Supervisor) Run(ctx context.Context, args []string) (int, error) {
	if len(args) == 0 {
		return 1, ErrNoCommand
	}

	// Create the child process
	s.cmd = exec.Command(args[0], args[1:]...)
	s.cmd.Stdin = os.Stdin
	s.cmd.Stdout = os.Stdout
	s.cmd.Stderr = os.Stderr

	// Start in a new process group so we can signal the whole group
	s.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := s.cmd.Start(); err != nil {
		return 1, fmt.Errorf("failed to start command: %w", err)
	}

	s.mu.Lock()
	s.started = true
	s.mu.Unlock()

	// Wait for the child in a goroutine
	go s.waitForChild()

	// Wait for either context cancellation or child exit
	select {
	case <-ctx.Done():
		// Context cancelled, initiate graceful shutdown
		return s.shutdown()
	case <-s.done:
		// Child exited on its own
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.exitCode, s.exitError
	}
}

// Signal sends a signal to the child process.
func (s *Supervisor) Signal(sig os.Signal) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started || s.exited || s.cmd.Process == nil {
		return nil
	}

	return s.cmd.Process.Signal(sig)
}

// waitForChild waits for the child process to exit and records its exit status.
func (s *Supervisor) waitForChild() {
	err := s.cmd.Wait()

	s.mu.Lock()
	s.exited = true
	s.exitError = err

	if err == nil {
		s.exitCode = 0
	} else {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			s.exitCode = exitErr.ExitCode()
			s.exitError = nil // Exit with non-zero is not an error condition
		} else {
			s.exitCode = 1
		}
	}
	s.mu.Unlock()

	close(s.done)
}

// shutdown performs a graceful shutdown of the child process.
func (s *Supervisor) shutdown() (int, error) {
	s.mu.Lock()
	if s.exited {
		exitCode := s.exitCode
		exitErr := s.exitError
		s.mu.Unlock()
		return exitCode, exitErr
	}
	s.mu.Unlock()

	// Send SIGTERM first
	if err := s.Signal(syscall.SIGTERM); err != nil {
		// If we can't signal, try to get exit status anyway
		s.mu.Lock()
		if s.exited {
			exitCode := s.exitCode
			exitErr := s.exitError
			s.mu.Unlock()
			return exitCode, exitErr
		}
		s.mu.Unlock()
		return 1, fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	// Wait for graceful exit or timeout
	select {
	case <-s.done:
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.exitCode, s.exitError
	case <-time.After(s.config.GracePeriod):
		// Grace period expired, force kill
		if err := s.Signal(syscall.SIGKILL); err != nil {
			s.mu.Lock()
			if s.exited {
				exitCode := s.exitCode
				exitErr := s.exitError
				s.mu.Unlock()
				return exitCode, exitErr
			}
			s.mu.Unlock()
		}
		// Wait for process to actually exit after SIGKILL
		<-s.done
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.exitCode, s.exitError
	}
}

// Done returns a channel that is closed when the child process exits.
func (s *Supervisor) Done() <-chan struct{} {
	return s.done
}

// ExitCode returns the exit code of the child process.
// Only valid after Done() is closed.
func (s *Supervisor) ExitCode() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exitCode
}

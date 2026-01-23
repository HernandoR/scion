/*
Copyright 2025 The Scion Authors.
*/

package supervisor

import (
	"context"
	"syscall"
	"testing"
	"time"
)

func TestSupervisor_RunSuccessfulCommand(t *testing.T) {
	config := DefaultConfig()
	sup := New(config)

	ctx := context.Background()
	exitCode, err := sup.Run(ctx, []string{"true"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestSupervisor_RunFailingCommand(t *testing.T) {
	config := DefaultConfig()
	sup := New(config)

	ctx := context.Background()
	exitCode, err := sup.Run(ctx, []string{"false"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestSupervisor_RunNoCommand(t *testing.T) {
	config := DefaultConfig()
	sup := New(config)

	ctx := context.Background()
	exitCode, err := sup.Run(ctx, []string{})

	if err != ErrNoCommand {
		t.Errorf("expected ErrNoCommand, got %v", err)
	}
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestSupervisor_RunNonExistentCommand(t *testing.T) {
	config := DefaultConfig()
	sup := New(config)

	ctx := context.Background()
	exitCode, err := sup.Run(ctx, []string{"/nonexistent/command/that/does/not/exist"})

	if err == nil {
		t.Error("expected error for non-existent command")
	}
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestSupervisor_RunWithSpecificExitCode(t *testing.T) {
	config := DefaultConfig()
	sup := New(config)

	ctx := context.Background()
	exitCode, err := sup.Run(ctx, []string{"sh", "-c", "exit 42"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if exitCode != 42 {
		t.Errorf("expected exit code 42, got %d", exitCode)
	}
}

func TestSupervisor_ContextCancellation(t *testing.T) {
	config := Config{
		GracePeriod: 100 * time.Millisecond,
	}
	sup := New(config)

	ctx, cancel := context.WithCancel(context.Background())

	// Start a long-running command
	done := make(chan struct{})
	var runErr error
	go func() {
		_, runErr = sup.Run(ctx, []string{"sleep", "60"})
		close(done)
	}()

	// Give the process time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel the context
	cancel()

	// Wait for supervisor to complete
	select {
	case <-done:
		// Expected
	case <-time.After(5 * time.Second):
		t.Fatal("supervisor did not complete after context cancellation")
	}

	if runErr != nil {
		t.Errorf("unexpected error: %v", runErr)
	}
	// Exit code depends on how the process was terminated
	// We just verify it completed
}

func TestSupervisor_Signal(t *testing.T) {
	config := Config{
		GracePeriod: 100 * time.Millisecond,
	}
	sup := New(config)

	ctx := context.Background()

	// Start a long-running command
	done := make(chan struct{})
	go func() {
		sup.Run(ctx, []string{"sleep", "60"})
		close(done)
	}()

	// Give the process time to start
	time.Sleep(50 * time.Millisecond)

	// Send SIGTERM
	if err := sup.Signal(syscall.SIGTERM); err != nil {
		t.Errorf("failed to send signal: %v", err)
	}

	// Wait for process to exit
	select {
	case <-done:
		// Expected
	case <-time.After(5 * time.Second):
		t.Fatal("process did not exit after SIGTERM")
	}
}

func TestSupervisor_Done(t *testing.T) {
	config := DefaultConfig()
	sup := New(config)

	ctx := context.Background()

	go sup.Run(ctx, []string{"true"})

	select {
	case <-sup.Done():
		// Expected
	case <-time.After(5 * time.Second):
		t.Fatal("Done channel not closed after process exit")
	}
}

func TestSupervisor_ExitCode(t *testing.T) {
	config := DefaultConfig()
	sup := New(config)

	ctx := context.Background()
	sup.Run(ctx, []string{"sh", "-c", "exit 7"})

	if code := sup.ExitCode(); code != 7 {
		t.Errorf("expected exit code 7, got %d", code)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.GracePeriod != 10*time.Second {
		t.Errorf("expected default grace period 10s, got %s", config.GracePeriod)
	}
}

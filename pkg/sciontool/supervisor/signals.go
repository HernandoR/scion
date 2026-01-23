/*
Copyright 2025 The Scion Authors.
*/

package supervisor

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// SignalHandler handles OS signals and forwards them to the supervisor.
type SignalHandler struct {
	supervisor *Supervisor
	sigChan    chan os.Signal
	cancel     context.CancelFunc
}

// NewSignalHandler creates a signal handler that forwards signals to the supervisor.
func NewSignalHandler(supervisor *Supervisor, cancel context.CancelFunc) *SignalHandler {
	return &SignalHandler{
		supervisor: supervisor,
		sigChan:    make(chan os.Signal, 1),
		cancel:     cancel,
	}
}

// Start begins listening for signals. It handles SIGTERM and SIGINT by
// cancelling the context (which triggers graceful shutdown), and forwards
// SIGHUP to the child process.
func (h *SignalHandler) Start() {
	signal.Notify(h.sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	go func() {
		for sig := range h.sigChan {
			switch sig {
			case syscall.SIGTERM, syscall.SIGINT:
				// Trigger graceful shutdown by cancelling context
				h.cancel()
				return
			case syscall.SIGHUP:
				// Forward SIGHUP to child (can be used for reload in future)
				h.supervisor.Signal(sig)
			}
		}
	}()
}

// Stop stops the signal handler.
func (h *SignalHandler) Stop() {
	signal.Stop(h.sigChan)
	close(h.sigChan)
}

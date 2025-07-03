// Package cli provides styled terminal output using lipgloss.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// InterruptHandler manages graceful shutdown with friendly messages.
type InterruptHandler struct {
	writer       io.Writer
	cancelFunc   context.CancelFunc
	interrupted  bool
	showProgress bool
	mu           sync.Mutex
}

// NewInterruptHandler creates a new interrupt handler.
func NewInterruptHandler(writer io.Writer) *InterruptHandler {
	if writer == nil {
		writer = os.Stdout
	}
	return &InterruptHandler{
		writer: writer,
	}
}

// HandleInterrupts sets up signal handling and returns a context that will be canceled on interrupt.
func (h *InterruptHandler) HandleInterrupts(ctx context.Context, showProgress bool) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	h.cancelFunc = cancel
	h.showProgress = showProgress

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		h.mu.Lock()
		if !h.interrupted {
			h.interrupted = true
			h.showInterruptMessage()
		}
		h.mu.Unlock()
		cancel()

		// Create a timer to give the program a moment to clean up gracefully
		timer := time.NewTimer(100 * time.Millisecond)
		<-timer.C

		// Force exit if still running
		os.Exit(0)
	}()

	return ctx
}

// showInterruptMessage displays a friendly interrupt message.
func (h *InterruptHandler) showInterruptMessage() {
	msg := "\n\n" + FormatWarning("Classification interrupted!")
	msg += "\n" + FormatInfo("See you later! 🌶️") + "\n"

	if _, err := fmt.Fprint(h.writer, msg); err != nil {
		// Best effort - we're shutting down anyway
		fmt.Fprintf(os.Stderr, "Failed to write interrupt message: %v\n", err)
	}
}

// WasInterrupted returns true if the process was interrupted.
func (h *InterruptHandler) WasInterrupted() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.interrupted
}

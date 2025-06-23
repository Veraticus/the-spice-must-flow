// Package cli provides styled terminal output using lipgloss.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
)

// InterruptHandler manages graceful shutdown with friendly messages.
type InterruptHandler struct {
	writer       io.Writer
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

// HandleInterrupts monitors the context for cancellation and shows a message when interrupted.
func (h *InterruptHandler) HandleInterrupts(ctx context.Context, showProgress bool) context.Context {
	h.showProgress = showProgress

	// Monitor the context for cancellation
	go func() {
		<-ctx.Done()
		h.mu.Lock()
		if !h.interrupted {
			h.interrupted = true
			h.showInterruptMessage()
		}
		h.mu.Unlock()
	}()

	return ctx
}

// showInterruptMessage displays a friendly interrupt message.
func (h *InterruptHandler) showInterruptMessage() {
	msg := "\n\n" + FormatWarning("Classification interrupted!")

	if h.showProgress {
		msg += "\n" + FormatInfo("Progress has been saved. Resume with: spice classify --resume")
	}

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

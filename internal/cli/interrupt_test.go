package cli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// syncBuffer provides thread-safe access to a bytes.Buffer.
type syncBuffer struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

func (s *syncBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func TestNewInterruptHandler(t *testing.T) {
	tests := []struct {
		writer io.Writer
		name   string
	}{
		{
			name:   "with custom writer",
			writer: &bytes.Buffer{},
		},
		{
			name:   "with nil writer",
			writer: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewInterruptHandler(tt.writer)
			assert.NotNil(t, handler)
			assert.NotNil(t, handler.writer)
			assert.False(t, handler.interrupted)
		})
	}
}

func TestHandleInterrupts(t *testing.T) {
	output := &syncBuffer{}
	handler := NewInterruptHandler(output)

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	ctx = handler.HandleInterrupts(ctx, true) //nolint:ineffassign // We need the returned context

	// Context should not be canceled initially
	select {
	case <-ctx.Done():
		t.Fatal("Context should not be canceled initially")
	default:
	}

	// Cancel the context to simulate interruption
	cancel()

	// Give the handler time to detect cancellation and write the message
	time.Sleep(50 * time.Millisecond)

	assert.True(t, handler.WasInterrupted())
	outputStr := output.String()
	assert.Contains(t, outputStr, "Classification interrupted!")
	assert.Contains(t, outputStr, "Progress has been saved")
	assert.Contains(t, outputStr, "Resume with: spice classify --resume")
}

func TestHandleInterrupts_NoProgress(t *testing.T) {
	output := &syncBuffer{}
	handler := NewInterruptHandler(output)

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	_ = handler.HandleInterrupts(ctx, false)

	// Cancel the context to simulate interruption
	cancel()

	// Give the handler time to detect cancellation and write the message
	time.Sleep(50 * time.Millisecond)

	assert.True(t, handler.WasInterrupted())
	outputStr := output.String()
	assert.Contains(t, outputStr, "Classification interrupted!")
	assert.NotContains(t, outputStr, "Progress has been saved")
}

func TestMultipleInterrupts(t *testing.T) {
	output := &syncBuffer{}
	handler := NewInterruptHandler(output)

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	_ = handler.HandleInterrupts(ctx, true)

	// Cancel the context
	cancel()

	// Give time for the handler to process
	time.Sleep(50 * time.Millisecond)

	// Message should only be shown once
	outputStr := output.String()
	count := strings.Count(outputStr, "Classification interrupted!")
	assert.Equal(t, 1, count, "Interrupt message should only be shown once")
}

func TestShowInterruptMessage(t *testing.T) {
	tests := []struct {
		name         string
		expected     []string
		notExpected  []string
		showProgress bool
	}{
		{
			name:         "with progress",
			showProgress: true,
			expected: []string{
				"Classification interrupted!",
				"Progress has been saved",
				"Resume with: spice classify --resume",
				"See you later!",
			},
			notExpected: []string{},
		},
		{
			name:         "without progress",
			showProgress: false,
			expected: []string{
				"Classification interrupted!",
				"See you later!",
			},
			notExpected: []string{
				"Progress has been saved",
				"Resume with",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			handler := &InterruptHandler{
				writer:       &output,
				showProgress: tt.showProgress,
			}

			handler.showInterruptMessage()

			outputStr := output.String()
			for _, expected := range tt.expected {
				assert.Contains(t, outputStr, expected)
			}
			for _, notExpected := range tt.notExpected {
				assert.NotContains(t, outputStr, notExpected)
			}
		})
	}
}

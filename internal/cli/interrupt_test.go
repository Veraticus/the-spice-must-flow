package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	var output bytes.Buffer
	handler := NewInterruptHandler(&output)

	ctx := context.Background()
	ctx = handler.HandleInterrupts(ctx, true)

	// Context should not be canceled initially
	select {
	case <-ctx.Done():
		t.Fatal("Context should not be canceled initially")
	default:
	}

	// Simulate interrupt
	process, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	err = process.Signal(os.Interrupt)
	require.NoError(t, err)

	// Wait for context to be canceled
	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Context should be canceled after interrupt")
	}

	// Give the handler time to write the message using a timer
	timer := time.NewTimer(10 * time.Millisecond)
	<-timer.C

	assert.True(t, handler.WasInterrupted())
	outputStr := output.String()
	assert.Contains(t, outputStr, "Classification interrupted!")
	assert.Contains(t, outputStr, "See you later! 🌶️")
}

func TestHandleInterrupts_NoProgress(t *testing.T) {
	var output bytes.Buffer
	handler := NewInterruptHandler(&output)

	ctx := context.Background()
	ctx = handler.HandleInterrupts(ctx, false)

	// Simulate interrupt
	process, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	err = process.Signal(syscall.SIGTERM)
	require.NoError(t, err)

	// Wait for context to be canceled
	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Context should be canceled after SIGTERM")
	}

	// Give the handler time to write the message using a timer
	timer := time.NewTimer(10 * time.Millisecond)
	<-timer.C

	assert.True(t, handler.WasInterrupted())
	outputStr := output.String()
	assert.Contains(t, outputStr, "Classification interrupted!")
	// Progress saving was removed
}

func TestMultipleInterrupts(t *testing.T) {
	var output bytes.Buffer
	handler := NewInterruptHandler(&output)

	ctx := context.Background()
	ctx = handler.HandleInterrupts(ctx, true)

	// Send multiple interrupts
	process, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		err = process.Signal(os.Interrupt)
		require.NoError(t, err)
		// Use timer instead of sleep
		timer := time.NewTimer(5 * time.Millisecond)
		<-timer.C
	}

	// Wait for context to be canceled
	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Context should be canceled")
	}

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
				// No specific excludes needed
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

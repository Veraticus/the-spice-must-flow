package cli

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
)

// ErrInputCancelled is returned when input is canceled by context.
var ErrInputCancelled = errors.New("input canceled")

// NonBlockingReader provides context-aware input reading that can be interrupted.
type NonBlockingReader struct {
	reader      *bufio.Reader
	readingLock sync.Mutex
}

// NewNonBlockingReader creates a new non-blocking reader.
func NewNonBlockingReader(reader io.Reader) *NonBlockingReader {
	if reader == nil {
		panic("reader cannot be nil")
	}

	return &NonBlockingReader{
		reader: bufio.NewReader(reader),
	}
}

// Start is a no-op for compatibility but no longer needed.
func (r *NonBlockingReader) Start(_ context.Context) {
	// No-op - we'll handle context per read operation
}

// ReadString reads a string until delimiter, respecting context cancellation.
func (r *NonBlockingReader) ReadString(ctx context.Context, delim byte) (string, error) {
	// Channel to receive the result
	type result struct {
		err   error
		value string
	}
	resultCh := make(chan result, 1)

	// Start reading in a goroutine
	go func() {
		r.readingLock.Lock()
		defer r.readingLock.Unlock()

		value, err := r.reader.ReadString(delim)
		resultCh <- result{value: value, err: err}
	}()

	// Wait for either the read to complete or context cancellation
	select {
	case <-ctx.Done():
		// Context canceled
		// Note: The reading goroutine will continue until it completes,
		// but we return immediately to the caller
		return "", ErrInputCancelled
	case res := <-resultCh:
		return res.value, res.err
	}
}

// ReadLine reads a line, respecting context cancellation.
func (r *NonBlockingReader) ReadLine(ctx context.Context) (string, error) {
	line, err := r.ReadString(ctx, '\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// Close is a no-op for compatibility.
func (r *NonBlockingReader) Close() {
	// No-op - nothing to clean up
}

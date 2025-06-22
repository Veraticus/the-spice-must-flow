package common

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

var (
	// ErrRateLimit indicates that the API rate limit has been exceeded.
	ErrRateLimit = errors.New("rate limit exceeded")
	// ErrMaxRetries indicates that all retry attempts have been exhausted.
	ErrMaxRetries = errors.New("max retries exceeded")
)

// RetryableError wraps an error with retry-specific metadata.
type RetryableError struct {
	Err       error
	Retryable bool
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

// WithRetry executes an operation with configurable retry behavior.
func WithRetry(ctx context.Context, operation func() error, opts service.RetryOptions) error {
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = 3
	}
	if opts.InitialDelay <= 0 {
		opts.InitialDelay = 100 * time.Millisecond
	}
	if opts.MaxDelay <= 0 {
		opts.MaxDelay = 30 * time.Second
	}
	if opts.Multiplier <= 0 {
		opts.Multiplier = 2.0
	}

	delay := opts.InitialDelay

	for attempt := 1; attempt <= opts.MaxAttempts; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		// Check if error is retryable
		var retryableErr *RetryableError
		if errors.As(err, &retryableErr) && !retryableErr.Retryable {
			return err
		}

		// Special handling for rate limits
		if errors.Is(err, ErrRateLimit) {
			delay = opts.MaxDelay
		}

		if attempt == opts.MaxAttempts {
			return fmt.Errorf("%w after %d attempts: %v", ErrMaxRetries, opts.MaxAttempts, err)
		}

		slog.Warn("Operation failed, retrying",
			"attempt", attempt,
			"max_attempts", opts.MaxAttempts,
			"delay", delay,
			"error", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Exponential backoff with jitter
			delay = time.Duration(float64(delay) * opts.Multiplier)
			if delay > opts.MaxDelay {
				delay = opts.MaxDelay
			}
		}
	}

	return ErrMaxRetries
}

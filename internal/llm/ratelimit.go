package llm

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// rateLimiter implements a simple token bucket rate limiter.
type rateLimiter struct {
	lastRefill time.Time
	stopCh     chan struct{}
	tokens     int
	capacity   int
	refillRate int
	mu         sync.Mutex
}

// newRateLimiter creates a new rate limiter with the specified requests per minute.
func newRateLimiter(requestsPerMinute int) *rateLimiter {
	if requestsPerMinute <= 0 {
		requestsPerMinute = 60 // Default to 60 requests per minute
	}

	rl := &rateLimiter{
		tokens:     requestsPerMinute,
		capacity:   requestsPerMinute,
		refillRate: requestsPerMinute,
		lastRefill: time.Now(),
		stopCh:     make(chan struct{}),
	}

	// Start refill goroutine
	go rl.refill()

	return rl
}

// wait blocks until a token is available or the context is canceled.
func (rl *rateLimiter) wait(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		if rl.tryAcquire() {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("rate limiter canceled: %w", ctx.Err())
		case <-ticker.C:
			// Try again
		}
	}
}

// tryAcquire attempts to acquire a token without blocking.
func (rl *rateLimiter) tryAcquire() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.tokens > 0 {
		rl.tokens--
		return true
	}
	return false
}

// refill periodically adds tokens to the bucket.
func (rl *rateLimiter) refill() {
	ticker := time.NewTicker(time.Minute / time.Duration(rl.refillRate))
	defer ticker.Stop()

	for {
		select {
		case <-rl.stopCh:
			return
		case <-ticker.C:
			rl.mu.Lock()
			if rl.tokens < rl.capacity {
				rl.tokens++
			}
			rl.mu.Unlock()
		}
	}
}

// reset resets the rate limiter to full capacity.
func (rl *rateLimiter) reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.tokens = rl.capacity
	rl.lastRefill = time.Now()
}

// Close stops the refill goroutine.
func (rl *rateLimiter) Close() {
	close(rl.stopCh)
}

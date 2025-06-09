package llm

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter(t *testing.T) {
	t.Run("basic rate limiting", func(t *testing.T) {
		// Create a rate limiter with 10 requests per minute
		rl := newRateLimiter(10)
		ctx := context.Background()

		// Should be able to make 10 requests immediately
		for i := 0; i < 10; i++ {
			err := rl.wait(ctx)
			require.NoError(t, err)
		}

		// 11th request should need to wait
		start := time.Now()
		done := make(chan bool)
		go func() {
			err := rl.wait(ctx)
			assert.NoError(t, err)
			done <- true
		}()

		select {
		case <-done:
			// Should have waited for refill
			elapsed := time.Since(start)
			// Allow some tolerance for timing
			assert.True(t, elapsed >= 50*time.Millisecond, "Expected to wait for refill, but completed too quickly")
		case <-time.After(10 * time.Second):
			t.Fatal("Rate limiter wait timed out")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		rl := newRateLimiter(1) // Only 1 request per minute

		// Use up the token
		err := rl.wait(context.Background())
		require.NoError(t, err)

		// Create a cancelable context
		ctx, cancel := context.WithCancel(context.Background())

		// Start waiting in a goroutine
		done := make(chan error)
		go func() {
			done <- rl.wait(ctx)
		}()

		// Cancel the context
		time.Sleep(10 * time.Millisecond)
		cancel()

		// Should get context error
		err = <-done
		require.Error(t, err)
		assert.Contains(t, err.Error(), "rate limiter canceled")
	})

	t.Run("tryAcquire", func(t *testing.T) {
		rl := newRateLimiter(5)

		// Should succeed for first 5 attempts
		for i := 0; i < 5; i++ {
			success := rl.tryAcquire()
			assert.True(t, success, "Expected tryAcquire to succeed for attempt %d", i+1)
		}

		// 6th attempt should fail
		success := rl.tryAcquire()
		assert.False(t, success, "Expected tryAcquire to fail after tokens exhausted")
	})

	t.Run("reset", func(t *testing.T) {
		rl := newRateLimiter(3)

		// Use up all tokens
		for i := 0; i < 3; i++ {
			success := rl.tryAcquire()
			require.True(t, success)
		}

		// Should be out of tokens
		success := rl.tryAcquire()
		assert.False(t, success)

		// Reset the limiter
		rl.reset()

		// Should have tokens again
		success = rl.tryAcquire()
		assert.True(t, success)
	})

	t.Run("default rate limit", func(t *testing.T) {
		// Test with zero rate limit (should default to 60)
		rl := newRateLimiter(0)

		// Should be able to make many requests
		for i := 0; i < 50; i++ {
			success := rl.tryAcquire()
			require.True(t, success, "Expected default rate limit to allow many requests")
		}
	})

	t.Run("concurrent access", func(t *testing.T) {
		rl := newRateLimiter(100)
		ctx := context.Background()

		// Run multiple goroutines trying to acquire tokens
		var acquired int32
		done := make(chan bool, 10)
		mu := sync.Mutex{}

		for i := 0; i < 10; i++ {
			go func() {
				for j := 0; j < 10; j++ {
					if err := rl.wait(ctx); err == nil {
						mu.Lock()
						acquired++
						mu.Unlock()
					}
				}
				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}

		// Should have acquired exactly 100 tokens
		assert.Equal(t, int32(100), acquired)
	})
}

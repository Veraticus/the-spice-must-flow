package llm

import (
	"testing"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSuggestionCache(t *testing.T) {
	t.Run("basic operations", func(t *testing.T) {
		cache := newSuggestionCache(5 * time.Minute)
		defer cache.clear()

		// Test empty cache
		_, found := cache.get("non-existent")
		assert.False(t, found)

		// Test set and get
		suggestion := service.LLMSuggestion{
			TransactionID: "test-123",
			Category:      "Coffee & Dining",
			Confidence:    0.95,
		}
		cache.set("key1", suggestion)

		retrieved, found := cache.get("key1")
		assert.True(t, found)
		assert.Equal(t, suggestion, retrieved)

		// Test size
		assert.Equal(t, 1, cache.size())

		// Test clear
		cache.clear()
		assert.Equal(t, 0, cache.size())
		_, found = cache.get("key1")
		assert.False(t, found)
	})

	t.Run("expiration", func(t *testing.T) {
		// Use a very short TTL for testing
		cache := newSuggestionCache(50 * time.Millisecond)
		defer cache.clear()

		suggestion := service.LLMSuggestion{
			TransactionID: "test-456",
			Category:      "Shopping",
			Confidence:    0.85,
		}
		cache.set("key2", suggestion)

		// Should be found immediately
		_, found := cache.get("key2")
		assert.True(t, found)

		// Wait for expiration
		time.Sleep(100 * time.Millisecond)

		// Should not be found after expiration
		_, found = cache.get("key2")
		assert.False(t, found)
	})

	t.Run("concurrent access", func(t *testing.T) {
		cache := newSuggestionCache(5 * time.Minute)
		defer cache.clear()

		// Run concurrent operations
		done := make(chan bool)
		go func() {
			for i := 0; i < 100; i++ {
				cache.set("concurrent", service.LLMSuggestion{
					TransactionID: "test",
					Category:      "Test",
					Confidence:    0.8,
				})
			}
			done <- true
		}()

		go func() {
			for i := 0; i < 100; i++ {
				_, _ = cache.get("concurrent")
			}
			done <- true
		}()

		go func() {
			for i := 0; i < 10; i++ {
				_ = cache.size()
				time.Sleep(time.Millisecond)
			}
			done <- true
		}()

		// Wait for all goroutines
		for i := 0; i < 3; i++ {
			<-done
		}

		// Cache should still be functional
		cache.set("after-concurrent", service.LLMSuggestion{
			TransactionID: "final",
			Category:      "Final",
			Confidence:    0.9,
		})
		_, found := cache.get("after-concurrent")
		assert.True(t, found)
	})

	t.Run("multiple entries", func(t *testing.T) {
		cache := newSuggestionCache(5 * time.Minute)
		defer cache.clear()

		suggestions := []service.LLMSuggestion{
			{TransactionID: "1", Category: "Coffee & Dining", Confidence: 0.95},
			{TransactionID: "2", Category: "Shopping", Confidence: 0.85},
			{TransactionID: "3", Category: "Groceries", Confidence: 0.90},
		}

		// Add multiple entries
		for i, s := range suggestions {
			cache.set(string(rune(i)), s)
		}

		assert.Equal(t, 3, cache.size())

		// Verify all entries
		for i, expected := range suggestions {
			retrieved, found := cache.get(string(rune(i)))
			require.True(t, found)
			assert.Equal(t, expected, retrieved)
		}
	})
}

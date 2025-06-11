package llm

import (
	"sync"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/service"
)

// cacheEntry represents a cached classification suggestion.
type cacheEntry struct {
	expiry     time.Time
	suggestion service.LLMSuggestion
}

// suggestionCache provides thread-safe caching for LLM suggestions.
type suggestionCache struct {
	entries map[string]cacheEntry
	stopCh  chan struct{}
	ttl     time.Duration
	mu      sync.RWMutex
}

// newSuggestionCache creates a new cache with the specified TTL.
func newSuggestionCache(ttl time.Duration) *suggestionCache {
	if ttl == 0 {
		ttl = 15 * time.Minute // Default TTL
	}

	cache := &suggestionCache{
		entries: make(map[string]cacheEntry),
		ttl:     ttl,
		stopCh:  make(chan struct{}),
	}

	// Start cleanup goroutine
	go cache.cleanup()

	return cache
}

// get retrieves a suggestion from the cache if it exists and hasn't expired.
func (c *suggestionCache) get(key string) (service.LLMSuggestion, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return service.LLMSuggestion{}, false
	}

	if time.Now().After(entry.expiry) {
		return service.LLMSuggestion{}, false
	}

	return entry.suggestion, true
}

// set stores a suggestion in the cache.
func (c *suggestionCache) set(key string, suggestion service.LLMSuggestion) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = cacheEntry{
		suggestion: suggestion,
		expiry:     time.Now().Add(c.ttl),
	}
}

// cleanup periodically removes expired entries.
func (c *suggestionCache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for key, entry := range c.entries {
				if now.After(entry.expiry) {
					delete(c.entries, key)
				}
			}
			c.mu.Unlock()
		}
	}
}

// clear removes all entries from the cache.
func (c *suggestionCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]cacheEntry)
}

// size returns the number of entries in the cache.
func (c *suggestionCache) size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Close stops the cleanup goroutine.
func (c *suggestionCache) Close() {
	close(c.stopCh)
}

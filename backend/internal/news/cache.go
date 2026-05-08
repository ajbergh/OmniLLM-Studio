package news

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// cacheEntry holds a cached response with its expiration time.
type cacheEntry struct {
	data      *StoryPage
	expiresAt time.Time
}

// Cache is a simple in-memory TTL cache for news API responses.
type Cache struct {
	mu        sync.RWMutex
	entries   map[string]*cacheEntry
	ttl       time.Duration
	hitCount  int
	missCount int
}

// NewCache creates a new cache with the given TTL.
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
	}
}

// cacheKey generates a deterministic cache key from query parameters.
func cacheKey(q StoryQuery) string {
	h := sha256.New()
	fmt.Fprintf(h, "page=%d&pageSize=%d&issueSlug=%s&search=%s&emotionTags=%v",
		q.Page, q.PageSize, q.IssueSlug, q.Search, q.EmotionTags)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Get retrieves a cached response if available and not expired.
func (c *Cache) Get(q StoryQuery) (*StoryPage, bool) {
	key := cacheKey(q)
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		c.mu.Lock()
		c.missCount++
		c.mu.Unlock()
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.entries, key)
		c.missCount++
		c.mu.Unlock()
		return nil, false
	}

	c.mu.Lock()
	c.hitCount++
	c.mu.Unlock()
	return entry.data, true
}

// Set stores a response in the cache.
func (c *Cache) Set(q StoryQuery, data *StoryPage) {
	key := cacheKey(q)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = &cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Stats returns cache hit/miss counts.
func (c *Cache) Stats() (hits, misses int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hitCount, c.missCount
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*cacheEntry)
	c.hitCount = 0
	c.missCount = 0
}

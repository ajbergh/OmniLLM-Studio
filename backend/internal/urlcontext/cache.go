package urlcontext

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// cacheEntry stores a resolved source and its expiry.
type cacheEntry struct {
	source    *ResolvedSource
	storedAt  time.Time
	expiresAt time.Time
}

// Cache is a concurrency-safe in-memory TTL cache for resolved URL sources.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	ttl     time.Duration
}

// NewCache creates a new Cache with the given TTL.
func NewCache(ttl time.Duration) *Cache {
	c := &Cache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
	}
	go c.gcLoop()
	return c
}

// Get returns a cached result or nil if not found / expired.
func (c *Cache) Get(key string) *ResolvedSource {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	if !ok {
		return nil
	}
	if time.Now().After(entry.expiresAt) {
		return nil
	}
	return entry.source
}

// Set stores a result with the configured TTL.
func (c *Cache) Set(key string, source *ResolvedSource) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = &cacheEntry{
		source:    source,
		storedAt:  now,
		expiresAt: now.Add(c.ttl),
	}
}

// Delete removes a cache entry.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// RepoKey builds a cache key for a GitHub repository inspection.
func RepoKey(owner, repo, ref string, goal AnalysisGoal) string {
	return fmt.Sprintf("github_repo:%s/%s:%s:%s", owner, repo, ref, goal)
}

// URLKey builds a cache key for a generic URL fetch.
func URLKey(rawURL string, goal AnalysisGoal) string {
	h := sha256.Sum256([]byte(rawURL))
	return fmt.Sprintf("url_context:%x:%s", h[:8], goal)
}

// gcLoop periodically evicts expired entries.
func (c *Cache) gcLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		c.evictExpired()
	}
}

func (c *Cache) evictExpired() {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, e := range c.entries {
		if now.After(e.expiresAt) {
			delete(c.entries, k)
		}
	}
}

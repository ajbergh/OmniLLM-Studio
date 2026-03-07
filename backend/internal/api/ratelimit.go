package api

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// rateLimiter provides a simple in-memory sliding-window rate limiter keyed by
// arbitrary string (typically client IP). It is safe for concurrent use.
type rateLimiter struct {
	mu          sync.Mutex
	attempts    map[string][]time.Time
	window      time.Duration
	maxAttempts int
}

// newRateLimiter returns a rate limiter that allows at most max requests per
// window per key. A background goroutine prunes stale entries periodically.
func newRateLimiter(window time.Duration, max int) *rateLimiter {
	rl := &rateLimiter{
		attempts:    make(map[string][]time.Time),
		window:      window,
		maxAttempts: max,
	}
	go func() {
		for range time.Tick(window) {
			rl.cleanup()
		}
	}()
	return rl
}

// Allow returns true if the key has not exceeded maxAttempts within the current
// window, recording the attempt. Returns false (rate-limited) otherwise.
func (rl *rateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Keep only timestamps within the window.
	valid := make([]time.Time, 0, len(rl.attempts[key]))
	for _, t := range rl.attempts[key] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.maxAttempts {
		rl.attempts[key] = valid
		return false
	}

	rl.attempts[key] = append(valid, now)
	return true
}

// cleanup removes entries whose most recent attempt is older than the window.
func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.window)
	for key, times := range rl.attempts {
		valid := make([]time.Time, 0, len(times))
		for _, t := range times {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(rl.attempts, key)
		} else {
			rl.attempts[key] = valid
		}
	}
}

// RateLimit returns chi-compatible middleware that limits requests per IP using
// the provided rateLimiter instance.
func RateLimit(rl *rateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			// Strip port from RemoteAddr (e.g. "127.0.0.1:54321" -> "127.0.0.1")
			if host, _, err := net.SplitHostPort(ip); err == nil {
				ip = host
			}
			if !rl.Allow(ip) {
				w.Header().Set("Retry-After", "60")
				respondError(w, http.StatusTooManyRequests, "too many requests, try again later")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

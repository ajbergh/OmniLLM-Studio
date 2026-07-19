package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// rateLimiter provides an in-memory sliding-window limiter keyed by arbitrary
// strings. Authentication middleware uses both network and account keys so a
// spoofed forwarded-address header cannot remove account-level throttling.
type rateLimiter struct {
	mu          sync.Mutex
	attempts    map[string][]time.Time
	window      time.Duration
	maxAttempts int
}

func newRateLimiter(window time.Duration, max int) *rateLimiter {
	limiter := &rateLimiter{
		attempts: make(map[string][]time.Time), window: window, maxAttempts: max,
	}
	go func() {
		ticker := time.NewTicker(window)
		defer ticker.Stop()
		for range ticker.C {
			limiter.cleanup()
		}
	}()
	return limiter
}

func (rl *rateLimiter) Allow(key string) bool {
	if strings.TrimSpace(key) == "" {
		key = "unknown"
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-rl.window)
	valid := rl.attempts[key][:0]
	for _, attempt := range rl.attempts[key] {
		if attempt.After(cutoff) {
			valid = append(valid, attempt)
		}
	}
	if len(valid) >= rl.maxAttempts {
		rl.attempts[key] = valid
		return false
	}
	rl.attempts[key] = append(valid, now)
	return true
}

func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-rl.window)
	for key, attempts := range rl.attempts {
		valid := attempts[:0]
		for _, attempt := range attempts {
			if attempt.After(cutoff) {
				valid = append(valid, attempt)
			}
		}
		if len(valid) == 0 {
			delete(rl.attempts, key)
		} else {
			rl.attempts[key] = valid
		}
	}
}

// RateLimit limits authentication requests by both source and normalized
// username. The body is restored before the handler receives it.
func RateLimit(rl *rateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if host, _, err := net.SplitHostPort(ip); err == nil {
				ip = host
			}
			keys := []string{"ip:" + strings.ToLower(strings.TrimSpace(ip))}
			if account := authAccountKey(r); account != "" {
				keys = append(keys, "account:"+account)
			}
			for _, key := range keys {
				if !rl.Allow(key) {
					w.Header().Set("Retry-After", "60")
					respondError(w, http.StatusTooManyRequests, "too many authentication attempts, try again later")
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func authAccountKey(r *http.Request) string {
	if r.Method != http.MethodPost || r.Body == nil {
		return ""
	}
	const maxAuthBody = 8 << 10
	data, err := io.ReadAll(io.LimitReader(r.Body, maxAuthBody+1))
	if err != nil {
		return ""
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	if len(data) > maxAuthBody {
		return ""
	}
	var request struct {
		Username string `json:"username"`
	}
	if json.Unmarshal(data, &request) != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(request.Username))
}

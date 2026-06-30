package signal

import (
	"net"
	"sync"
	"time"
)

// RateLimiter implements per-IP token bucket rate limiting.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     int           // requests per minute
	burst    int           // max burst
	interval time.Duration // cleanup interval
}

type bucket struct {
	tokens    float64
	lastLeak  time.Time
}

// NewRateLimiter creates a rate limiter allowing `rate` requests per minute per IP.
func NewRateLimiter(ratePerMinute int) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     ratePerMinute,
		burst:    ratePerMinute, // burst = full minute allowance
		interval: 5 * time.Minute,
	}

	// Cleanup stale entries periodically
	go rl.cleanupLoop()

	return rl
}

// Allow checks if a request from the given IP is allowed.
// Returns true if allowed, false if rate limited.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	b, ok := rl.buckets[ip]
	if !ok {
		rl.buckets[ip] = &bucket{
			tokens:   float64(rl.burst - 1),
			lastLeak: now,
		}
		return true
	}

	// Leak tokens based on elapsed time
	elapsed := now.Sub(b.lastLeak).Seconds()
	b.lastLeak = now
	leakRate := float64(rl.rate) / 60.0 // tokens per second
	b.tokens += elapsed * leakRate

	if b.tokens > float64(rl.burst) {
		b.tokens = float64(rl.burst)
	}

	if b.tokens < 1 {
		return false
	}

	b.tokens--
	return true
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.interval)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-rl.interval)
		for ip, b := range rl.buckets {
			if b.lastLeak.Before(cutoff) {
				delete(rl.buckets, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// extractIP returns the IP portion of an address (host:port).
func extractIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

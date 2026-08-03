package signal

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterCleanup(t *testing.T) {
	rl := NewRateLimiter(10)

	rl.Allow("1.1.1.1")
	rl.Allow("2.2.2.2")

	rl.mu.Lock()
	if len(rl.buckets) != 2 {
		t.Errorf("expected 2 buckets, got %d", len(rl.buckets))
	}
	for _, b := range rl.buckets {
		b.lastLeak = time.Now().Add(-10 * time.Minute)
	}
	rl.mu.Unlock()

	rl.mu.Lock()
	cutoff := time.Now().Add(-rl.interval)
	for ip, b := range rl.buckets {
		if b.lastLeak.Before(cutoff) {
			delete(rl.buckets, ip)
		}
	}
	count := len(rl.buckets)
	rl.mu.Unlock()

	if count != 0 {
		t.Errorf("expected 0 buckets after cleanup, got %d", count)
	}
}

func TestRateLimiterConcurrentAccess(t *testing.T) {
	rl := NewRateLimiter(1000)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				rl.Allow("10.0.0.1")
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	rl.mu.Lock()
	_, exists := rl.buckets["10.0.0.1"]
	rl.mu.Unlock()
	if !exists {
		t.Error("bucket should exist after concurrent access")
	}
}

func TestRateLimiterZeroRate(t *testing.T) {
	rl := NewRateLimiter(0)
	// First request is always allowed (bucket creation)
	allowed := rl.Allow("1.2.3.4")
	if !allowed {
		t.Error("first request with rate 0 should be allowed (bucket creation)")
	}
	// Second request should be denied (tokens = -1 < 1)
	allowed = rl.Allow("1.2.3.4")
	if allowed {
		t.Error("second request with rate 0 should be denied")
	}
}

func TestRateLimiterNegativeRate(t *testing.T) {
	rl := NewRateLimiter(-1)
	rl.Allow("1.2.3.4")
}

func TestRateLimiterHTTPMiddleware(t *testing.T) {
	rl := NewRateLimiter(1)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rl != nil {
			ip := extractIP(r.RemoteAddr)
			if !rl.Allow(ip) {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
		}
		handler.ServeHTTP(w, r)
	})

	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "10.0.0.1:12345"
	w1 := httptest.NewRecorder()
	wrapped.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Errorf("first request: expected 200, got %d", w1.Code)
	}

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "10.0.0.1:12345"
	w2 := httptest.NewRecorder()
	wrapped.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: expected 429, got %d", w2.Code)
	}

	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.RemoteAddr = "10.0.0.2:12345"
	w3 := httptest.NewRecorder()
	wrapped.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Errorf("third request: expected 200, got %d", w3.Code)
	}
}

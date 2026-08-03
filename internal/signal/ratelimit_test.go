package signal

import (
	"testing"
	"time"
)

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(5) // 5 requests per minute

	// Should allow first 5 requests
	for i := 0; i < 5; i++ {
		if !rl.Allow("1.2.3.4") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 6th should be rejected
	if rl.Allow("1.2.3.4") {
		t.Error("6th request should be rate limited")
	}
}

func TestRateLimiterDifferentIPs(t *testing.T) {
	rl := NewRateLimiter(3)

	// IP 1: use all tokens
	for i := 0; i < 3; i++ {
		if !rl.Allow("1.2.3.4") {
			t.Errorf("IP1 request %d should be allowed", i+1)
		}
	}
	if rl.Allow("1.2.3.4") {
		t.Error("IP1 should be rate limited")
	}

	// IP 2: should have its own bucket
	if !rl.Allow("5.6.7.8") {
		t.Error("IP2 should be allowed (separate bucket)")
	}
}

func TestRateLimiterTokenRefill(t *testing.T) {
	rl := NewRateLimiter(60) // 60/minute = 1/second

	// Exhaust tokens
	for i := 0; i < 60; i++ {
		rl.Allow("1.2.3.4")
	}

	if rl.Allow("1.2.3.4") {
		t.Error("should be rate limited after exhaustion")
	}

	// Simulate time passing by manipulating the bucket directly
	rl.mu.Lock()
	if b, ok := rl.buckets["1.2.3.4"]; ok {
		b.lastLeak = b.lastLeak.Add(-2 * time.Second) // 2 seconds ago = 2 tokens refilled
	}
	rl.mu.Unlock()

	// Should now allow 2 more requests
	if !rl.Allow("1.2.3.4") {
		t.Error("should be allowed after refill")
	}
	if !rl.Allow("1.2.3.4") {
		t.Error("second request should also be allowed")
	}
}

func TestExtractIP(t *testing.T) {
	cases := []struct {
		input  string
		expect string
	}{
		{"1.2.3.4:12345", "1.2.3.4"},
		{"[::1]:12345", "::1"},
		{"no-port", "no-port"},
	}
	for _, tc := range cases {
		got := extractIP(tc.input)
		if got != tc.expect {
			t.Errorf("extractIP(%q) = %q, want %q", tc.input, got, tc.expect)
		}
	}
}

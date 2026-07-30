package monitoring

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler := HealthHandler("test-version")
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}
	if resp.Version != "test-version" {
		t.Errorf("expected version 'test-version', got %q", resp.Version)
	}
	if resp.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
}

func TestReadinessHandler(t *testing.T) {
	h := NewReadinessHandler()

	// No checks — should be ready
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 for no checks, got %d", rec.Code)
	}

	// Add a failing check
	h.AddCheck("test-check", func() error {
		return nil // always passes
	})

	req2 := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("expected status 200 for passing check, got %d", rec2.Code)
	}
}

func TestMetricsRegistration(t *testing.T) {
	m := NewMetrics()
	if m == nil {
		t.Fatal("expected non-nil metrics")
	}
	if m.ConnectionsTotal == nil {
		t.Error("ConnectionsTotal should not be nil")
	}
	if m.TransfersTotal == nil {
		t.Error("TransfersTotal should not be nil")
	}
	if m.ErrorsTotal == nil {
		t.Error("ErrorsTotal should not be nil")
	}
	if m.ActiveConnections == nil {
		t.Error("ActiveConnections should not be nil")
	}
	if m.ActiveSessions == nil {
		t.Error("ActiveSessions should not be nil")
	}
	if m.HostUptimeSeconds == nil {
		t.Error("HostUptimeSeconds should not be nil")
	}
	if m.MessageLatencySeconds == nil {
		t.Error("MessageLatencySeconds should not be nil")
	}
	if m.FrameEncodingDurationSeconds == nil {
		t.Error("FrameEncodingDurationSeconds should not be nil")
	}
}

func TestMetricsHandler(t *testing.T) {
	handler := MetricsHandler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if body == "" {
		t.Error("expected non-empty metrics body")
	}
	// Should contain prometheus metrics output
	if len(body) < 100 {
		t.Errorf("expected substantial metrics output, got %d bytes", len(body))
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{5 * time.Second, "5s"},
		{1*time.Minute + 30*time.Second, "1m30s"},
		{2*time.Hour + 5*time.Minute + 10*time.Second, "2h5m10s"},
		{25*time.Hour + 30*time.Minute, "1d1h30m0s"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

// Package monitoring provides Prometheus metrics and health check endpoints
// for the remotty system.
package monitoring

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ======== Metrics Registry ========

// Metrics holds all Prometheus metric collectors for remotty.
type Metrics struct {
	// Counters
	ConnectionsTotal prometheus.Counter
	TransfersTotal   prometheus.Counter
	ErrorsTotal      *prometheus.CounterVec

	// Gauges
	ActiveConnections prometheus.Gauge
	ActiveSessions    prometheus.Gauge
	HostUptimeSeconds prometheus.Gauge

	// Histograms
	MessageLatencySeconds      prometheus.Histogram
	FrameEncodingDurationSeconds prometheus.Histogram
}

var (
	metrics     *Metrics
	metricsOnce sync.Once
	startTime   time.Time
)

// NewMetrics creates (or returns the existing singleton) remotty metric collectors.
func NewMetrics() *Metrics {
	metricsOnce.Do(func() {
		startTime = time.Now()
		metrics = &Metrics{
			ConnectionsTotal: promauto.NewCounter(prometheus.CounterOpts{
				Namespace: "remotty",
				Subsystem: "host",
				Name:      "connections_total",
				Help:      "Total number of client connections received.",
			}),
			TransfersTotal: promauto.NewCounter(prometheus.CounterOpts{
				Namespace: "remotty",
				Subsystem: "host",
				Name:      "transfers_total",
				Help:      "Total number of file transfers initiated.",
			}),
			ErrorsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
				Namespace: "remotty",
				Subsystem: "host",
				Name:      "errors_total",
				Help:      "Total number of errors, partitioned by type.",
			}, []string{"type"}),
			ActiveConnections: promauto.NewGauge(prometheus.GaugeOpts{
				Namespace: "remotty",
				Subsystem: "host",
				Name:      "active_connections",
				Help:      "Current number of active WebRTC connections.",
			}),
			ActiveSessions: promauto.NewGauge(prometheus.GaugeOpts{
				Namespace: "remotty",
				Subsystem: "host",
				Name:      "active_sessions",
				Help:      "Current number of authenticated sessions.",
			}),
			HostUptimeSeconds: promauto.NewGauge(prometheus.GaugeOpts{
				Namespace: "remotty",
				Subsystem: "host",
				Name:      "host_uptime_seconds",
				Help:      "Uptime of the host daemon in seconds.",
			}),
			MessageLatencySeconds: promauto.NewHistogram(prometheus.HistogramOpts{
				Namespace: "remotty",
				Subsystem: "host",
				Name:      "message_latency_seconds",
				Help:      "Latency of message processing in seconds.",
				Buckets:   prometheus.DefBuckets,
			}),
			FrameEncodingDurationSeconds: promauto.NewHistogram(prometheus.HistogramOpts{
				Namespace: "remotty",
				Subsystem: "host",
				Name:      "frame_encoding_duration_seconds",
				Help:      "Duration of screen frame encoding in seconds.",
				Buckets:   prometheus.DefBuckets,
			}),
		}
	})
	return metrics
}

// ======== Health Endpoints ========

// HealthResponse represents the JSON body returned by health endpoints.
type HealthResponse struct {
	Status    string `json:"status"`
	Uptime    string `json:"uptime,omitempty"`
	Timestamp string `json:"timestamp"`
	Version   string `json:"version,omitempty"`
}

// HealthHandler returns an HTTP handler that reports /health (always 200 OK).
func HealthHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := HealthResponse{
			Status:    "ok",
			Uptime:    formatDuration(time.Since(startTime)),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Version:   version,
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}

// ReadinessHandler returns an HTTP handler that reports /ready.
// It can be given a set of readiness checks. Returns 200 when ready,
// 503 when not.
type ReadinessHandler struct {
	mu       sync.RWMutex
	checks   []ReadinessCheck
}

// ReadinessCheck is a function that returns nil when the component is ready,
// or an error describing why it is not.
type ReadinessCheck func() error

// NewReadinessHandler creates a readiness handler.
func NewReadinessHandler() *ReadinessHandler {
	return &ReadinessHandler{}
}

// AddCheck registers a readiness check.
func (h *ReadinessHandler) AddCheck(name string, fn func() error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks = append(h.checks, ReadinessCheck(fn))
}

// ServeHTTP implements http.Handler.
func (h *ReadinessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var unavailable bool
	checks := make(map[string]string)

	for _, check := range h.checks {
		if err := check(); err != nil {
			unavailable = true
			checks[fmt.Sprintf("%T", check)] = err.Error()
		}
	}

	if unavailable {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "not ready",
			"checks": checks,
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(HealthResponse{
		Status:    "ready",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// ======== Metrics HTTP Handler ========

// MetricsHandler returns an HTTP handler that exposes Prometheus metrics.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// ======== Helper ========

func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	secs := int(d.Seconds()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd%dh%dm%ds", days, hours, mins, secs)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh%dm%ds", hours, mins, secs)
	}
	if mins > 0 {
		return fmt.Sprintf("%dm%ds", mins, secs)
	}
	return fmt.Sprintf("%ds", secs)
}

// ======== Uptime Tracking ========

// RecordUptime sets the host_uptime_seconds gauge to the current uptime.
// Call this periodically (e.g., every 10s) from the daemon main loop.
func RecordUptime() {
	if metrics == nil {
		return
	}
	metrics.HostUptimeSeconds.Set(time.Since(startTime).Seconds())
}

// ======== Go Runtime Metrics (optional) ========

// RecordGoMetrics exposes basic Go runtime stats as prometheus gauges.
func RecordGoMetrics() {
	if metrics == nil {
		return
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// These are optional — expose via promhttp.DefaultRegisterer metrics.
}

// Metrics returns the global Metrics instance.
func GetMetrics() *Metrics {
	return metrics
}

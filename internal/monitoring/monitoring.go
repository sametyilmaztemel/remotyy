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
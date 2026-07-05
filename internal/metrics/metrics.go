// Package metrics owns Bothan's Prometheus registry and the bothan_ metric set.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const namespace = "bothan"

// Metrics holds the application's Prometheus registry and instrument handles.
type Metrics struct {
	Registry *prometheus.Registry

	ScansTotal      *prometheus.CounterVec // labels: status
	ScanDuration    prometheus.Histogram
	ScanQueueDepth  prometheus.Gauge
	SSLLabsRequests *prometheus.CounterVec // labels: status_code
	SSLLabsCapacity *prometheus.GaugeVec   // labels: kind (max|current)
	Notifications   *prometheus.CounterVec // labels: channel_type, result
}

// New builds a registry seeded with the standard Go/process collectors and the
// bothan_ instruments.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	m := &Metrics{
		Registry: reg,
		ScansTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "scans_total", Help: "Total scans by final status.",
		}, []string{"status"}),
		ScanDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace, Name: "scan_duration_seconds", Help: "Scan wall-clock duration.",
			Buckets: []float64{15, 30, 60, 120, 300, 600, 1200},
		}),
		ScanQueueDepth: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace, Name: "scan_queue_depth", Help: "Scans queued or in progress.",
		}),
		SSLLabsRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "ssllabs_requests_total", Help: "SSL Labs API requests by HTTP status code.",
		}, []string{"status_code"}),
		SSLLabsCapacity: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace, Name: "ssllabs_capacity", Help: "SSL Labs assessment capacity.",
		}, []string{"kind"}),
		Notifications: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "notifications_total", Help: "Notifications sent by channel type and result.",
		}, []string{"channel_type", "result"}),
	}
	reg.MustRegister(m.ScansTotal, m.ScanDuration, m.ScanQueueDepth,
		m.SSLLabsRequests, m.SSLLabsCapacity, m.Notifications)
	return m
}

// Handler returns the HTTP handler exposing the registry at /metrics.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
}

// RegisterCollector adds a custom collector (e.g. the store-backed host gauges).
func (m *Metrics) RegisterCollector(c prometheus.Collector) {
	m.Registry.MustRegister(c)
}

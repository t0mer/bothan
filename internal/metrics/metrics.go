// Package metrics owns Bothan's Prometheus registry and HTTP handler. The full
// bothan_ metric set is registered here as later phases wire it up; for now the
// registry carries the standard Go and process collectors.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the application's Prometheus registry.
type Metrics struct {
	Registry *prometheus.Registry
}

// New builds a registry seeded with the standard Go and process collectors.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return &Metrics{Registry: reg}
}

// Handler returns the HTTP handler exposing the registry at /metrics.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
}

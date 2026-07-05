package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/t0mer/bothan/internal/model"
)

// HostMetricsProvider supplies per-host data for the store-backed gauges.
type HostMetricsProvider interface {
	HostMetrics(ctx context.Context) ([]model.HostMetric, error)
}

// storeCollector emits DB-derived gauges on each scrape so they never go stale.
type storeCollector struct {
	provider     HostMetricsProvider
	hostsTotal   *prometheus.Desc
	hostsByGrade *prometheus.Desc
	hostGrade    *prometheus.Desc
	certExpiry   *prometheus.Desc
}

// NewStoreCollector builds the collector for host/grade/cert gauges.
func NewStoreCollector(p HostMetricsProvider) prometheus.Collector {
	return &storeCollector{
		provider: p,
		hostsTotal: prometheus.NewDesc(namespace+"_hosts_total",
			"Number of hosts, by enabled state.", []string{"enabled"}, nil),
		hostsByGrade: prometheus.NewDesc(namespace+"_hosts_by_grade",
			"Number of hosts currently at each grade (latest ready scan).", []string{"grade"}, nil),
		hostGrade: prometheus.NewDesc(namespace+"_host_grade",
			"Numeric grade of a host's latest ready scan (see grade ordering).", []string{"host"}, nil),
		certExpiry: prometheus.NewDesc(namespace+"_cert_expiry_days",
			"Days until a host's earliest certificate expiry.", []string{"host"}, nil),
	}
}

func (c *storeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.hostsTotal
	ch <- c.hostsByGrade
	ch <- c.hostGrade
	ch <- c.certExpiry
}

func (c *storeCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hosts, err := c.provider.HostMetrics(ctx)
	if err != nil {
		return
	}

	var enabled, disabled int
	byGrade := map[string]int{}
	now := time.Now().UTC()

	for _, h := range hosts {
		if h.Enabled {
			enabled++
		} else {
			disabled++
		}
		grade := h.Grade
		if grade == "" {
			grade = "none"
		}
		byGrade[grade]++

		if h.Grade != "" {
			ch <- prometheus.MustNewConstMetric(c.hostGrade, prometheus.GaugeValue,
				float64(model.GradeRank(h.Grade)), h.Hostname)
		}
		if h.CertNotAfter != nil {
			days := h.CertNotAfter.Sub(now).Hours() / 24
			ch <- prometheus.MustNewConstMetric(c.certExpiry, prometheus.GaugeValue, days, h.Hostname)
		}
	}

	ch <- prometheus.MustNewConstMetric(c.hostsTotal, prometheus.GaugeValue, float64(enabled), "true")
	ch <- prometheus.MustNewConstMetric(c.hostsTotal, prometheus.GaugeValue, float64(disabled), "false")
	for grade, n := range byGrade {
		ch <- prometheus.MustNewConstMetric(c.hostsByGrade, prometheus.GaugeValue, float64(n), grade)
	}
}

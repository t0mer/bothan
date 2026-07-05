package metrics

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/t0mer/bothan/internal/model"
)

type fakeProvider struct{ hosts []model.HostMetric }

func (f fakeProvider) HostMetrics(context.Context) ([]model.HostMetric, error) {
	return f.hosts, nil
}

func TestStoreCollector(t *testing.T) {
	soon := time.Now().Add(10 * 24 * time.Hour)
	p := fakeProvider{hosts: []model.HostMetric{
		{Hostname: "a.com", Enabled: true, Grade: "A+", CertNotAfter: &soon},
		{Hostname: "b.com", Enabled: true, Grade: "B"},
		{Hostname: "c.com", Enabled: false, Grade: ""}, // never scanned
	}}

	reg := prometheus.NewRegistry()
	reg.MustRegister(NewStoreCollector(p))

	expected := `
# HELP bothan_hosts_total Number of hosts, by enabled state.
# TYPE bothan_hosts_total gauge
bothan_hosts_total{enabled="true"} 2
bothan_hosts_total{enabled="false"} 1
`
	if err := testutil.GatherAndCompare(reg, strings.NewReader(expected), "bothan_hosts_total"); err != nil {
		t.Errorf("hosts_total: %v", err)
	}

	// A+ should rank 8; never-scanned host emits no host_grade series.
	byGrade := `
# HELP bothan_hosts_by_grade Number of hosts currently at each grade (latest ready scan).
# TYPE bothan_hosts_by_grade gauge
bothan_hosts_by_grade{grade="A+"} 1
bothan_hosts_by_grade{grade="B"} 1
bothan_hosts_by_grade{grade="none"} 1
`
	if err := testutil.GatherAndCompare(reg, strings.NewReader(byGrade), "bothan_hosts_by_grade"); err != nil {
		t.Errorf("hosts_by_grade: %v", err)
	}

	if n := testutil.CollectAndCount(NewStoreCollector(p), "bothan_host_grade"); n != 2 {
		t.Errorf("host_grade series = %d, want 2 (never-scanned excluded)", n)
	}
	if n := testutil.CollectAndCount(NewStoreCollector(p), "bothan_cert_expiry_days"); n != 1 {
		t.Errorf("cert_expiry_days series = %d, want 1", n)
	}
}

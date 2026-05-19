package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics exposes the small set of counters and histograms the HTTP
// middleware records on every request. Keeping them on a single
// struct lets the composition root inject one value into the
// middleware constructor instead of fishing them out of a global.
type Metrics struct {
	HTTPRequests *prometheus.CounterVec
	HTTPDuration *prometheus.HistogramVec
	Registry     *prometheus.Registry
}

// NewMetrics constructs a fresh registry seeded with the Go runtime
// and process collectors plus the application-level instruments. The
// composition root mounts `Registry` at /metrics with `promhttp`.
func NewMetrics() *Metrics {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	factory := promauto.With(registry)
	return &Metrics{
		Registry: registry,
		HTTPRequests: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "HTTP requests by method, route, status.",
		}, []string{"method", "route", "status"}),
		HTTPDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),
	}
}

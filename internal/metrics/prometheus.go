package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// RequestsTotal counts all HTTP requests labeled by method, route pattern, and status code.
var RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "http_requests_total",
	Help: "Total number of HTTP requests.",
}, []string{"method", "path", "status"})

// RequestDuration records the latency of each HTTP request labeled by method and route pattern.
var RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "http_request_duration_seconds",
	Help:    "HTTP request latency.",
	Buckets: prometheus.DefBuckets,
}, []string{"method", "path"})

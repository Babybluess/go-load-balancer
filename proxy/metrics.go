package proxy

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "requests_total",
		Help: "Total number of proxied HTTP requests.",
	}, []string{"status", "backend"})

	requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "request_duration_seconds",
		Help:    "Duration of proxied HTTP requests in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"status", "backend"})
)

// MetricsHandler serves the Prometheus text exposition format for scraping.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// statusRecorder wraps a ResponseWriter to capture the status code the
// handler actually wrote, defaulting to 200 if WriteHeader is never called.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(status int) {
	rec.status = status
	rec.ResponseWriter.WriteHeader(status)
}

// observeRequest records requests_total and request_duration_seconds for a
// completed request. backend identifies which pool member served it, or
// "none" if the request never reached a backend (no route, no healthy pool).
func observeRequest(status int, backend string, start time.Time) {
	statusLabel := strconv.Itoa(status)
	requestsTotal.WithLabelValues(statusLabel, backend).Inc()
	requestDuration.WithLabelValues(statusLabel, backend).Observe(time.Since(start).Seconds())
}

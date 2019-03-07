package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

var defaultBuckets = []float64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000}

// RequestCounter tracks request counts and latencies partitioned by response
// code, HTTP method and path.
//
// The provided service should be unique to each tracked service. The registry
// is optional and will default to a process-level singleton.
//
// Latency buckets, specified by upper bound, may be optionally be provided. If
// omitted they will be set to the following default values (in milliseconds):
// []float64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000}
//
// For accuracy, buckets should mirror the distribution of the latencies of the service.
// See https://github.com/danielfm/prometheus-for-developers#quantile-estimation-errors
func RequestCounter(
	service string,
	registry prometheus.Registerer,
	buckets ...float64,
) func(next http.Handler) http.Handler {
	if registry == nil {
		registry = prometheus.DefaultRegisterer
	}
	if len(buckets) == 0 {
		buckets = defaultBuckets
	}

	labels := prometheus.Labels{"service": service}
	partitions := []string{"code", "method", "path"}

	requestsOpts := prometheus.CounterOpts{
		Name:        "request_count",
		Help:        "Request counts, partitioned by status code, method and HTTP path.",
		ConstLabels: labels,
	}
	requests := prometheus.NewCounterVec(requestsOpts, partitions)
	registry.MustRegister(requests)

	latenciesOpts := prometheus.HistogramOpts{
		Name:        "request_latency_ms",
		Help:        "Request latencies, partitioned by status code, method and HTTP path.",
		ConstLabels: labels,
		Buckets:     buckets,
	}
	latencies := prometheus.NewHistogramVec(latenciesOpts, partitions)
	registry.MustRegister(latencies)

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			// Use the route pattern instead of URL to avoid explosion in metric
			// dimensionality. The route pattern should be determined after the
			// router chain is complete, meaning this should run after 'next'.
			code := strconv.Itoa(ww.Status())
			path := chi.RouteContext(r.Context()).RoutePattern()
			requests.WithLabelValues(code, r.Method, path).Inc()
			latencies.WithLabelValues(code, r.Method, path).Observe(time.Since(start).Seconds() * 1000)
		}
		return http.HandlerFunc(fn)
	}
}

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func nilHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

type requestMetric struct {
	Name string `json:"name"`
	Help string `json:"help"`
}

func TestRequestCounter(t *testing.T) {
	registry := prometheus.NewRegistry()
	r := chi.NewRouter()
	r.Use(RequestCounter("test", registry))
	r.Route("/root", func(r chi.Router) {
		r.Get("/sub", nilHandler)
		r.Route("/{param}", func(r chi.Router) {
			r.Get("/", nilHandler)
			r.Put("/", nilHandler)
		})
	})

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/root/sub", nil))
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/root/a", nil))
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/root/b", nil))
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/root/sub", nil))

	metrics, err := registry.Gather()
	require.NoError(t, err)
	require.Len(t, metrics, 2)

	expectedCounts := []*dto.Metric{
		{
			Label: []*dto.LabelPair{
				{Name: stringPtr("code"), Value: stringPtr("200")},
				{Name: stringPtr("method"), Value: stringPtr("GET")},
				{Name: stringPtr("path"), Value: stringPtr("/root/sub")},
				{Name: stringPtr("service"), Value: stringPtr("test")},
			},
			Counter: &dto.Counter{Value: floatPtr(2)},
		},
		{
			Label: []*dto.LabelPair{
				{Name: stringPtr("code"), Value: stringPtr("200")},
				{Name: stringPtr("method"), Value: stringPtr("GET")},
				{Name: stringPtr("path"), Value: stringPtr("/root/{param}/")},
				{Name: stringPtr("service"), Value: stringPtr("test")},
			},
			Counter: &dto.Counter{Value: floatPtr(1)},
		},
		{
			Label: []*dto.LabelPair{
				{Name: stringPtr("code"), Value: stringPtr("200")},
				{Name: stringPtr("method"), Value: stringPtr("PUT")},
				{Name: stringPtr("path"), Value: stringPtr("/root/{param}/")},
				{Name: stringPtr("service"), Value: stringPtr("test")},
			},
			Counter: &dto.Counter{Value: floatPtr(1)},
		},
	}
	assert.Equal(t, "request_count", metrics[0].GetName())
	assert.Equal(t, "Request counts, partitioned by status code, method and HTTP path.", metrics[0].GetHelp())
	assert.Equal(t, dto.MetricType_COUNTER, metrics[0].GetType())
	assert.Equal(t, expectedCounts, metrics[0].GetMetric())

	// We don't directly validate latencies, but we can validate that they exist.
	assert.Equal(t, "request_latency_ms", metrics[1].GetName())
	assert.Equal(t, "Request latencies, partitioned by status code, method and HTTP path.", metrics[1].GetHelp())
	assert.Equal(t, dto.MetricType_HISTOGRAM, metrics[1].GetType())
	assert.Len(t, metrics[1].GetMetric(), 3)
}

func stringPtr(s string) *string { return &s }

func floatPtr(f float64) *float64 { return &f }

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// TestMetricsSuite orquesta las pruebas unitarias de recolección y middleware de métricas
func TestMetricsSuite(t *testing.T) {
	t.Run("Metrics Middleware Recording", testMetricsMiddlewareRecording)
	t.Run("Metrics Middleware Bypasses Scrape Endpoint", testMetricsBypassesScrapeEndpoint)
}

func testMetricsMiddlewareRecording(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	wrapped := metrics.Middleware(testHandler)

	req := httptest.NewRequest(http.MethodGet, "/test-metric", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Verificar que las métricas fueron registradas en el registro Prometheus aislado
	metricFamilies, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather prometheus metrics: %v", err)
	}

	foundCounter := false
	foundHistogram := false
	for _, mf := range metricFamilies {
		if mf.GetName() == "http_requests_total" {
			foundCounter = true
		}
		if mf.GetName() == "http_request_duration_seconds" {
			foundHistogram = true
		}
	}

	if !foundCounter {
		t.Errorf("expected http_requests_total metric to be gathered")
	}
	if !foundHistogram {
		t.Errorf("expected http_request_duration_seconds metric to be gathered")
	}
}

func testMetricsBypassesScrapeEndpoint(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)

	invoked := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		invoked = true
		w.WriteHeader(http.StatusOK)
	})

	wrapped := metrics.Middleware(testHandler)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if !invoked {
		t.Errorf("expected handler to be invoked for /metrics")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

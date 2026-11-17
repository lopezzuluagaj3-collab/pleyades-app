package httpadapter_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	httpadapter "pleyades-app/internal/adapters/http"
)

// TestHealthHandlersSuite orquesta las pruebas unitarias de los endpoints de salud
func TestHealthHandlersSuite(t *testing.T) {
	t.Run("Liveness Endpoint", testLivenessHandler)
	t.Run("Readiness Endpoint", testReadinessHandler)
}

func testLivenessHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	httpadapter.LivenessHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json header")
	}
}

func testReadinessHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	httpadapter.ReadinessHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json header")
	}
}

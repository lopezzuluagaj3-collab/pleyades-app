package httpadapter

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"pleyades-app/internal/adapters/http/middleware"
	"pleyades-app/internal/usecase"
)

// TestRouterSuite orquesta las pruebas de inicialización y ruteo del HTTP mux
func TestRouterSuite(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := middleware.NewMetrics(reg)
	rateLimiter := middleware.NewRateLimiter(100.0, 50.0, 1*time.Minute)

	userHandler := NewUserHandler(&usecase.RegisterUserUseCase{})
	authHandler := NewAuthHandler(&usecase.LoginUserUseCase{})

	router := NewRouter(userHandler, authHandler, metrics, rateLimiter)
	if router == nil {
		t.Fatal("expected non-nil router instance")
	}

	// Verificar endpoints básicos de salud
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected /healthz to return %d, got %d", http.StatusOK, rec.Code)
	}
}

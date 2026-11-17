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

	tokenService := &mockTokenServiceForHTTP{}
	authMiddleware := middleware.NewAuthMiddleware(tokenService, 5*time.Minute, 15*time.Minute)
	idempotency := middleware.NewIdempotencyMiddleware(10 * time.Minute)

	userHandler := NewUserHandler(&usecase.RegisterUserUseCase{})
	authHandler := NewAuthHandler(&usecase.LoginUserUseCase{})
	tenantHandler := NewTenantHandler(
		&usecase.CreateTenantUseCase{},
		&usecase.ResetTenantPasswordUseCase{},
	)

	router := NewRouter(
		userHandler,
		authHandler,
		tenantHandler,
		metrics,
		rateLimiter,
		authMiddleware,
		idempotency,
	)
	if router == nil {
		t.Fatal("expected non-nil router instance")
	}

	// 1. Verificar endpoints básicos de salud
	reqHealth := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recHealth := httptest.NewRecorder()
	router.ServeHTTP(recHealth, reqHealth)

	if recHealth.Code != http.StatusOK {
		t.Errorf("expected /healthz to return %d, got %d", http.StatusOK, recHealth.Code)
	}

	// 2. Verificar que la ruta protegida /api/v1/tenants intercepte y rechace peticiones no autenticadas (401)
	reqTenant := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", nil)
	recTenant := httptest.NewRecorder()
	router.ServeHTTP(recTenant, reqTenant)

	if recTenant.Code != http.StatusUnauthorized {
		t.Errorf("expected /api/v1/tenants without token to return %d, got %d", http.StatusUnauthorized, recTenant.Code)
	}

	// 3. Verificar que la ruta protegida /api/v1/tenants/{id}/reset-password también rechace peticiones no autenticadas (401)
	reqReset := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/t-123/reset-password", nil)
	recReset := httptest.NewRecorder()
	router.ServeHTTP(recReset, reqReset)

	if recReset.Code != http.StatusUnauthorized {
		t.Errorf("expected /api/v1/tenants/{id}/reset-password without token to return %d, got %d", http.StatusUnauthorized, recReset.Code)
	}

	// 4. Verificar redirección de Swagger UI
	reqSwagger := httptest.NewRequest(http.MethodGet, "/swagger", nil)
	recSwagger := httptest.NewRecorder()
	router.ServeHTTP(recSwagger, reqSwagger)
	if recSwagger.Code != http.StatusMovedPermanently {
		t.Errorf("expected /swagger to redirect with %d, got %d", http.StatusMovedPermanently, recSwagger.Code)
	}
}

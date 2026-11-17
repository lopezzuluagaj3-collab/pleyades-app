package httpadapter

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"pleyades-app/internal/adapters/http/middleware"
)

// NewRouter ensambla el árbol de rutas HTTP y aplica middlewares globales y especializados
func NewRouter(
	userHandler *UserHandler,
	authHandler *AuthHandler,
	tenantHandler *TenantHandler,
	metrics *middleware.Metrics,
	rateLimiter *middleware.RateLimiter,
	authMiddleware *middleware.AuthMiddleware,
	idempotency *middleware.IdempotencyMiddleware,
) http.Handler {
	mux := http.NewServeMux()

	// Probes de salud de Kubernetes
	mux.HandleFunc("GET /healthz", LivenessHandler)
	mux.HandleFunc("GET /readyz", ReadinessHandler)

	// Exposición de métricas Prometheus
	mux.Handle("GET /metrics", promhttp.Handler())

	// Endpoints públicos de usuarios y autenticación
	mux.HandleFunc("POST /api/v1/users", userHandler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)

	// Endpoints protegidos por JWT e Idempotencia
	mux.Handle("POST /api/v1/tenants", middleware.Chain(
		http.HandlerFunc(tenantHandler.Create),
		authMiddleware.Middleware,
		idempotency.Middleware,
	))

	return middleware.Chain(mux,
		metrics.Middleware,
		rateLimiter.Middleware,
	)
}

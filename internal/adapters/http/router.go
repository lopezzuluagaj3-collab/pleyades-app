package httpadapter

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"pleyades-app/internal/adapters/http/middleware"
)

func NewRouter(
	userHandler *UserHandler,
	authHandler *AuthHandler,
	metrics *middleware.Metrics,
	rateLimiter *middleware.RateLimiter,
) http.Handler {
	mux := http.NewServeMux()

	// Probes de salud de Kubernetes
	mux.HandleFunc("GET /healthz", LivenessHandler)
	mux.HandleFunc("GET /readyz", ReadinessHandler)

	// Exposición de métricas Prometheus
	mux.Handle("GET /metrics", promhttp.Handler())

	// Endpoints de autenticación y usuarios
	mux.HandleFunc("POST /api/v1/users", userHandler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)

	return middleware.Chain(mux,
		metrics.Middleware,
		rateLimiter.Middleware,
	)
}

package httpadapter

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewRouter(userHandler *UserHandler, metrics *Metrics) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", LivenessHandler)
	mux.HandleFunc("GET /readyz", ReadinessHandler)
	mux.HandleFunc("POST /api/v1/users", userHandler.Register)
	mux.Handle("GET /metrics", promhttp.Handler())

	return metrics.Middleware(mux)
}

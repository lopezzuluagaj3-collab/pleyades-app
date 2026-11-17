package httpadapter

import (
	"net/http"
)

func NewRouter(userHandler *UserHandler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", LivenessHandler)
	mux.HandleFunc("GET /readyz", ReadinessHandler)
	mux.HandleFunc("POST /api/v1/users", userHandler.Register)

	return mux
}

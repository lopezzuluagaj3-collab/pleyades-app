package httpadapter

import (
	"net/http"
)

func NewRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", LivenessHandler)
	mux.HandleFunc("GET /readyz", ReadinessHandler)

	return mux
}

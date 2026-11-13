package httpadapter

import (
	"net/http"
)

func LivenessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte(`{"status":"up"}`)); err != nil {
		return
	}
}

func ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte(`{"status":"up"}`)); err != nil {
		return
	}
}

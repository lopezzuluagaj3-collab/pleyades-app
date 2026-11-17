package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestIdempotencySuite orquesta las pruebas del middleware de idempotencia
func TestIdempotencySuite(t *testing.T) {
	t.Run("Pass Through Without Key", testIdempotencyWithoutKey)
	t.Run("Record and Replay Cached Response", testIdempotencyRecordAndReplay)
	t.Run("In-Flight Concurrent Conflict", testIdempotencyInFlightConflict)
}

func testIdempotencyWithoutKey(t *testing.T) {
	im := NewIdempotencyMiddleware(1 * time.Hour)
	invocations := 0

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		invocations++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	wrapped := im.Middleware(handler)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/resource", nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}

	if invocations != 3 {
		t.Fatalf("expected 3 invocations when header is absent, got %d", invocations)
	}
}

func testIdempotencyRecordAndReplay(t *testing.T) {
	im := NewIdempotencyMiddleware(1 * time.Hour)
	invocations := 0
	key := "idem-uuid-999-aaa"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		invocations++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "created",
			"id":     "res-1234",
		})
	})

	wrapped := im.Middleware(handler)

	// 1ra Petición: Se ejecuta el handler y se almacena la respuesta
	req1 := httptest.NewRequest(http.MethodPost, "/resource", nil)
	req1.Header.Set("Idempotency-Key", key)
	rec1 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on 1st request, got %d", rec1.Code)
	}
	if rec1.Header().Get("Idempotent-Replayed") == "true" {
		t.Errorf("expected 1st request to not be flagged as replayed")
	}

	// 2da Petición idéntica (Reintento de red del cliente): El handler NO debe ejecutarse de nuevo
	req2 := httptest.NewRequest(http.MethodPost, "/resource", nil)
	req2.Header.Set("Idempotency-Key", key)
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	if invocations != 1 {
		t.Fatalf("expected handler to execute only once, but ran %d times", invocations)
	}
	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected replayed status 201 Created, got %d", rec2.Code)
	}
	if rec2.Header().Get("Idempotent-Replayed") != "true" {
		t.Errorf("expected Idempotent-Replayed header on replayed response")
	}
	if rec2.Body.String() != rec1.Body.String() {
		t.Errorf("replayed body mismatch: expected %q, got %q", rec1.Body.String(), rec2.Body.String())
	}
}

func testIdempotencyInFlightConflict(t *testing.T) {
	im := NewIdempotencyMiddleware(1 * time.Hour)
	key := "idem-in-flight-key"

	started := make(chan struct{})
	continueHandler := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-continueHandler
		w.WriteHeader(http.StatusOK)
	})

	wrapped := im.Middleware(handler)

	var wg sync.WaitGroup
	wg.Add(1)

	var rec1 *httptest.ResponseRecorder
	go func() {
		defer wg.Done()
		req := httptest.NewRequest(http.MethodPost, "/resource", nil)
		req.Header.Set("Idempotency-Key", key)
		rec1 = httptest.NewRecorder()
		wrapped.ServeHTTP(rec1, req)
	}()

	// Esperar a que la 1ra petición esté dentro del handler (In-Flight)
	<-started

	// 2da petición concurrente con la misma clave mientras la 1ra sigue en curso
	req2 := httptest.NewRequest(http.MethodPost, "/resource", nil)
	req2.Header.Set("Idempotency-Key", key)
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	// La 2da petición debe recibir 409 Conflict de inmediato
	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for in-flight key, got %d", rec2.Code)
	}

	// Liberar la 1ra petición
	close(continueHandler)
	wg.Wait()

	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 1st request to complete with 200, got %d", rec1.Code)
	}
}

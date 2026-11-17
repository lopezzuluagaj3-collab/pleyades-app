package middleware

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// TestChainSuite orquesta las pruebas unitarias de la composición secuencial de middlewares
func TestChainSuite(t *testing.T) {
	t.Run("Execution Order", testChainExecutionOrder)
	t.Run("Empty Chain", testChainEmpty)
}

func testChainExecutionOrder(t *testing.T) {
	var executionTrace []string

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executionTrace = append(executionTrace, "m1_in")
			next.ServeHTTP(w, r)
			executionTrace = append(executionTrace, "m1_out")
		})
	}

	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executionTrace = append(executionTrace, "m2_in")
			next.ServeHTTP(w, r)
			executionTrace = append(executionTrace, "m2_out")
		})
	}

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		executionTrace = append(executionTrace, "handler")
		w.WriteHeader(http.StatusOK)
	})

	chained := Chain(finalHandler, m1, m2)

	req := httptest.NewRequest(http.MethodGet, "/test-chain", nil)
	rec := httptest.NewRecorder()

	chained.ServeHTTP(rec, req)

	expectedTrace := []string{"m1_in", "m2_in", "handler", "m2_out", "m1_out"}
	if !reflect.DeepEqual(executionTrace, expectedTrace) {
		t.Fatalf("expected execution order %v, got %v", expectedTrace, executionTrace)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func testChainEmpty(t *testing.T) {
	invoked := false
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		invoked = true
		w.WriteHeader(http.StatusAccepted)
	})

	chained := Chain(finalHandler)

	req := httptest.NewRequest(http.MethodGet, "/test-empty", nil)
	rec := httptest.NewRecorder()

	chained.ServeHTTP(rec, req)

	if !invoked {
		t.Errorf("expected final handler to be executed when chain is empty")
	}
	if rec.Code != http.StatusAccepted {
		t.Errorf("expected status %d, got %d", http.StatusAccepted, rec.Code)
	}
}

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestRateLimiterSuite orquesta las pruebas unitarias del algoritmo Token Bucket
func TestRateLimiterSuite(t *testing.T) {
	t.Run("Allow Within Burst", testRateLimiterAllowWithinBurst)
	t.Run("Block When Exhausted", testRateLimiterBlockWhenExhausted)
	t.Run("Refill Over Time", testRateLimiterRefill)
	t.Run("Middleware HTTP 429", testRateLimiterMiddleware429)
	t.Run("Client IP Extraction", testRateLimiterIPExtraction)
}

func testRateLimiterAllowWithinBurst(t *testing.T) {
	rl := NewRateLimiter(5.0, 3.0, 1*time.Minute)
	ip := "192.168.1.100"

	// Ráfaga permitida: 3 peticiones
	for i := 1; i <= 3; i++ {
		if !rl.Allow(ip) {
			t.Fatalf("expected request %d to be allowed within burst", i)
		}
	}
}

func testRateLimiterBlockWhenExhausted(t *testing.T) {
	rl := NewRateLimiter(1.0, 2.0, 1*time.Minute)
	ip := "10.0.0.50"

	// Consumir ráfaga (2)
	rl.Allow(ip)
	rl.Allow(ip)

	// La 3ra debe ser rechazada inmediatamente
	if rl.Allow(ip) {
		t.Fatalf("expected 3rd request to be blocked when tokens are exhausted")
	}
}

func testRateLimiterRefill(t *testing.T) {
	// 10 tokens por segundo -> 1 token cada 100ms
	rl := NewRateLimiter(10.0, 1.0, 1*time.Minute)
	ip := "10.0.0.99"

	if !rl.Allow(ip) {
		t.Fatal("expected 1st request to pass")
	}
	if rl.Allow(ip) {
		t.Fatal("expected 2nd request to fail immediately")
	}

	// Esperar 150ms para que se regenere al menos 1 token
	time.Sleep(150 * time.Millisecond)

	if !rl.Allow(ip) {
		t.Fatal("expected request to pass after token refill")
	}
}

func testRateLimiterMiddleware429(t *testing.T) {
	rl := NewRateLimiter(1.0, 1.0, 1*time.Minute)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := rl.Middleware(handler)

	// 1ra petición: OK
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "172.16.0.1:1234"
	rec1 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec1.Code)
	}

	// 2da petición inmediata de la misma IP: 429 Too Many Requests
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "172.16.0.1:1234"
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", rec2.Code)
	}
	if rec2.Header().Get("Retry-After") != "1" {
		t.Errorf("expected Retry-After header to be 1")
	}
}

func testRateLimiterIPExtraction(t *testing.T) {
	// Caso 1: X-Forwarded-For con múltiples proxies
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18, 150.172.238.178")
	if ip := extractClientIP(req1); ip != "203.0.113.195" {
		t.Errorf("expected 203.0.113.195, got %s", ip)
	}

	// Caso 2: X-Real-IP
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("X-Real-IP", "198.51.100.1")
	if ip := extractClientIP(req2); ip != "198.51.100.1" {
		t.Errorf("expected 198.51.100.1, got %s", ip)
	}

	// Caso 3: Fallback a RemoteAddr con puerto
	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.RemoteAddr = "192.0.2.1:54321"
	if ip := extractClientIP(req3); ip != "192.0.2.1" {
		t.Errorf("expected 192.0.2.1, got %s", ip)
	}
}

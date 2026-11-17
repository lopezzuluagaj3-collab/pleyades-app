package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pleyades-app/internal/ports"
)

// Mock de TokenService para pruebas unitarias aisladas
type mockTokenService struct {
	generateFunc func(uuid, email string, ttl time.Duration) (string, error)
	validateFunc func(tokenString string) (*ports.UserClaims, error)
}

func (m *mockTokenService) Generate(uuid, email string, ttl time.Duration) (string, error) {
	if m.generateFunc != nil {
		return m.generateFunc(uuid, email, ttl)
	}
	return "mock-new-token", nil
}

func (m *mockTokenService) Validate(tokenString string) (*ports.UserClaims, error) {
	if m.validateFunc != nil {
		return m.validateFunc(tokenString)
	}
	return nil, nil
}

// TestAuthMiddlewareSuite orquesta las pruebas del middleware de autenticación JWT
func TestAuthMiddlewareSuite(t *testing.T) {
	t.Run("Missing Authorization Header", testAuthMissingHeader)
	t.Run("Malformed Authorization Header", testAuthMalformedHeader)
	t.Run("Expired Token Rejection", testAuthExpiredToken)
	t.Run("Invalid Token Rejection", testAuthInvalidToken)
	t.Run("Valid Token and Context Extraction", testAuthValidToken)
	t.Run("Sliding Session Auto Renewal", testAuthSlidingRenewal)
	t.Run("UserFromContext Fallback", testUserFromContextFallback)
}

func testAuthMissingHeader(t *testing.T) {
	authMw := NewAuthMiddleware(&mockTokenService{}, 5*time.Minute, 15*time.Minute)
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()

	authMw.Middleware(dummyHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func testAuthMalformedHeader(t *testing.T) {
	authMw := NewAuthMiddleware(&mockTokenService{}, 5*time.Minute, 15*time.Minute)
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	malformedHeaders := []string{
		"Basic dXNlcjpwYXNz",
		"Bearer",
		"Bearer   ",
		"Token 12345",
	}

	for _, headerVal := range malformedHeaders {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", headerVal)
		rec := httptest.NewRecorder()

		authMw.Middleware(dummyHandler).ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for header %q, got %d", headerVal, rec.Code)
		}
	}
}

func testAuthExpiredToken(t *testing.T) {
	ts := &mockTokenService{
		validateFunc: func(tokenString string) (*ports.UserClaims, error) {
			return nil, ports.ErrExpiredToken
		},
	}
	authMw := NewAuthMiddleware(ts, 5*time.Minute, 15*time.Minute)
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer expired-token")
	rec := httptest.NewRecorder()

	authMw.Middleware(dummyHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token, got %d", rec.Code)
	}
}

func testAuthInvalidToken(t *testing.T) {
	ts := &mockTokenService{
		validateFunc: func(tokenString string) (*ports.UserClaims, error) {
			return nil, ports.ErrInvalidToken
		},
	}
	authMw := NewAuthMiddleware(ts, 5*time.Minute, 15*time.Minute)
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer forged-token")
	rec := httptest.NewRecorder()

	authMw.Middleware(dummyHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for forged token, got %d", rec.Code)
	}
}

func testAuthValidToken(t *testing.T) {
	expectedClaims := &ports.UserClaims{
		UUID:      "user-uuid-12345",
		Email:     "tenant_admin@pleyades.io",
		ExpiresAt: time.Now().Add(10 * time.Minute), // Más de 5 minutos, no debe renovar
	}

	ts := &mockTokenService{
		validateFunc: func(tokenString string) (*ports.UserClaims, error) {
			return expectedClaims, nil
		},
	}

	authMw := NewAuthMiddleware(ts, 5*time.Minute, 15*time.Minute)

	handlerExecuted := false
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerExecuted = true
		claims, ok := UserFromContext(r.Context())
		if !ok || claims == nil {
			t.Errorf("expected claims in context, got none")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if claims.UUID != expectedClaims.UUID || claims.Email != expectedClaims.Email {
			t.Errorf("claims in context mismatch: expected %v, got %v", expectedClaims, claims)
		}
		w.WriteHeader(http.StatusOK)
	})

	flexibleHeaders := []string{
		"Bearer valid-token",
		"\"Bearer valid-token\"",
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.token",
		"\"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.token\"",
	}

	for _, headerVal := range flexibleHeaders {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", headerVal)
		rec := httptest.NewRecorder()

		handlerExecuted = false
		authMw.Middleware(dummyHandler).ServeHTTP(rec, req)

		if !handlerExecuted {
			t.Errorf("expected protected handler to execute for header %s", headerVal)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 for header %s, got %d", headerVal, rec.Code)
		}
		if rec.Header().Get("X-Renewed-Token") != "" {
			t.Errorf("expected no X-Renewed-Token when remaining TTL is above threshold")
		}
	}
}

func testAuthSlidingRenewal(t *testing.T) {
	// Token al que le quedan solo 2 minutos (menos de los 5 minutos de umbral)
	claimsNearExpiration := &ports.UserClaims{
		UUID:      "user-uuid-12345",
		Email:     "tenant_admin@pleyades.io",
		ExpiresAt: time.Now().Add(2 * time.Minute),
	}

	renewedTokenValue := "new-fresh-sliding-token-jwt"
	ts := &mockTokenService{
		validateFunc: func(tokenString string) (*ports.UserClaims, error) {
			return claimsNearExpiration, nil
		},
		generateFunc: func(uuid, email string, ttl time.Duration) (string, error) {
			return renewedTokenValue, nil
		},
	}

	authMw := NewAuthMiddleware(ts, 5*time.Minute, 15*time.Minute)
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer near-expiry-token")
	rec := httptest.NewRecorder()

	authMw.Middleware(dummyHandler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	renewedHeader := rec.Header().Get("X-Renewed-Token")
	if renewedHeader != renewedTokenValue {
		t.Errorf("expected X-Renewed-Token header %q, got %q", renewedTokenValue, renewedHeader)
	}
}

func testUserFromContextFallback(t *testing.T) {
	claims, ok := UserFromContext(context.Background())
	if ok || claims != nil {
		t.Errorf("expected false and nil for empty context, got %v, %v", ok, claims)
	}
}

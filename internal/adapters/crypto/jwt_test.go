package crypto

import (
	"errors"
	"testing"
	"time"

	"pleyades-app/internal/ports"
)

// TestJWTTokenServiceSuite orquesta las pruebas unitarias de emisión y verificación de JWT
func TestJWTTokenServiceSuite(t *testing.T) {
	t.Run("Generate and Validate Success", testJWTSuccess)
	t.Run("Expired Token Failure", testJWTExpired)
	t.Run("Forged Signature / Wrong Secret Failure", testJWTWrongSecret)
	t.Run("Malformed Token String Failure", testJWTMalformed)
}

func testJWTSuccess(t *testing.T) {
	service := NewJWTTokenService("super-secret-key-1234567890-enterprise")

	uuid := "550e8400-e29b-41d4-a716-446655440000"
	email := "architect@enterprise.io"
	ttl := 15 * time.Minute

	token, err := service.Generate(uuid, email, ttl)
	if err != nil {
		t.Fatalf("unexpected error generating token: %v", err)
	}

	claims, err := service.Validate(token)
	if err != nil {
		t.Fatalf("unexpected error validating valid token: %v", err)
	}

	if claims.UUID != uuid {
		t.Errorf("expected uuid %s, got %s", uuid, claims.UUID)
	}
	if claims.Email != email {
		t.Errorf("expected email %s, got %s", email, claims.Email)
	}
	if claims.ExpiresAt.Before(time.Now()) {
		t.Errorf("expected expiration in the future, got %v", claims.ExpiresAt)
	}
}

func testJWTExpired(t *testing.T) {
	service := NewJWTTokenService("super-secret-key-1234567890-enterprise")

	// Emitir token con TTL negativo (ya expirado)
	token, err := service.Generate("uuid", "test@enterprise.io", -1*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error generating expired token: %v", err)
	}

	_, err = service.Validate(token)
	if !errors.Is(err, ports.ErrExpiredToken) {
		t.Fatalf("expected ErrExpiredToken, got %v", err)
	}
}

func testJWTWrongSecret(t *testing.T) {
	signerService := NewJWTTokenService("secret-A-1234567890")
	verifierService := NewJWTTokenService("secret-B-0987654321")

	token, err := signerService.Generate("uuid", "user@enterprise.io", 15*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error generating token: %v", err)
	}

	// El verificador usa un secreto diferente -> Debe rechazar por firma manipulada
	_, err = verifierService.Validate(token)
	if !errors.Is(err, ports.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken for mismatched secret, got %v", err)
	}
}

func testJWTMalformed(t *testing.T) {
	service := NewJWTTokenService("super-secret-key-1234567890-enterprise")

	_, err := service.Validate("this.is.not.a.valid.jwt.token")
	if !errors.Is(err, ports.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken for malformed string, got %v", err)
	}
}

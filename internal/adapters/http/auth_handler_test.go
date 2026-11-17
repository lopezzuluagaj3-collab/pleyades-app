package httpadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pleyades-app/internal/domain"
	"pleyades-app/internal/ports"
	"pleyades-app/internal/usecase"
)

// Mock de TokenService para pruebas HTTP
type mockTokenServiceForHTTP struct {
	generateFunc func(uuid, email string, ttl time.Duration) (string, error)
	validateFunc func(tokenString string) (*ports.UserClaims, error)
}

func (m *mockTokenServiceForHTTP) Generate(uuid, email string, ttl time.Duration) (string, error) {
	if m.generateFunc != nil {
		return m.generateFunc(uuid, email, ttl)
	}
	return "mock-jwt-token", nil
}

func (m *mockTokenServiceForHTTP) Validate(tokenString string) (*ports.UserClaims, error) {
	if m.validateFunc != nil {
		return m.validateFunc(tokenString)
	}
	return nil, nil
}

// TestAuthHandlerSuite orquesta las pruebas unitarias del handler HTTP de autenticación
func TestAuthHandlerSuite(t *testing.T) {
	t.Run("Login Success", testLoginHandlerSuccess)
	t.Run("Login Malformed JSON", testLoginHandlerMalformedJSON)
	t.Run("Login Invalid Credentials", testLoginHandlerInvalidCredentials)
	t.Run("Login Internal Server Error", testLoginHandlerInternalError)
}

func testLoginHandlerSuccess(t *testing.T) {
	user := &domain.User{
		UUID:     "uuid-1234",
		Email:    "user@enterprise.io",
		Password: "hashed-password",
		Name:     "Enterprise User",
	}

	repo := &mockUserRepoForHTTP{
		findByEmailFunc: func(ctx context.Context, email string) (*domain.User, error) {
			return user, nil
		},
	}
	hasher := &mockHasherForHTTP{}
	tokenService := &mockTokenServiceForHTTP{
		generateFunc: func(uuid, email string, ttl time.Duration) (string, error) {
			return "valid-signed-token", nil
		},
	}

	uc := usecase.NewLoginUserUseCase(repo, hasher, tokenService, 15*time.Minute)
	handler := NewAuthHandler(uc)

	body := `{"email":"user@enterprise.io","password":"SecurePassword1!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var resp LoginResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.AccessToken != "valid-signed-token" {
		t.Errorf("expected access_token valid-signed-token, got %s", resp.AccessToken)
	}
	if resp.TokenType != "Bearer" {
		t.Errorf("expected token_type Bearer, got %s", resp.TokenType)
	}
}

func testLoginHandlerMalformedJSON(t *testing.T) {
	uc := usecase.NewLoginUserUseCase(&mockUserRepoForHTTP{}, &mockHasherForHTTP{}, &mockTokenServiceForHTTP{}, 15*time.Minute)
	handler := NewAuthHandler(uc)

	body := `{"email": "broken-json`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for malformed json, got %d", rec.Code)
	}
}

func testLoginHandlerInvalidCredentials(t *testing.T) {
	repo := &mockUserRepoForHTTP{
		findByEmailFunc: func(ctx context.Context, email string) (*domain.User, error) {
			return nil, nil // Usuario no encontrado
		},
	}
	uc := usecase.NewLoginUserUseCase(repo, &mockHasherForHTTP{}, &mockTokenServiceForHTTP{}, 15*time.Minute)
	handler := NewAuthHandler(uc)

	body := `{"email":"unknown@enterprise.io","password":"WrongPassword!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for invalid credentials, got %d", rec.Code)
	}
}

func testLoginHandlerInternalError(t *testing.T) {
	user := &domain.User{
		UUID:     "uuid-1234",
		Email:    "user@enterprise.io",
		Password: "hashed-password",
	}
	repo := &mockUserRepoForHTTP{
		findByEmailFunc: func(ctx context.Context, email string) (*domain.User, error) {
			return user, nil
		},
	}
	tokenService := &mockTokenServiceForHTTP{
		generateFunc: func(uuid, email string, ttl time.Duration) (string, error) {
			return "", errors.New("jwt signing key failure")
		},
	}
	uc := usecase.NewLoginUserUseCase(repo, &mockHasherForHTTP{}, tokenService, 15*time.Minute)
	handler := NewAuthHandler(uc)

	body := `{"email":"user@enterprise.io","password":"ValidPassword1!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 for internal error, got %d", rec.Code)
	}
}

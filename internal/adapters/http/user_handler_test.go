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

	"pleyades-app/internal/domain"
	"pleyades-app/internal/usecase"
)

// Mock de repositorio para desacoplar las pruebas HTTP de la capa de persistencia
type mockUserRepoForHTTP struct {
	findByEmailFunc func(ctx context.Context, email string) (*domain.User, error)
	saveFunc        func(ctx context.Context, u domain.User) error
}

func (m *mockUserRepoForHTTP) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	if m.findByEmailFunc != nil {
		return m.findByEmailFunc(ctx, email)
	}
	return nil, nil
}

func (m *mockUserRepoForHTTP) Save(ctx context.Context, u domain.User) error {
	if m.saveFunc != nil {
		return m.saveFunc(ctx, u)
	}
	return nil
}

func (m *mockUserRepoForHTTP) Delete(ctx context.Context, uuid string) error {
	return nil
}

// Mock de hasher para pruebas HTTP
type mockHasherForHTTP struct {
	hashFunc func(password string) (string, error)
}

func (m *mockHasherForHTTP) Hash(password string) (string, error) {
	if m.hashFunc != nil {
		return m.hashFunc(password)
	}
	return "hashed_secret", nil
}

func (m *mockHasherForHTTP) Compare(hashedPassword, password string) error {
	return nil
}

// TestUserHandlerSuite orquesta las pruebas atómicas del handler HTTP de usuarios
func TestUserHandlerSuite(t *testing.T) {
	t.Run("Register Success", testRegisterSuccess)
	t.Run("Register Malformed JSON", testRegisterMalformedJSON)
	t.Run("Register Email Conflict", testRegisterEmailConflict)
	t.Run("Register Validation Failure", testRegisterValidationFailure)
	t.Run("Register Internal Server Error", testRegisterInternalServerError)
	t.Run("Conflict Error Mapping", testConflictMapping)
	t.Run("Validation Errors Mapping", testValidationMapping)
	t.Run("Unexpected Error Mapping", testUnexpectedErrorMapping)
}

func testRegisterSuccess(t *testing.T) {
	repo := &mockUserRepoForHTTP{
		findByEmailFunc: func(ctx context.Context, email string) (*domain.User, error) {
			return nil, nil // Email disponible
		},
		saveFunc: func(ctx context.Context, u domain.User) error {
			return nil // Guardado exitoso
		},
	}
	hasher := &mockHasherForHTTP{}
	uc := usecase.NewRegisterUserUseCase(repo, hasher)
	handler := NewUserHandler(uc)

	body := `{"email":"test@enterprise.io","password":"SecurePassword1!","name":"Enterprise User"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var resp UserResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Email != "test@enterprise.io" {
		t.Errorf("expected email test@enterprise.io, got %s", resp.Email)
	}
	if resp.Name != "Enterprise User" {
		t.Errorf("expected name Enterprise User, got %s", resp.Name)
	}
	if resp.UUID == "" {
		t.Errorf("expected non-empty UUID in response")
	}
}

func testRegisterMalformedJSON(t *testing.T) {
	uc := usecase.NewRegisterUserUseCase(&mockUserRepoForHTTP{}, &mockHasherForHTTP{})
	handler := NewUserHandler(uc)

	body := `{"email": "broken-json-no-closing-quote}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for malformed JSON, got %d", http.StatusBadRequest, rec.Code)
	}
}

func testRegisterEmailConflict(t *testing.T) {
	repo := &mockUserRepoForHTTP{
		findByEmailFunc: func(ctx context.Context, email string) (*domain.User, error) {
			return &domain.User{Email: email}, nil // Conflicto
		},
	}
	hasher := &mockHasherForHTTP{}
	uc := usecase.NewRegisterUserUseCase(repo, hasher)
	handler := NewUserHandler(uc)

	body := `{"email":"existing@enterprise.io","password":"SecurePassword1!","name":"User"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected status %d for conflict, got %d", http.StatusConflict, rec.Code)
	}
}

func testRegisterValidationFailure(t *testing.T) {
	repo := &mockUserRepoForHTTP{}
	hasher := &mockHasherForHTTP{}
	uc := usecase.NewRegisterUserUseCase(repo, hasher)
	handler := NewUserHandler(uc)

	body := `{"email":"invalid-email-no-at","password":"weak","name":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for validation error, got %d", http.StatusBadRequest, rec.Code)
	}
}

func testRegisterInternalServerError(t *testing.T) {
	repo := &mockUserRepoForHTTP{
		saveFunc: func(ctx context.Context, u domain.User) error {
			return errors.New("db disk write failure")
		},
	}
	hasher := &mockHasherForHTTP{}
	uc := usecase.NewRegisterUserUseCase(repo, hasher)
	handler := NewUserHandler(uc)

	body := `{"email":"test@enterprise.io","password":"SecurePassword1!","name":"Enterprise User"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d for internal error, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func testConflictMapping(t *testing.T) {
	status, msg := mapError(domain.ErrEmailAlreadyExists)
	if status != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, status)
	}
	if msg != domain.ErrEmailAlreadyExists.Error() {
		t.Errorf("expected message %s, got %s", domain.ErrEmailAlreadyExists.Error(), msg)
	}
}

func testValidationMapping(t *testing.T) {
	validationErrors := []error{
		domain.ErrEmptyEmail,
		domain.ErrArroaEmail,
		domain.ErrEmptyName,
		domain.ErrLongName,
		domain.ErrName,
		domain.ErrEmptyPass,
		domain.ErrValidatePass,
	}

	for _, err := range validationErrors {
		t.Run(err.Error(), func(t *testing.T) {
			status, msg := mapError(err)
			if status != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, status)
			}
			if msg != err.Error() {
				t.Errorf("expected message %s, got %s", err.Error(), msg)
			}
		})
	}
}

func testUnexpectedErrorMapping(t *testing.T) {
	unknownErr := errors.New("unexpected database partition failure")
	status, msg := mapError(unknownErr)
	if status != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, status)
	}
	if msg != "error interno del servidor" {
		t.Errorf("expected sanitized internal message, got %s", msg)
	}
}

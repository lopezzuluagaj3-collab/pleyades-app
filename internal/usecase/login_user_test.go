package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"pleyades-app/internal/domain"
	"pleyades-app/internal/ports"
	"pleyades-app/internal/usecase"
)

// Mock de TokenService para pruebas unitarias de casos de uso
type mockTokenServiceForUseCase struct {
	generateFunc func(uuid, email string, ttl time.Duration) (string, error)
	validateFunc func(tokenString string) (*ports.UserClaims, error)
}

func (m *mockTokenServiceForUseCase) Generate(uuid, email string, ttl time.Duration) (string, error) {
	if m.generateFunc != nil {
		return m.generateFunc(uuid, email, ttl)
	}
	return "mock-jwt-token-string", nil
}

func (m *mockTokenServiceForUseCase) Validate(tokenString string) (*ports.UserClaims, error) {
	if m.validateFunc != nil {
		return m.validateFunc(tokenString)
	}
	return nil, nil
}

// TestLoginUserUseCaseSuite orquesta las pruebas unitarias del caso de uso de inicio de sesión
func TestLoginUserUseCaseSuite(t *testing.T) {
	t.Run("Happy Path Login", testLoginHappyPath)
	t.Run("Empty or Invalid Email", testLoginInvalidEmail)
	t.Run("Empty Password", testLoginEmptyPassword)
	t.Run("User Not Found Rejection", testLoginUserNotFound)
	t.Run("Password Mismatch Rejection", testLoginPasswordMismatch)
	t.Run("Token Generation Failure", testLoginTokenServiceFailure)
}

func testLoginHappyPath(t *testing.T) {
	repo := newMockUserRepository()
	_ = repo.Save(context.Background(), domain.User{
		UUID:     "uuid-1234-5678",
		Email:    "architect@pleyades.io",
		Password: "hashed_CorrectPassword123!",
		Name:     "Architect",
	})

	hasher := &mockPasswordHasher{
		compareErr: nil, // Contraseña correcta
	}

	tokenService := &mockTokenServiceForUseCase{
		generateFunc: func(uuid, email string, ttl time.Duration) (string, error) {
			return "valid-access-token-jwt", nil
		},
	}

	uc := usecase.NewLoginUserUseCase(repo, hasher, tokenService, 15*time.Minute)

	output, err := uc.Execute(context.Background(), usecase.LoginUserInput{
		Email:    "architect@pleyades.io",
		Password: "CorrectPassword123!",
	})

	if err != nil {
		t.Fatalf("unexpected error on valid login: %v", err)
	}

	if output.AccessToken != "valid-access-token-jwt" {
		t.Errorf("expected token valid-access-token-jwt, got %s", output.AccessToken)
	}
	if output.TokenType != "Bearer" {
		t.Errorf("expected token type Bearer, got %s", output.TokenType)
	}
	if output.ExpiresIn != 900 {
		t.Errorf("expected expires_in 900, got %d", output.ExpiresIn)
	}
}

func testLoginInvalidEmail(t *testing.T) {
	repo := newMockUserRepository()
	hasher := &mockPasswordHasher{}
	ts := &mockTokenServiceForUseCase{}
	uc := usecase.NewLoginUserUseCase(repo, hasher, ts, 15*time.Minute)

	invalidEmails := []string{"", "   ", "invalid-no-at"}
	for _, email := range invalidEmails {
		_, err := uc.Execute(context.Background(), usecase.LoginUserInput{
			Email:    email,
			Password: "Password123!",
		})
		if !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Errorf("expected ErrInvalidCredentials for email %q, got %v", email, err)
		}
	}
}

func testLoginEmptyPassword(t *testing.T) {
	repo := newMockUserRepository()
	hasher := &mockPasswordHasher{}
	ts := &mockTokenServiceForUseCase{}
	uc := usecase.NewLoginUserUseCase(repo, hasher, ts, 15*time.Minute)

	_, err := uc.Execute(context.Background(), usecase.LoginUserInput{
		Email:    "valid@pleyades.io",
		Password: "",
	})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials for empty password, got %v", err)
	}
}

func testLoginUserNotFound(t *testing.T) {
	repo := newMockUserRepository() // Vacío
	hasher := &mockPasswordHasher{}
	ts := &mockTokenServiceForUseCase{}
	uc := usecase.NewLoginUserUseCase(repo, hasher, ts, 15*time.Minute)

	_, err := uc.Execute(context.Background(), usecase.LoginUserInput{
		Email:    "nonexistent@pleyades.io",
		Password: "Password123!",
	})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials for unknown user, got %v", err)
	}
	if !hasher.compareDummyCalled {
		t.Fatalf("expected CompareDummy to be executed to mitigate timing attack, but it was not invoked")
	}
}

func testLoginPasswordMismatch(t *testing.T) {
	repo := newMockUserRepository()
	_ = repo.Save(context.Background(), domain.User{
		UUID:     "uuid-1234",
		Email:    "user@pleyades.io",
		Password: "hashed-secret",
	})

	hasher := &mockPasswordHasher{
		compareErr: errors.New("password mismatch"),
	}
	ts := &mockTokenServiceForUseCase{}
	uc := usecase.NewLoginUserUseCase(repo, hasher, ts, 15*time.Minute)

	_, err := uc.Execute(context.Background(), usecase.LoginUserInput{
		Email:    "user@pleyades.io",
		Password: "WrongPassword!",
	})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials on password mismatch, got %v", err)
	}
}

func testLoginTokenServiceFailure(t *testing.T) {
	repo := newMockUserRepository()
	_ = repo.Save(context.Background(), domain.User{
		UUID:     "uuid-1234",
		Email:    "user@pleyades.io",
		Password: "hashed-secret",
	})
	hasher := &mockPasswordHasher{}
	tokenService := &mockTokenServiceForUseCase{
		generateFunc: func(uuid, email string, ttl time.Duration) (string, error) {
			return "", errors.New("entropy error in token signing")
		},
	}

	uc := usecase.NewLoginUserUseCase(repo, hasher, tokenService, 15*time.Minute)

	_, err := uc.Execute(context.Background(), usecase.LoginUserInput{
		Email:    "user@pleyades.io",
		Password: "Password123!",
	})
	if err == nil {
		t.Fatalf("expected error when token service fails, got nil")
	}
}

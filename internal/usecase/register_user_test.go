package usecase_test

import (
	"context"
	"errors"
	"testing"

	"pleyades-app/internal/domain"
	"pleyades-app/internal/usecase"
)

// Mock in-memory del repositorio de usuarios
type mockUserRepository struct {
	users          map[string]domain.User
	saveErr        error
	findByEmailErr error
}

func newMockUserRepository() *mockUserRepository {
	return &mockUserRepository{
		users: make(map[string]domain.User),
	}
}

func (m *mockUserRepository) Save(ctx context.Context, u domain.User) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.users[u.Email] = u
	return nil
}

func (m *mockUserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	if m.findByEmailErr != nil {
		return nil, m.findByEmailErr
	}
	u, exists := m.users[email]
	if !exists {
		return nil, domain.ErrUserNotFound
	}
	return &u, nil
}

func (m *mockUserRepository) Delete(ctx context.Context, email string) error {
	delete(m.users, email)
	return nil
}

// Mock del hasher de contraseñas
type mockPasswordHasher struct {
	hashErr            error
	compareErr         error
	compareDummyCalled bool
}

func (m *mockPasswordHasher) Hash(password string) (string, error) {
	if m.hashErr != nil {
		return "", m.hashErr
	}
	return "hashed_" + password, nil
}

func (m *mockPasswordHasher) Compare(hashedPassword, password string) error {
	if m.compareErr != nil {
		return m.compareErr
	}
	if hashedPassword != "hashed_"+password {
		return errors.New("hash mismatch")
	}
	return nil
}

func (m *mockPasswordHasher) CompareDummy(password string) error {
	m.compareDummyCalled = true
	return nil
}

// TestRegisterUserUseCaseSuite es el orquestador principal de las pruebas del caso de uso
func TestRegisterUserUseCaseSuite(t *testing.T) {
	t.Run("Happy Path Registration", testRegisterUserHappyPath)
	t.Run("Domain Validation Short-Circuit", testRegisterUserValidationShortCircuit)
	t.Run("Duplicate Email Conflict", testRegisterUserDuplicateConflict)
	t.Run("Password Hashing Failure", testRegisterUserHasherFailure)
	t.Run("Repository Save Failure", testRegisterUserSaveFailure)
}

func testRegisterUserHappyPath(t *testing.T) {
	repo := newMockUserRepository()
	hasher := &mockPasswordHasher{}
	uc := usecase.NewRegisterUserUseCase(repo, hasher)

	input := usecase.RegisterUserInput{
		UUID:     "custom-uuid-123",
		Email:    "dev@pleyades.io",
		Name:     "Cloud Architect",
		Password: "Password123!",
	}

	user, err := uc.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if user.UUID != input.UUID {
		t.Errorf("expected UUID %s, got %s", input.UUID, user.UUID)
	}
	if user.Password != "hashed_Password123!" {
		t.Errorf("expected password to be hashed, got %s", user.Password)
	}
	if _, exists := repo.users[input.Email]; !exists {
		t.Errorf("expected user to be persisted in repository")
	}
}

func testRegisterUserValidationShortCircuit(t *testing.T) {
	repo := newMockUserRepository()
	hasher := &mockPasswordHasher{}
	uc := usecase.NewRegisterUserUseCase(repo, hasher)

	input := usecase.RegisterUserInput{
		Email:    "invalid-email",
		Name:     "Cloud Architect",
		Password: "Password123!",
	}

	user, err := uc.Execute(context.Background(), input)
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}
	if !errors.Is(err, domain.ErrArroaEmail) {
		t.Errorf("expected ErrArroaEmail, got %v", err)
	}
	if user != nil {
		t.Errorf("expected user to be nil on error")
	}
	if len(repo.users) > 0 {
		t.Errorf("repository should not be touched when validation fails")
	}
}

func testRegisterUserDuplicateConflict(t *testing.T) {
	repo := newMockUserRepository()
	hasher := &mockPasswordHasher{}
	uc := usecase.NewRegisterUserUseCase(repo, hasher)

	// Pre-existente
	existingEmail := "dev@pleyades.io"
	repo.users[existingEmail] = domain.User{
		Email: existingEmail,
		Name:  "Existing Dev",
	}

	input := usecase.RegisterUserInput{
		Email:    existingEmail,
		Name:     "Duplicate Dev",
		Password: "Password123!",
	}

	_, err := uc.Execute(context.Background(), input)
	if err == nil {
		t.Fatalf("expected duplicate conflict error, got nil")
	}
	if !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Errorf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func testRegisterUserHasherFailure(t *testing.T) {
	repo := newMockUserRepository()
	expectedHashErr := errors.New("cryptographic entropy exhaustion")
	hasher := &mockPasswordHasher{hashErr: expectedHashErr}
	uc := usecase.NewRegisterUserUseCase(repo, hasher)

	input := usecase.RegisterUserInput{
		Email:    "dev@pleyades.io",
		Name:     "Cloud Dev",
		Password: "Password123!",
	}

	_, err := uc.Execute(context.Background(), input)
	if err == nil {
		t.Fatalf("expected hasher error, got nil")
	}
	if !errors.Is(err, expectedHashErr) {
		t.Errorf("expected %v, got %v", expectedHashErr, err)
	}
	if len(repo.users) > 0 {
		t.Errorf("repository should not persist user when hashing fails")
	}
}

func testRegisterUserSaveFailure(t *testing.T) {
	repo := newMockUserRepository()
	expectedDBErr := errors.New("database disk full")
	repo.saveErr = expectedDBErr
	hasher := &mockPasswordHasher{}
	uc := usecase.NewRegisterUserUseCase(repo, hasher)

	input := usecase.RegisterUserInput{
		Email:    "dev@pleyades.io",
		Name:     "Cloud Dev",
		Password: "Password123!",
	}

	_, err := uc.Execute(context.Background(), input)
	if err == nil {
		t.Fatalf("expected database error, got nil")
	}
	if !errors.Is(err, expectedDBErr) {
		t.Errorf("expected %v, got %v", expectedDBErr, err)
	}
}

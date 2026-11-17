package usecase_test

import (
	"context"
	"errors"
	"testing"

	"pleyades-app/internal/domain"
	"pleyades-app/internal/ports"
	"pleyades-app/internal/usecase"
)

type mockPasswordResetter struct {
	resetErr     error
	resetCalls   []string
	mockNewCreds *ports.TenantDBCredentials
}

func (m *mockPasswordResetter) ResetPassword(ctx context.Context, slug string) (*ports.TenantDBCredentials, error) {
	m.resetCalls = append(m.resetCalls, slug)
	if m.resetErr != nil {
		return nil, m.resetErr
	}
	return m.mockNewCreds, nil
}

func TestResetTenantPasswordUseCaseSuite(t *testing.T) {
	t.Run("Happy Path Password Reset", testResetPasswordHappyPath)
	t.Run("Validation Failures", testResetPasswordValidationFailures)
	t.Run("Tenant Not Found Failure", testResetPasswordTenantNotFound)
	t.Run("Forbidden Access When Not Owner", testResetPasswordForbiddenNotOwner)
	t.Run("Provisioner Failure Propagation", testResetPasswordProvisionerFailure)
}

func testResetPasswordHappyPath(t *testing.T) {
	repo := newMockTenantRepository()
	newCreds := &ports.TenantDBCredentials{
		Host:     "localhost",
		Port:     5432,
		DBName:   "tenant_acme",
		Username: "usr_acme",
		Password: "new-super-secret-pass",
	}
	newCreds.ConnectionString = newCreds.BuildConnectionString("disable")
	resetter := &mockPasswordResetter{
		mockNewCreds: newCreds,
	}

	tenant := domain.Tenant{
		ID:          "tenant-uuid-1",
		Name:        "Acme",
		Slug:        "acme",
		OwnerUserID: "owner-uuid-1",
		Status:      domain.TenantStatusActive,
	}
	_ = repo.Save(context.Background(), tenant)

	uc := usecase.NewResetTenantPasswordUseCase(repo, resetter)
	output, err := uc.Execute(context.Background(), usecase.ResetTenantPasswordInput{
		TenantID:        "tenant-uuid-1",
		RequesterUserID: "owner-uuid-1",
	})
	if err != nil {
		t.Fatalf("unexpected error on valid reset: %v", err)
	}

	if output.TenantID != "tenant-uuid-1" {
		t.Errorf("expected TenantID 'tenant-uuid-1', got '%s'", output.TenantID)
	}
	if output.Database == nil || output.Database.Password != "new-super-secret-pass" {
		t.Errorf("expected new password in credentials, got %v", output.Database)
	}
	if len(resetter.resetCalls) != 1 || resetter.resetCalls[0] != "acme" {
		t.Errorf("expected 1 call to ResetPassword with 'acme', got %v", resetter.resetCalls)
	}
}

func testResetPasswordValidationFailures(t *testing.T) {
	repo := newMockTenantRepository()
	resetter := &mockPasswordResetter{}
	uc := usecase.NewResetTenantPasswordUseCase(repo, resetter)

	t.Run("Empty TenantID fails", func(t *testing.T) {
		_, err := uc.Execute(context.Background(), usecase.ResetTenantPasswordInput{
			TenantID:        "",
			RequesterUserID: "user-1",
		})
		if !errors.Is(err, domain.ErrTenantIDRequired) {
			t.Errorf("expected ErrTenantIDRequired, got %v", err)
		}
	})

	t.Run("Empty RequesterUserID fails", func(t *testing.T) {
		_, err := uc.Execute(context.Background(), usecase.ResetTenantPasswordInput{
			TenantID:        "tenant-1",
			RequesterUserID: "",
		})
		if !errors.Is(err, domain.ErrEmptyTenantOwnerID) {
			t.Errorf("expected ErrEmptyTenantOwnerID, got %v", err)
		}
	})
}

func testResetPasswordTenantNotFound(t *testing.T) {
	repo := newMockTenantRepository()
	resetter := &mockPasswordResetter{}
	uc := usecase.NewResetTenantPasswordUseCase(repo, resetter)

	_, err := uc.Execute(context.Background(), usecase.ResetTenantPasswordInput{
		TenantID:        "non-existent-tenant",
		RequesterUserID: "user-1",
	})
	if !errors.Is(err, domain.ErrTenantNotFound) {
		t.Errorf("expected ErrTenantNotFound, got %v", err)
	}
	if len(resetter.resetCalls) != 0 {
		t.Errorf("expected 0 reset calls, got %d", len(resetter.resetCalls))
	}
}

func testResetPasswordForbiddenNotOwner(t *testing.T) {
	repo := newMockTenantRepository()
	resetter := &mockPasswordResetter{}

	tenant := domain.Tenant{
		ID:          "tenant-uuid-1",
		Name:        "Acme",
		Slug:        "acme",
		OwnerUserID: "owner-real-uuid",
		Status:      domain.TenantStatusActive,
	}
	_ = repo.Save(context.Background(), tenant)

	uc := usecase.NewResetTenantPasswordUseCase(repo, resetter)
	_, err := uc.Execute(context.Background(), usecase.ResetTenantPasswordInput{
		TenantID:        "tenant-uuid-1",
		RequesterUserID: "impostor-user-uuid",
	})
	if !errors.Is(err, domain.ErrForbiddenTenantAccess) {
		t.Errorf("expected ErrForbiddenTenantAccess, got %v", err)
	}
	if len(resetter.resetCalls) != 0 {
		t.Errorf("expected 0 reset calls on forbidden, got %d", len(resetter.resetCalls))
	}
}

func testResetPasswordProvisionerFailure(t *testing.T) {
	repo := newMockTenantRepository()
	provErr := errors.New("postgres connection refused")
	resetter := &mockPasswordResetter{resetErr: provErr}

	tenant := domain.Tenant{
		ID:          "tenant-uuid-1",
		Name:        "Acme",
		Slug:        "acme",
		OwnerUserID: "owner-uuid-1",
		Status:      domain.TenantStatusActive,
	}
	_ = repo.Save(context.Background(), tenant)

	uc := usecase.NewResetTenantPasswordUseCase(repo, resetter)
	_, err := uc.Execute(context.Background(), usecase.ResetTenantPasswordInput{
		TenantID:        "tenant-uuid-1",
		RequesterUserID: "owner-uuid-1",
	})
	if err == nil {
		t.Fatal("expected error on provisioner reset failure, got nil")
	}
}

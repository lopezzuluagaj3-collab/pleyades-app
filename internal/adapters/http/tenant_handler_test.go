package httpadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pleyades-app/internal/adapters/http/middleware"
	"pleyades-app/internal/domain"
	"pleyades-app/internal/ports"
	"pleyades-app/internal/usecase"
)

type mockTenantRepoForHTTP struct {
	saveFunc        func(ctx context.Context, tenant domain.Tenant) error
	findBySlugFunc  func(ctx context.Context, slug string) (*domain.Tenant, error)
	findByIDFunc    func(ctx context.Context, id string) (*domain.Tenant, error)
	findByOwnerFunc func(ctx context.Context, ownerUserID string) ([]domain.Tenant, error)
}

func (m *mockTenantRepoForHTTP) Save(ctx context.Context, tenant domain.Tenant) error {
	if m.saveFunc != nil {
		return m.saveFunc(ctx, tenant)
	}
	return nil
}

func (m *mockTenantRepoForHTTP) FindBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	if m.findBySlugFunc != nil {
		return m.findBySlugFunc(ctx, slug)
	}
	return nil, domain.ErrTenantNotFound
}

func (m *mockTenantRepoForHTTP) FindByID(ctx context.Context, id string) (*domain.Tenant, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, domain.ErrTenantNotFound
}

func (m *mockTenantRepoForHTTP) FindByOwnerUserID(ctx context.Context, ownerUserID string) ([]domain.Tenant, error) {
	if m.findByOwnerFunc != nil {
		return m.findByOwnerFunc(ctx, ownerUserID)
	}
	return nil, nil
}

type mockTenantProvisionerForHTTP struct {
	provisionFunc func(ctx context.Context, slug string) (*ports.TenantDBCredentials, error)
	resetFunc     func(ctx context.Context, slug string) (*ports.TenantDBCredentials, error)
}

func (m *mockTenantProvisionerForHTTP) Provision(ctx context.Context, slug string) (*ports.TenantDBCredentials, error) {
	if m.provisionFunc != nil {
		return m.provisionFunc(ctx, slug)
	}
	creds := &ports.TenantDBCredentials{
		Host:     "localhost",
		Port:     5432,
		DBName:   "tenant_acme",
		Username: "usr_acme",
		Password: "mock-password",
	}
	creds.ConnectionString = creds.BuildConnectionString("disable")
	return creds, nil
}

func (m *mockTenantProvisionerForHTTP) ResetPassword(ctx context.Context, slug string) (*ports.TenantDBCredentials, error) {
	if m.resetFunc != nil {
		return m.resetFunc(ctx, slug)
	}
	creds := &ports.TenantDBCredentials{
		Host:     "localhost",
		Port:     5432,
		DBName:   "tenant_acme",
		Username: "usr_acme",
		Password: "new-rotated-password",
	}
	creds.ConnectionString = creds.BuildConnectionString("disable")
	return creds, nil
}

func TestTenantHandlerSuite(t *testing.T) {
	t.Run("Create Tenant Success", testCreateTenantSuccess)
	t.Run("Create Tenant Unauthorized", testCreateTenantUnauthorized)
	t.Run("Create Tenant Malformed JSON", testCreateTenantMalformedJSON)
	t.Run("Create Tenant Validation Error", testCreateTenantValidationError)
	t.Run("Create Tenant Slug Conflict", testCreateTenantSlugConflict)
	t.Run("Create Tenant Internal Server Error", testCreateTenantInternalError)
	t.Run("Reset Password Success", testResetPasswordSuccess)
	t.Run("Reset Password Unauthorized", testResetPasswordUnauthorized)
	t.Run("Reset Password Forbidden Not Owner", testResetPasswordForbidden)
	t.Run("Reset Password Not Found", testResetPasswordNotFound)
}

func testCreateTenantSuccess(t *testing.T) {
	repo := &mockTenantRepoForHTTP{}
	prov := &mockTenantProvisionerForHTTP{}
	createUC := usecase.NewCreateTenantUseCase(repo, prov)
	resetUC := usecase.NewResetTenantPasswordUseCase(repo, prov)
	handler := NewTenantHandler(createUC, resetUC)

	body := `{"name":"Acme Global","slug":"acme-global"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	claims := &ports.UserClaims{
		UUID:      "user-uuid-1234",
		Email:     "admin@acme.com",
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	req = req.WithContext(middleware.ContextWithUser(req.Context(), claims))

	rec := httptest.NewRecorder()
	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var resp TenantResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Database == nil {
		t.Fatal("expected database credentials in response")
	}
	if resp.Database.Password != "mock-password" {
		t.Errorf("expected Password 'mock-password', got '%s'", resp.Database.Password)
	}
	if resp.Database.ConnectionString == "" {
		t.Error("expected non-empty ConnectionString")
	}
}

func testCreateTenantUnauthorized(t *testing.T) {
	repo := &mockTenantRepoForHTTP{}
	prov := &mockTenantProvisionerForHTTP{}
	createUC := usecase.NewCreateTenantUseCase(repo, prov)
	resetUC := usecase.NewResetTenantPasswordUseCase(repo, prov)
	handler := NewTenantHandler(createUC, resetUC)

	body := `{"name":"Acme Global","slug":"acme-global"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.Create(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized, got %d", rec.Code)
	}
}

func testCreateTenantMalformedJSON(t *testing.T) {
	repo := &mockTenantRepoForHTTP{}
	prov := &mockTenantProvisionerForHTTP{}
	createUC := usecase.NewCreateTenantUseCase(repo, prov)
	resetUC := usecase.NewResetTenantPasswordUseCase(repo, prov)
	handler := NewTenantHandler(createUC, resetUC)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(`{invalid-json`))
	req.Header.Set("Content-Type", "application/json")

	claims := &ports.UserClaims{UUID: "user-1"}
	req = req.WithContext(middleware.ContextWithUser(req.Context(), claims))

	rec := httptest.NewRecorder()
	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d", rec.Code)
	}
}

func testCreateTenantValidationError(t *testing.T) {
	repo := &mockTenantRepoForHTTP{}
	prov := &mockTenantProvisionerForHTTP{}
	createUC := usecase.NewCreateTenantUseCase(repo, prov)
	resetUC := usecase.NewResetTenantPasswordUseCase(repo, prov)
	handler := NewTenantHandler(createUC, resetUC)

	body := `{"name":"","slug":"acme"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	claims := &ports.UserClaims{UUID: "user-1"}
	req = req.WithContext(middleware.ContextWithUser(req.Context(), claims))

	rec := httptest.NewRecorder()
	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d", rec.Code)
	}
}

func testCreateTenantSlugConflict(t *testing.T) {
	repo := &mockTenantRepoForHTTP{
		findBySlugFunc: func(ctx context.Context, slug string) (*domain.Tenant, error) {
			return &domain.Tenant{Slug: slug}, nil
		},
	}
	prov := &mockTenantProvisionerForHTTP{}
	createUC := usecase.NewCreateTenantUseCase(repo, prov)
	resetUC := usecase.NewResetTenantPasswordUseCase(repo, prov)
	handler := NewTenantHandler(createUC, resetUC)

	body := `{"name":"Acme Global","slug":"acme-global"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	claims := &ports.UserClaims{UUID: "user-1"}
	req = req.WithContext(middleware.ContextWithUser(req.Context(), claims))

	rec := httptest.NewRecorder()
	handler.Create(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status 409 Conflict, got %d", rec.Code)
	}
}

func testCreateTenantInternalError(t *testing.T) {
	repo := &mockTenantRepoForHTTP{}
	prov := &mockTenantProvisionerForHTTP{
		provisionFunc: func(ctx context.Context, slug string) (*ports.TenantDBCredentials, error) {
			return nil, errors.New("infra quota failure")
		},
	}
	createUC := usecase.NewCreateTenantUseCase(repo, prov)
	resetUC := usecase.NewResetTenantPasswordUseCase(repo, prov)
	handler := NewTenantHandler(createUC, resetUC)

	body := `{"name":"Acme Global","slug":"acme-global"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	claims := &ports.UserClaims{UUID: "user-1"}
	req = req.WithContext(middleware.ContextWithUser(req.Context(), claims))

	rec := httptest.NewRecorder()
	handler.Create(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 Internal Server Error, got %d", rec.Code)
	}
}

func testResetPasswordSuccess(t *testing.T) {
	repo := &mockTenantRepoForHTTP{
		findByIDFunc: func(ctx context.Context, id string) (*domain.Tenant, error) {
			return &domain.Tenant{
				ID:          id,
				Slug:        "acme-global",
				OwnerUserID: "user-owner-1",
			}, nil
		},
	}
	prov := &mockTenantProvisionerForHTTP{}
	createUC := usecase.NewCreateTenantUseCase(repo, prov)
	resetUC := usecase.NewResetTenantPasswordUseCase(repo, prov)
	handler := NewTenantHandler(createUC, resetUC)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/t-123/reset-password", nil)
	req.SetPathValue("id", "t-123")
	claims := &ports.UserClaims{UUID: "user-owner-1"}
	req = req.WithContext(middleware.ContextWithUser(req.Context(), claims))

	rec := httptest.NewRecorder()
	handler.ResetPassword(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var resp ResetPasswordResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Database == nil || resp.Database.Password != "new-rotated-password" {
		t.Errorf("expected new password in response, got %v", resp.Database)
	}
}

func testResetPasswordUnauthorized(t *testing.T) {
	repo := &mockTenantRepoForHTTP{}
	prov := &mockTenantProvisionerForHTTP{}
	createUC := usecase.NewCreateTenantUseCase(repo, prov)
	resetUC := usecase.NewResetTenantPasswordUseCase(repo, prov)
	handler := NewTenantHandler(createUC, resetUC)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/t-123/reset-password", nil)
	rec := httptest.NewRecorder()
	handler.ResetPassword(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized, got %d", rec.Code)
	}
}

func testResetPasswordForbidden(t *testing.T) {
	repo := &mockTenantRepoForHTTP{
		findByIDFunc: func(ctx context.Context, id string) (*domain.Tenant, error) {
			return &domain.Tenant{
				ID:          id,
				OwnerUserID: "user-real-owner",
			}, nil
		},
	}
	prov := &mockTenantProvisionerForHTTP{}
	createUC := usecase.NewCreateTenantUseCase(repo, prov)
	resetUC := usecase.NewResetTenantPasswordUseCase(repo, prov)
	handler := NewTenantHandler(createUC, resetUC)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/t-123/reset-password", nil)
	req.SetPathValue("id", "t-123")
	claims := &ports.UserClaims{UUID: "user-impostor"}
	req = req.WithContext(middleware.ContextWithUser(req.Context(), claims))

	rec := httptest.NewRecorder()
	handler.ResetPassword(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 Forbidden, got %d", rec.Code)
	}
}

func testResetPasswordNotFound(t *testing.T) {
	repo := &mockTenantRepoForHTTP{
		findByIDFunc: func(ctx context.Context, id string) (*domain.Tenant, error) {
			return nil, domain.ErrTenantNotFound
		},
	}
	prov := &mockTenantProvisionerForHTTP{}
	createUC := usecase.NewCreateTenantUseCase(repo, prov)
	resetUC := usecase.NewResetTenantPasswordUseCase(repo, prov)
	handler := NewTenantHandler(createUC, resetUC)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/t-123/reset-password", nil)
	req.SetPathValue("id", "t-123")
	claims := &ports.UserClaims{UUID: "user-owner"}
	req = req.WithContext(middleware.ContextWithUser(req.Context(), claims))

	rec := httptest.NewRecorder()
	handler.ResetPassword(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 Not Found, got %d", rec.Code)
	}
}

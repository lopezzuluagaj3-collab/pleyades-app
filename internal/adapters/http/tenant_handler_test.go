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
}

func (m *mockTenantProvisionerForHTTP) Provision(ctx context.Context, slug string) (*ports.TenantDBCredentials, error) {
	if m.provisionFunc != nil {
		return m.provisionFunc(ctx, slug)
	}
	return &ports.TenantDBCredentials{
		Host:     "localhost",
		Port:     5432,
		DBName:   "tenant_acme",
		Username: "usr_acme",
		Password: "password",
	}, nil
}

func TestTenantHandlerSuite(t *testing.T) {
	t.Run("Create Tenant Success", testCreateTenantSuccess)
	t.Run("Create Tenant Unauthorized", testCreateTenantUnauthorized)
	t.Run("Create Tenant Malformed JSON", testCreateTenantMalformedJSON)
	t.Run("Create Tenant Validation Error", testCreateTenantValidationError)
	t.Run("Create Tenant Slug Conflict", testCreateTenantSlugConflict)
	t.Run("Create Tenant Internal Server Error", testCreateTenantInternalError)
}

func testCreateTenantSuccess(t *testing.T) {
	repo := &mockTenantRepoForHTTP{}
	prov := &mockTenantProvisionerForHTTP{}
	uc := usecase.NewCreateTenantUseCase(repo, prov)
	handler := NewTenantHandler(uc)

	body := `{"name":"Acme Global","slug":"acme-global"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	// Simular contexto inyectado por authMiddleware
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

	if resp.Name != "Acme Global" {
		t.Errorf("expected Name 'Acme Global', got '%s'", resp.Name)
	}
	if resp.Slug != "acme-global" {
		t.Errorf("expected Slug 'acme-global', got '%s'", resp.Slug)
	}
	if resp.OwnerUserID != "user-uuid-1234" {
		t.Errorf("expected OwnerUserID 'user-uuid-1234', got '%s'", resp.OwnerUserID)
	}
	if resp.Status != domain.TenantStatusActive {
		t.Errorf("expected Status 'ACTIVE', got '%s'", resp.Status)
	}
	if resp.DBName != "tenant_acme_global" {
		t.Errorf("expected DBName 'tenant_acme_global', got '%s'", resp.DBName)
	}
}

func testCreateTenantUnauthorized(t *testing.T) {
	repo := &mockTenantRepoForHTTP{}
	prov := &mockTenantProvisionerForHTTP{}
	uc := usecase.NewCreateTenantUseCase(repo, prov)
	handler := NewTenantHandler(uc)

	body := `{"name":"Acme Global","slug":"acme-global"}`
	// Petición SIN claims de usuario en el contexto
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.Create(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "no autorizado") {
		t.Errorf("expected unauthorized error message, got %s", rec.Body.String())
	}
}

func testCreateTenantMalformedJSON(t *testing.T) {
	repo := &mockTenantRepoForHTTP{}
	prov := &mockTenantProvisionerForHTTP{}
	uc := usecase.NewCreateTenantUseCase(repo, prov)
	handler := NewTenantHandler(uc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(`{invalid-json`))
	req.Header.Set("Content-Type", "application/json")

	claims := &ports.UserClaims{UUID: "user-1"}
	req = req.WithContext(middleware.ContextWithUser(req.Context(), claims))

	rec := httptest.NewRecorder()
	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "formato de petición json inválido") {
		t.Errorf("expected malformed json error, got %s", rec.Body.String())
	}
}

func testCreateTenantValidationError(t *testing.T) {
	repo := &mockTenantRepoForHTTP{}
	prov := &mockTenantProvisionerForHTTP{}
	uc := usecase.NewCreateTenantUseCase(repo, prov)
	handler := NewTenantHandler(uc)

	// Nombre vacío -> debe fallar con 400
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

	if !strings.Contains(rec.Body.String(), "nombre del inquilino no puede estar vacío") {
		t.Errorf("expected validation error message, got %s", rec.Body.String())
	}
}

func testCreateTenantSlugConflict(t *testing.T) {
	repo := &mockTenantRepoForHTTP{
		findBySlugFunc: func(ctx context.Context, slug string) (*domain.Tenant, error) {
			return &domain.Tenant{Slug: slug}, nil
		},
	}
	prov := &mockTenantProvisionerForHTTP{}
	uc := usecase.NewCreateTenantUseCase(repo, prov)
	handler := NewTenantHandler(uc)

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

	if !strings.Contains(rec.Body.String(), "slug del inquilino ya está registrado") {
		t.Errorf("expected conflict message, got %s", rec.Body.String())
	}
}

func testCreateTenantInternalError(t *testing.T) {
	repo := &mockTenantRepoForHTTP{}
	prov := &mockTenantProvisionerForHTTP{
		provisionFunc: func(ctx context.Context, slug string) (*ports.TenantDBCredentials, error) {
			return nil, errors.New("infra quota failure")
		},
	}
	uc := usecase.NewCreateTenantUseCase(repo, prov)
	handler := NewTenantHandler(uc)

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

	if !strings.Contains(rec.Body.String(), "error interno del servidor") {
		t.Errorf("expected internal server error message, got %s", rec.Body.String())
	}
}

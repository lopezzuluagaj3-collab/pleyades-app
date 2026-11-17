package usecase_test

import (
	"context"
	"errors"
	"testing"

	"pleyades-app/internal/domain"
	"pleyades-app/internal/ports"
	"pleyades-app/internal/usecase"
)

type mockTenantRepository struct {
	tenantsBySlug  map[string]domain.Tenant
	tenantsByID    map[string]domain.Tenant
	saveErr        error
	findBySlugErr  error
	findByIDErr    error
	findByOwnerErr error
	saveCalls      int
}

func newMockTenantRepository() *mockTenantRepository {
	return &mockTenantRepository{
		tenantsBySlug: make(map[string]domain.Tenant),
		tenantsByID:   make(map[string]domain.Tenant),
	}
}

func (m *mockTenantRepository) Save(ctx context.Context, tenant domain.Tenant) error {
	m.saveCalls++
	if m.saveErr != nil {
		return m.saveErr
	}
	m.tenantsBySlug[tenant.Slug] = tenant
	m.tenantsByID[tenant.ID] = tenant
	return nil
}

func (m *mockTenantRepository) FindByID(ctx context.Context, id string) (*domain.Tenant, error) {
	if m.findByIDErr != nil {
		return nil, m.findByIDErr
	}
	t, exists := m.tenantsByID[id]
	if !exists {
		return nil, domain.ErrTenantNotFound
	}
	return &t, nil
}

func (m *mockTenantRepository) FindBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	if m.findBySlugErr != nil {
		return nil, m.findBySlugErr
	}
	t, exists := m.tenantsBySlug[slug]
	if !exists {
		return nil, domain.ErrTenantNotFound
	}
	return &t, nil
}

func (m *mockTenantRepository) FindByOwnerUserID(ctx context.Context, ownerUserID string) ([]domain.Tenant, error) {
	if m.findByOwnerErr != nil {
		return nil, m.findByOwnerErr
	}
	var res []domain.Tenant
	for _, t := range m.tenantsByID {
		if t.OwnerUserID == ownerUserID {
			res = append(res, t)
		}
	}
	return res, nil
}

type mockTenantProvisioner struct {
	provisionErr    error
	provisionCalls  []string
	mockCredentials *ports.TenantDBCredentials
}

func newMockTenantProvisioner() *mockTenantProvisioner {
	return &mockTenantProvisioner{
		mockCredentials: &ports.TenantDBCredentials{
			Host:     "localhost",
			Port:     5432,
			DBName:   "tenant_acme",
			Username: "usr_acme",
			Password: "secure-password",
		},
	}
}

func (m *mockTenantProvisioner) Provision(ctx context.Context, slug string) (*ports.TenantDBCredentials, error) {
	m.provisionCalls = append(m.provisionCalls, slug)
	if m.provisionErr != nil {
		return nil, m.provisionErr
	}
	return m.mockCredentials, nil
}

func (m *mockTenantProvisioner) ResetPassword(ctx context.Context, slug string) (*ports.TenantDBCredentials, error) {
	m.provisionCalls = append(m.provisionCalls, slug)
	if m.provisionErr != nil {
		return nil, m.provisionErr
	}
	return m.mockCredentials, nil
}

func TestCreateTenantUseCaseSuite(t *testing.T) {
	t.Run("Happy Path Tenant Creation", testCreateTenantHappyPath)
	t.Run("Domain Validation Short-Circuits Before Infrastructure", testCreateTenantDomainValidation)
	t.Run("Duplicate Slug Conflict Detection", testCreateTenantSlugConflict)
	t.Run("Repository Lookup Unexpected Error Propagation", testCreateTenantRepoLookupError)
	t.Run("Provisioner Failure Prevents Repository Save", testCreateTenantProvisionerFailure)
	t.Run("Repository Save Failure Propagation", testCreateTenantRepoSaveFailure)
}

func testCreateTenantHappyPath(t *testing.T) {
	repo := newMockTenantRepository()
	prov := newMockTenantProvisioner()
	uc := usecase.NewCreateTenantUseCase(repo, prov)

	input := usecase.CreateTenantInput{
		Name:        "Acme Global",
		Slug:        "acme-global",
		OwnerUserID: "user-uuid-1234",
	}

	output, err := uc.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error on valid tenant creation: %v", err)
	}

	if output.ID == "" {
		t.Error("expected generated tenant ID, got empty string")
	}
	if output.Name != "Acme Global" {
		t.Errorf("expected Name 'Acme Global', got '%s'", output.Name)
	}
	if output.Slug != "acme-global" {
		t.Errorf("expected Slug 'acme-global', got '%s'", output.Slug)
	}
	if output.DBName != "tenant_acme_global" {
		t.Errorf("expected DBName 'tenant_acme_global', got '%s'", output.DBName)
	}
	if output.Status != domain.TenantStatusActive {
		t.Errorf("expected Status 'ACTIVE', got '%s'", output.Status)
	}
	if output.OwnerUserID != "user-uuid-1234" {
		t.Errorf("expected OwnerUserID 'user-uuid-1234', got '%s'", output.OwnerUserID)
	}
	if output.Database == nil {
		t.Fatal("expected non-nil Database credentials in output")
	}
	if output.Database.Password != "secure-password" {
		t.Errorf("expected Password 'secure-password', got '%s'", output.Database.Password)
	}
	if len(prov.provisionCalls) != 1 || prov.provisionCalls[0] != "acme-global" {
		t.Errorf("expected 1 provision call for 'acme-global', got %v", prov.provisionCalls)
	}
	if repo.saveCalls != 1 {
		t.Errorf("expected 1 repo save call, got %d", repo.saveCalls)
	}
}

func testCreateTenantDomainValidation(t *testing.T) {
	repo := newMockTenantRepository()
	prov := newMockTenantProvisioner()
	uc := usecase.NewCreateTenantUseCase(repo, prov)

	invalidInputs := []struct {
		name        string
		input       usecase.CreateTenantInput
		expectedErr error
	}{
		{
			name: "Empty name fails",
			input: usecase.CreateTenantInput{
				Name:        "",
				Slug:        "acme",
				OwnerUserID: "owner-1",
			},
			expectedErr: domain.ErrEmptyTenantName,
		},
		{
			name: "Invalid slug fails",
			input: usecase.CreateTenantInput{
				Name:        "Acme",
				Slug:        "INVALID_SLUG",
				OwnerUserID: "owner-1",
			},
			expectedErr: domain.ErrInvalidTenantSlug,
		},
		{
			name: "Empty owner ID fails",
			input: usecase.CreateTenantInput{
				Name:        "Acme",
				Slug:        "acme",
				OwnerUserID: "",
			},
			expectedErr: domain.ErrEmptyTenantOwnerID,
		},
	}

	for _, tt := range invalidInputs {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uc.Execute(context.Background(), tt.input)
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("expected error %v, got %v", tt.expectedErr, err)
			}
			if len(prov.provisionCalls) > 0 {
				t.Errorf("expected 0 provision calls, got %d", len(prov.provisionCalls))
			}
			if repo.saveCalls > 0 {
				t.Errorf("expected 0 repo save calls, got %d", repo.saveCalls)
			}
		})
	}
}

func testCreateTenantSlugConflict(t *testing.T) {
	repo := newMockTenantRepository()
	prov := newMockTenantProvisioner()
	uc := usecase.NewCreateTenantUseCase(repo, prov)

	existingTenant := domain.Tenant{
		ID:          "existing-uuid",
		Name:        "Existing Corp",
		Slug:        "acme",
		OwnerUserID: "owner-1",
		Status:      domain.TenantStatusActive,
	}
	_ = repo.Save(context.Background(), existingTenant)
	repo.saveCalls = 0

	input := usecase.CreateTenantInput{
		Name:        "New Acme",
		Slug:        "acme",
		OwnerUserID: "owner-2",
	}

	_, err := uc.Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrTenantSlugConflict) {
		t.Errorf("expected ErrTenantSlugConflict, got %v", err)
	}
	if len(prov.provisionCalls) != 0 {
		t.Errorf("expected 0 provision calls on slug conflict, got %d", len(prov.provisionCalls))
	}
	if repo.saveCalls != 0 {
		t.Errorf("expected 0 repo save calls on slug conflict, got %d", repo.saveCalls)
	}
}

func testCreateTenantRepoLookupError(t *testing.T) {
	repo := newMockTenantRepository()
	prov := newMockTenantProvisioner()
	uc := usecase.NewCreateTenantUseCase(repo, prov)

	dbErr := errors.New("connection reset by peer")
	repo.findBySlugErr = dbErr

	input := usecase.CreateTenantInput{
		Name:        "Acme Global",
		Slug:        "acme",
		OwnerUserID: "owner-1",
	}

	_, err := uc.Execute(context.Background(), input)
	if !errors.Is(err, dbErr) {
		t.Errorf("expected dbErr, got %v", err)
	}
}

func testCreateTenantProvisionerFailure(t *testing.T) {
	repo := newMockTenantRepository()
	prov := newMockTenantProvisioner()
	prov.provisionErr = errors.New("azure flexible server quota exceeded")
	uc := usecase.NewCreateTenantUseCase(repo, prov)

	input := usecase.CreateTenantInput{
		Name:        "Acme Global",
		Slug:        "acme",
		OwnerUserID: "owner-1",
	}

	_, err := uc.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error on provisioner failure, got nil")
	}
	if repo.saveCalls != 0 {
		t.Errorf("expected 0 repo save calls on provisioner failure, got %d", repo.saveCalls)
	}
}

func testCreateTenantRepoSaveFailure(t *testing.T) {
	repo := newMockTenantRepository()
	prov := newMockTenantProvisioner()
	repo.saveErr = errors.New("database disk full")
	uc := usecase.NewCreateTenantUseCase(repo, prov)

	input := usecase.CreateTenantInput{
		Name:        "Acme Global",
		Slug:        "acme",
		OwnerUserID: "owner-1",
	}

	_, err := uc.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error on repository save failure, got nil")
	}
}

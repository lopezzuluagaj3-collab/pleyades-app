package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"pleyades-app/internal/domain"
)

type mockDatabasePoolRepository struct {
	availablePool       *domain.DatabasePool
	getAvailablePoolErr error
	findByIDPool        *domain.DatabasePool
	findByIDErr         error
	saveErr             error
	incrementCalls      int
	incrementErr        error
	incrementedPoolID   string
	listAllPools        []domain.DatabasePool
	listAllErr          error
}

func (m *mockDatabasePoolRepository) GetAvailablePool(ctx context.Context) (*domain.DatabasePool, error) {
	if m.getAvailablePoolErr != nil {
		return nil, m.getAvailablePoolErr
	}
	return m.availablePool, nil
}

func (m *mockDatabasePoolRepository) FindByID(ctx context.Context, id string) (*domain.DatabasePool, error) {
	if m.findByIDErr != nil {
		return nil, m.findByIDErr
	}
	return m.findByIDPool, nil
}

func (m *mockDatabasePoolRepository) Save(ctx context.Context, pool domain.DatabasePool) error {
	return m.saveErr
}

func (m *mockDatabasePoolRepository) IncrementTenantCount(ctx context.Context, poolID string) error {
	m.incrementCalls++
	m.incrementedPoolID = poolID
	return m.incrementErr
}

func (m *mockDatabasePoolRepository) ListAll(ctx context.Context) ([]domain.DatabasePool, error) {
	if m.listAllErr != nil {
		return nil, m.listAllErr
	}
	return m.listAllPools, nil
}

type mockTenantRepoForProvisioner struct {
	tenantBySlug  *domain.Tenant
	findBySlugErr error
}

func (m *mockTenantRepoForProvisioner) Save(ctx context.Context, tenant domain.Tenant) error {
	return nil
}

func (m *mockTenantRepoForProvisioner) FindByID(ctx context.Context, id string) (*domain.Tenant, error) {
	return nil, nil
}

func (m *mockTenantRepoForProvisioner) FindBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	if m.findBySlugErr != nil {
		return nil, m.findBySlugErr
	}
	if m.tenantBySlug == nil {
		return nil, domain.ErrTenantNotFound
	}
	return m.tenantBySlug, nil
}

func (m *mockTenantRepoForProvisioner) FindByOwnerUserID(ctx context.Context, ownerUserID string) ([]domain.Tenant, error) {
	return nil, nil
}

func TestShardedProvisionerValidation(t *testing.T) {
	poolRepo := &mockDatabasePoolRepository{}
	tenantRepo := &mockTenantRepoForProvisioner{}

	prov := NewShardedPostgresTenantProvisioner(nil, "localhost", 5432, poolRepo, tenantRepo, "disable")

	t.Run("Rejects invalid slug before querying pool", func(t *testing.T) {
		_, err := prov.Provision(context.Background(), "-invalid-slug")
		if err == nil {
			t.Errorf("expected error for invalid slug '-invalid-slug', got nil")
		}
	})

	t.Run("Returns ErrNoAvailablePool when no pools have capacity", func(t *testing.T) {
		poolRepo.getAvailablePoolErr = domain.ErrNoAvailablePool
		_, err := prov.Provision(context.Background(), "valid-slug")
		if err == nil || !errors.Is(err, domain.ErrNoAvailablePool) {
			t.Errorf("expected ErrNoAvailablePool, got %v", err)
		}
	})

	t.Run("ResetPassword rejects invalid slug", func(t *testing.T) {
		_, err := prov.ResetPassword(context.Background(), "INVALID_SLUG_CAPS")
		if err == nil {
			t.Errorf("expected error for slug with uppercase letters, got nil")
		}
	})

	t.Run("ResetPassword returns error when tenant does not exist", func(t *testing.T) {
		tenantRepo.tenantBySlug = nil
		tenantRepo.findBySlugErr = domain.ErrTenantNotFound
		_, err := prov.ResetPassword(context.Background(), "non-existent")
		if err == nil {
			t.Errorf("expected error for non-existent tenant, got nil")
		}
	})
}

func TestDatabasePoolCapacityRules(t *testing.T) {
	t.Run("Bin-packing threshold: marks FULL when reaching max_tenants", func(t *testing.T) {
		pool := domain.DatabasePool{
			ID:             "pool-01",
			Name:           "psql-pleyades-dev-01",
			Host:           "psql-pleyades-dev-01.postgres.database.azure.com",
			Port:           5432,
			AdminUser:      "pleyades_admin",
			MaxTenants:     7,
			CurrentTenants: 6,
			Status:         domain.PoolStatusActive,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		if !pool.CanAcceptTenant() {
			t.Errorf("pool with 6/7 should still accept a tenant")
		}

		// Simulate 7th tenant addition
		pool.CurrentTenants++
		if pool.CurrentTenants >= pool.MaxTenants {
			pool.Status = domain.PoolStatusFull
		}

		if pool.CanAcceptTenant() {
			t.Errorf("pool with 7/7 must NOT accept another tenant")
		}
		if pool.Status != domain.PoolStatusFull {
			t.Errorf("expected status FULL, got %s", pool.Status)
		}
	})
}

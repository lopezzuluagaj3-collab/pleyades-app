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

type mockPoolScaleTrigger struct {
	triggeredServers []string
	triggerErr       error
}

func (m *mockPoolScaleTrigger) TriggerWarmPool(ctx context.Context, serverName string) error {
	m.triggeredServers = append(m.triggeredServers, serverName)
	return m.triggerErr
}

func TestShardedProvisionerValidation(t *testing.T) {
	poolRepo := &mockDatabasePoolRepository{}
	tenantRepo := &mockTenantRepoForProvisioner{}
	scaleTrigger := &mockPoolScaleTrigger{}

	prov := NewShardedPostgresTenantProvisioner(nil, "localhost", 5432, poolRepo, tenantRepo, scaleTrigger, "disable")

	t.Run("Rejects invalid slug before querying pool", func(t *testing.T) {
		_, err := prov.Provision(context.Background(), "-invalid-slug")
		if err == nil {
			t.Errorf("expected error for invalid slug '-invalid-slug', got nil")
		}
	})

	t.Run("Rejects slug shorter than 3 chars", func(t *testing.T) {
		_, err := prov.Provision(context.Background(), "ab")
		if err == nil {
			t.Errorf("expected error for short slug 'ab', got nil")
		}
	})

	t.Run("Rejects slug longer than 30 chars preventing PostgreSQL truncation collision", func(t *testing.T) {
		longSlug := "a" + strings.Repeat("b", 29) + "c"
		_, err := prov.Provision(context.Background(), longSlug)
		if err == nil {
			t.Errorf("expected error for long slug '%s', got nil", longSlug)
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

	t.Run("Warm pool scale threshold: triggers scale out when reaching threshold 5 tenants", func(t *testing.T) {
		pool := domain.DatabasePool{
			Status:         domain.PoolStatusActive,
			CurrentTenants: 4,
			MaxTenants:     7,
		}

		if pool.ShouldTriggerScaleOut() {
			t.Errorf("pool with 4 tenants should not trigger scale out yet")
		}

		pool.CurrentTenants = 5
		if !pool.ShouldTriggerScaleOut() {
			t.Errorf("pool with 5 tenants must trigger scale out")
		}
	})
}

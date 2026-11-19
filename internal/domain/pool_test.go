package domain_test

import (
	"errors"
	"testing"
	"time"

	"pleyades-app/internal/domain"
)

func TestDatabasePoolSuite(t *testing.T) {
	t.Run("CanAcceptTenant returns true when active and below max", func(t *testing.T) {
		pool := domain.DatabasePool{
			Status:         domain.PoolStatusActive,
			CurrentTenants: 3,
			MaxTenants:     7,
		}
		if !pool.CanAcceptTenant() {
			t.Errorf("expected pool with 3/7 tenants to accept tenant")
		}
	})

	t.Run("CanAcceptTenant returns false when active but full", func(t *testing.T) {
		pool := domain.DatabasePool{
			Status:         domain.PoolStatusActive,
			CurrentTenants: 7,
			MaxTenants:     7,
		}
		if pool.CanAcceptTenant() {
			t.Errorf("expected pool with 7/7 tenants not to accept tenant")
		}
	})

	t.Run("CanAcceptTenant returns false when status is not active", func(t *testing.T) {
		pool := domain.DatabasePool{
			Status:         domain.PoolStatusMaintenance,
			CurrentTenants: 1,
			MaxTenants:     7,
		}
		if pool.CanAcceptTenant() {
			t.Errorf("expected pool in MAINTENANCE not to accept tenant")
		}
	})

	t.Run("Validate returns error on empty name", func(t *testing.T) {
		pool := domain.DatabasePool{
			Name:       "",
			Host:       "psql-01.postgres.database.azure.com",
			Port:       5432,
			MaxTenants: 7,
		}
		if err := pool.Validate(); !errors.Is(err, domain.ErrInvalidPoolName) {
			t.Errorf("expected ErrInvalidPoolName, got %v", err)
		}
	})

	t.Run("Validate returns error on empty host", func(t *testing.T) {
		pool := domain.DatabasePool{
			Name:       "pool-01",
			Host:       "   ",
			Port:       5432,
			MaxTenants: 7,
		}
		if err := pool.Validate(); !errors.Is(err, domain.ErrInvalidPoolHost) {
			t.Errorf("expected ErrInvalidPoolHost, got %v", err)
		}
	})

	t.Run("Validate returns error on invalid port", func(t *testing.T) {
		pool := domain.DatabasePool{
			Name:       "pool-01",
			Host:       "psql-01.postgres.database.azure.com",
			Port:       70000,
			MaxTenants: 7,
		}
		if err := pool.Validate(); err == nil {
			t.Errorf("expected error on port 70000, got nil")
		}
	})

	t.Run("Validate passes on valid pool", func(t *testing.T) {
		pool := domain.DatabasePool{
			ID:             "uuid-pool-1",
			Name:           "psql-pleyades-dev-01",
			Host:           "psql-pleyades-dev-01.postgres.database.azure.com",
			Port:           5432,
			AdminUser:      "pleyades_admin",
			MaxTenants:     7,
			CurrentTenants: 1,
			Status:         domain.PoolStatusActive,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if err := pool.Validate(); err != nil {
			t.Errorf("expected nil error on valid pool, got %v", err)
		}
	})
}

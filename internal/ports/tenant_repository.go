package ports

import (
	"context"
	"pleyades-app/internal/domain"
)

// TenantSaver define la operación atómica de persistencia para inquilinos
type TenantSaver interface {
	Save(ctx context.Context, tenant domain.Tenant) error
}

// TenantFinder define las operaciones de consulta y lectura de inquilinos
type TenantFinder interface {
	FindByID(ctx context.Context, id string) (*domain.Tenant, error)
	FindBySlug(ctx context.Context, slug string) (*domain.Tenant, error)
	FindByOwnerUserID(ctx context.Context, ownerUserID string) ([]domain.Tenant, error)
}

// TenantRepository compone las interfaces segregadas de persistencia para inquilinos (ISP)
type TenantRepository interface {
	TenantSaver
	TenantFinder
}

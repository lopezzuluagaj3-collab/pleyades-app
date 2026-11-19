package ports

import (
	"context"
	"pleyades-app/internal/domain"
)

// DatabasePoolRepository define las operaciones de persistencia e inventario para servidores de bases de datos
type DatabasePoolRepository interface {
	GetAvailablePool(ctx context.Context) (*domain.DatabasePool, error)
	FindByID(ctx context.Context, id string) (*domain.DatabasePool, error)
	Save(ctx context.Context, pool domain.DatabasePool) error
	IncrementTenantCount(ctx context.Context, poolID string) error
	ListAll(ctx context.Context) ([]domain.DatabasePool, error)
}

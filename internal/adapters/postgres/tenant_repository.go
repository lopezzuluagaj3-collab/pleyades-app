package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"pleyades-app/internal/domain"
)

// TenantRepository implementa el puerto ports.TenantRepository para PostgreSQL
type TenantRepository struct {
	db *sql.DB
}

// NewTenantRepository inicializa el repositorio inyectando el pool de conexiones
func NewTenantRepository(db *sql.DB) *TenantRepository {
	return &TenantRepository{
		db: db,
	}
}

// InitSchema garantiza que la tabla de tenants e índices existan en la base de datos principal
func (r *TenantRepository) InitSchema(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS tenants (
			id UUID PRIMARY KEY,
			name VARCHAR(50) NOT NULL,
			slug VARCHAR(30) UNIQUE NOT NULL,
			owner_user_id UUID NOT NULL,
			status VARCHAR(20) NOT NULL,
			db_name VARCHAR(63) NOT NULL,
			pool_id UUID,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);
		ALTER TABLE tenants ADD COLUMN IF NOT EXISTS pool_id UUID;
		CREATE INDEX IF NOT EXISTS idx_tenants_slug ON tenants(slug);
		CREATE INDEX IF NOT EXISTS idx_tenants_owner ON tenants(owner_user_id);
		CREATE INDEX IF NOT EXISTS idx_tenants_pool ON tenants(pool_id);
	`
	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("postgres: error al inicializar esquema de tenants: %w", err)
	}
	return nil
}

// Save inserta un nuevo inquilino en la base de datos de forma atómica
func (r *TenantRepository) Save(ctx context.Context, tenant domain.Tenant) error {
	query := `
		INSERT INTO tenants (id, name, slug, owner_user_id, status, db_name, pool_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	var poolID sql.NullString
	if tenant.PoolID != "" {
		poolID = sql.NullString{String: tenant.PoolID, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query,
		tenant.ID,
		tenant.Name,
		tenant.Slug,
		tenant.OwnerUserID,
		tenant.Status,
		tenant.DBName,
		poolID,
		tenant.CreatedAt,
		tenant.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: error al guardar tenant: %w", err)
	}
	return nil
}

// FindByID busca un inquilino por su UUID identificador
func (r *TenantRepository) FindByID(ctx context.Context, id string) (*domain.Tenant, error) {
	query := `
		SELECT id, name, slug, owner_user_id, status, db_name, COALESCE(pool_id::text, ''), created_at, updated_at
		FROM tenants
		WHERE id = $1
	`
	var t domain.Tenant
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&t.ID,
		&t.Name,
		&t.Slug,
		&t.OwnerUserID,
		&t.Status,
		&t.DBName,
		&t.PoolID,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrTenantNotFound
		}
		return nil, fmt.Errorf("postgres: error al buscar tenant por id: %w", err)
	}
	return &t, nil
}

// FindBySlug busca un inquilino por su slug único
func (r *TenantRepository) FindBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	query := `
		SELECT id, name, slug, owner_user_id, status, db_name, COALESCE(pool_id::text, ''), created_at, updated_at
		FROM tenants
		WHERE slug = $1
	`
	var t domain.Tenant
	err := r.db.QueryRowContext(ctx, query, slug).Scan(
		&t.ID,
		&t.Name,
		&t.Slug,
		&t.OwnerUserID,
		&t.Status,
		&t.DBName,
		&t.PoolID,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrTenantNotFound
		}
		return nil, fmt.Errorf("postgres: error al buscar tenant por slug: %w", err)
	}
	return &t, nil
}

// FindByOwnerUserID retorna todos los inquilinos pertenecientes a un usuario
func (r *TenantRepository) FindByOwnerUserID(ctx context.Context, ownerUserID string) ([]domain.Tenant, error) {
	query := `
		SELECT id, name, slug, owner_user_id, status, db_name, COALESCE(pool_id::text, ''), created_at, updated_at
		FROM tenants
		WHERE owner_user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("postgres: error al consultar tenants por propietario: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var tenants []domain.Tenant
	for rows.Next() {
		var t domain.Tenant
		if err := rows.Scan(
			&t.ID,
			&t.Name,
			&t.Slug,
			&t.OwnerUserID,
			&t.Status,
			&t.DBName,
			&t.PoolID,
			&t.CreatedAt,
			&t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres: error al escanear fila de tenant: %w", err)
		}
		tenants = append(tenants, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: error en iteración de tenants: %w", err)
	}

	return tenants, nil
}

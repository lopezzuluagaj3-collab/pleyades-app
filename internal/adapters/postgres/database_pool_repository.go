package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"pleyades-app/internal/domain"
	"pleyades-app/internal/ports"
)

var _ ports.DatabasePoolRepository = (*DatabasePoolRepository)(nil)

// DatabasePoolRepository gestiona el catálogo de servidores ADBFP en PostgreSQL
type DatabasePoolRepository struct {
	db *sql.DB
}

// NewDatabasePoolRepository instancia el repositorio inyectando el pool de conexiones
func NewDatabasePoolRepository(db *sql.DB) *DatabasePoolRepository {
	return &DatabasePoolRepository{db: db}
}

// InitSchema garantiza que la tabla database_pools y sus índices existan
func (r *DatabasePoolRepository) InitSchema(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS database_pools (
			id UUID PRIMARY KEY,
			name VARCHAR(50) NOT NULL,
			host VARCHAR(255) NOT NULL,
			port INT NOT NULL DEFAULT 5432,
			admin_user VARCHAR(50) NOT NULL,
			admin_password VARCHAR(255) NOT NULL,
			max_tenants INT NOT NULL DEFAULT 7,
			current_tenants INT NOT NULL DEFAULT 0,
			status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_database_pools_status ON database_pools(status);
	`
	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("postgres: error al inicializar esquema de database_pools: %w", err)
	}
	return nil
}

// EnsureDefaultPool registra el pool inicial en el catálogo si aún no existen servidores
func (r *DatabasePoolRepository) EnsureDefaultPool(ctx context.Context, defaultPool domain.DatabasePool) error {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM database_pools").Scan(&count)
	if err != nil {
		return fmt.Errorf("postgres: error al verificar database_pools existentes: %w", err)
	}

	if count > 0 {
		return nil
	}

	// Contar inquilinos preexistentes para reflejar la capacidad exacta
	var existingTenants int
	_ = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tenants").Scan(&existingTenants)

	poolID := defaultPool.ID
	if poolID == "" {
		poolID = generatePoolUUID()
	}

	maxTenants := defaultPool.MaxTenants
	if maxTenants <= 0 {
		maxTenants = domain.DefaultMaxTenantsPerPool
	}

	status := domain.PoolStatusActive
	if existingTenants >= maxTenants {
		status = domain.PoolStatusFull
	}

	now := time.Now()
	query := `
		INSERT INTO database_pools (
			id, name, host, port, admin_user, admin_password, max_tenants, current_tenants, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err = r.db.ExecContext(ctx, query,
		poolID,
		defaultPool.Name,
		defaultPool.Host,
		defaultPool.Port,
		defaultPool.AdminUser,
		defaultPool.AdminPassword,
		maxTenants,
		existingTenants,
		status,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("postgres: error al registrar pool base: %w", err)
	}

	// Asociar inquilinos preexistentes que no tengan pool_id asignado
	_, _ = r.db.ExecContext(ctx, "UPDATE tenants SET pool_id = $1 WHERE pool_id IS NULL", poolID)

	return nil
}

// GetAvailablePool busca el servidor activo con menor carga y capacidad disponible
func (r *DatabasePoolRepository) GetAvailablePool(ctx context.Context) (*domain.DatabasePool, error) {
	query := `
		SELECT id, name, host, port, admin_user, admin_password, max_tenants, current_tenants, status, created_at, updated_at
		FROM database_pools
		WHERE status = 'ACTIVE' AND current_tenants < max_tenants
		ORDER BY current_tenants ASC, created_at ASC
		LIMIT 1
	`
	var p domain.DatabasePool
	err := r.db.QueryRowContext(ctx, query).Scan(
		&p.ID,
		&p.Name,
		&p.Host,
		&p.Port,
		&p.AdminUser,
		&p.AdminPassword,
		&p.MaxTenants,
		&p.CurrentTenants,
		&p.Status,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNoAvailablePool
		}
		return nil, fmt.Errorf("postgres: error al buscar pool disponible: %w", err)
	}
	return &p, nil
}

// FindByID busca un pool por su identificador único
func (r *DatabasePoolRepository) FindByID(ctx context.Context, id string) (*domain.DatabasePool, error) {
	query := `
		SELECT id, name, host, port, admin_user, admin_password, max_tenants, current_tenants, status, created_at, updated_at
		FROM database_pools
		WHERE id = $1
	`
	var p domain.DatabasePool
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID,
		&p.Name,
		&p.Host,
		&p.Port,
		&p.AdminUser,
		&p.AdminPassword,
		&p.MaxTenants,
		&p.CurrentTenants,
		&p.Status,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrPoolNotFound
		}
		return nil, fmt.Errorf("postgres: error al buscar pool por id: %w", err)
	}
	return &p, nil
}

// Save inserta o actualiza un pool en el catálogo
func (r *DatabasePoolRepository) Save(ctx context.Context, pool domain.DatabasePool) error {
	query := `
		INSERT INTO database_pools (
			id, name, host, port, admin_user, admin_password, max_tenants, current_tenants, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			host = EXCLUDED.host,
			port = EXCLUDED.port,
			admin_user = EXCLUDED.admin_user,
			admin_password = EXCLUDED.admin_password,
			max_tenants = EXCLUDED.max_tenants,
			current_tenants = EXCLUDED.current_tenants,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at
	`
	_, err := r.db.ExecContext(ctx, query,
		pool.ID,
		pool.Name,
		pool.Host,
		pool.Port,
		pool.AdminUser,
		pool.AdminPassword,
		pool.MaxTenants,
		pool.CurrentTenants,
		pool.Status,
		pool.CreatedAt,
		pool.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: error al guardar database_pool: %w", err)
	}
	return nil
}

// IncrementTenantCount incrementa la ocupación y marca FULL si se alcanza la capacidad máxima
func (r *DatabasePoolRepository) IncrementTenantCount(ctx context.Context, poolID string) error {
	query := `
		UPDATE database_pools
		SET current_tenants = current_tenants + 1,
			status = CASE WHEN current_tenants + 1 >= max_tenants THEN 'FULL' ELSE status END,
			updated_at = NOW()
		WHERE id = $1
	`
	res, err := r.db.ExecContext(ctx, query, poolID)
	if err != nil {
		return fmt.Errorf("postgres: error al incrementar inquilinos en pool: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrPoolNotFound
	}
	return nil
}

// ListAll retorna el listado de todos los servidores registrados en el catálogo
func (r *DatabasePoolRepository) ListAll(ctx context.Context) ([]domain.DatabasePool, error) {
	query := `
		SELECT id, name, host, port, admin_user, admin_password, max_tenants, current_tenants, status, created_at, updated_at
		FROM database_pools
		ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("postgres: error al consultar pools: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var pools []domain.DatabasePool
	for rows.Next() {
		var p domain.DatabasePool
		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Host,
			&p.Port,
			&p.AdminUser,
			&p.AdminPassword,
			&p.MaxTenants,
			&p.CurrentTenants,
			&p.Status,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres: error al escanear fila de pool: %w", err)
		}
		pools = append(pools, p)
	}

	return pools, nil
}

func generatePoolUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}

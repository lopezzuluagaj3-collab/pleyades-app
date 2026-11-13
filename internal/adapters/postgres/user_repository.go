package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"pleyades-app/internal/domain"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) Save(ctx context.Context, user domain.User) error {
	query := `
		INSERT INTO users (uuid, email, password, name, tenant_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.ExecContext(ctx, query,
		user.UUID,
		user.Email,
		user.Password,
		user.Name,
		user.TenantID,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: error al guardar usuario: %w", err)
	}

	return nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT uuid, email, password, name, COALESCE(tenant_id, ''), created_at, updated_at
		FROM users
		WHERE email = $1
	`
	var u domain.User
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&u.UUID,
		&u.Email,
		&u.Password,
		&u.Name,
		&u.TenantID,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("postgres: error al buscar usuario por email: %w", err)
	}

	return &u, nil
}

func (r *UserRepository) Delete(ctx context.Context, email string) error{
	query := `
	DELETE FROM users where email = $1;
	`
	if _, err := r.db.ExecContext(ctx, query, email); err != nil{
		return fmt.Errorf("postgres: error al eliminar usuario: %w", err)
	}
	return nil
}
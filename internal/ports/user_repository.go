package ports

import (
	"context"
	"pleyades-app/internal/domain"
)

type UserSave interface {
	Save(ctx context.Context, user domain.User) error
}

type UserFinder interface {
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
}

type UserDelete interface {
	Delete(ctx context.Context, email string) error
}

type UserRepository interface {
	UserSave
	UserFinder
	UserDelete
}

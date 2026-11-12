package usecase

import (
	"context"
	"time"

	"pleyades-app/internal/domain"
	"pleyades-app/internal/ports"
)

type RegisterUserInput struct {
	UUID     string
	Email    string
	Password string
	Name     string
}

type RegisterUserUseCase struct {
	userRepo ports.UserRepository
}

func NewRegisterUserUseCase(userRepo ports.UserRepository) *RegisterUserUseCase {
	return &RegisterUserUseCase{
		userRepo: userRepo,
	}
}

func (uc *RegisterUserUseCase) Execute(ctx context.Context, input RegisterUserInput) (*domain.User, error) {

	user := domain.User{
		UUID:      input.UUID,
		Email:     input.Email,
		Name:      input.Name,
		Password:  input.Password,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := user.Validate(); err != nil {
		return nil, err
	}

	existingUser, _ := uc.userRepo.FindByEmail(ctx, user.Email)
	if existingUser != nil {
		return nil, domain.ErrEmailAlreadyExists
	}

	if err := uc.userRepo.Save(ctx, user); err != nil {
		return nil, err
	}

	return &user, nil
}

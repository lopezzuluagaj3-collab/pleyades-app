package usecase

import (
	"context"
	"strings"
	"time"

	"pleyades-app/internal/domain"
	"pleyades-app/internal/ports"
)

type LoginUserInput struct {
	Email    string
	Password string
}

type LoginUserOutput struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresIn   int          `json:"expires_in"`
	User        *domain.User `json:"-"`
}

// dummyBcryptHash es un hash precomputado válido (cost=10) para mitigar timing attacks cuando el usuario no existe
const dummyBcryptHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// LoginUserUseCase coordina la autenticación del usuario y la emisión del token JWT
type LoginUserUseCase struct {
	userRepo     ports.UserRepository
	hasher       ports.PasswordHasher
	tokenService ports.TokenService
	sessionTTL   time.Duration
}

func NewLoginUserUseCase(
	repo ports.UserRepository,
	hasher ports.PasswordHasher,
	tokenService ports.TokenService,
	sessionTTL time.Duration,
) *LoginUserUseCase {
	return &LoginUserUseCase{
		userRepo:     repo,
		hasher:       hasher,
		tokenService: tokenService,
		sessionTTL:   sessionTTL,
	}
}

// Execute valida credenciales y genera el token de sesión (Cumple SRP)
func (uc *LoginUserUseCase) Execute(ctx context.Context, input LoginUserInput) (*LoginUserOutput, error) {
	email := strings.TrimSpace(input.Email)
	password := input.Password

	// Validación básica de presencia para evitar enumeración y consultas innecesarias a la DB
	if email == "" || !strings.Contains(email, "@") || password == "" {
		return nil, domain.ErrInvalidCredentials
	}

	user, err := uc.userRepo.FindByEmail(ctx, email)
	if err != nil || user == nil {
		// Mitigación de Timing Attacks (OWASP): Ejecutamos una comparación contra un hash sintético
		// para igualar el tiempo de cómputo (~70ms de Bcrypt) al de un usuario existente con password inválido.
		_ = uc.hasher.Compare(dummyBcryptHash, password)
		return nil, domain.ErrInvalidCredentials
	}

	// Verificar contraseña contra el hash Bcrypt
	if err := uc.hasher.Compare(user.Password, password); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	// Emisión del token JWT stateless
	token, err := uc.tokenService.Generate(user.UUID, user.Email, uc.sessionTTL)
	if err != nil {
		return nil, err
	}

	return &LoginUserOutput{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int(uc.sessionTTL.Seconds()),
		User:        user,
	}, nil
}

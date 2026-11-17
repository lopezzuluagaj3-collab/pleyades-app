package ports

import (
	"errors"
	"time"
)

var (
	ErrInvalidToken = errors.New("token inválido o manipulado")
	ErrExpiredToken = errors.New("token expirado")
)

// UserClaims representa la identidad extraída y verificada de un JWT
type UserClaims struct {
	UUID      string    `json:"uuid"`
	Email     string    `json:"email"`
	IssuedAt  time.Time `json:"iat"`
	ExpiresAt time.Time `json:"exp"`
}

// TokenService define el contrato agnóstico para la emisión y validación de tokens
type TokenService interface {
	Generate(uuid, email string, ttl time.Duration) (string, error)
	Validate(tokenString string) (*UserClaims, error)
}

package crypto

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"pleyades-app/internal/ports"
)

type customClaims struct {
	UUID  string `json:"uuid"`
	Email string `json:"email"`
	jwt.RegisteredClaims
}

// JWTTokenService implementa el puerto ports.TokenService mediante HMAC-SHA256
type JWTTokenService struct {
	secret []byte
}

func NewJWTTokenService(secret string) *JWTTokenService {
	return &JWTTokenService{
		secret: []byte(secret),
	}
}

// Generate emite un JWT firmado criptográficamente con tiempo de expiración
func (s *JWTTokenService) Generate(uuid, email string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := customClaims{
		UUID:  uuid,
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("error al firmar token jwt: %w", err)
	}

	return signedToken, nil
}

// Validate verifica la autenticidad matemática de la firma y la vigencia temporal del token
func (s *JWTTokenService) Validate(tokenString string) (*ports.UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &customClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("método de firma inesperado: %v", t.Header["alg"])
		}
		return s.secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ports.ErrExpiredToken
		}
		return nil, ports.ErrInvalidToken
	}

	claims, ok := token.Claims.(*customClaims)
	if !ok || !token.Valid {
		return nil, ports.ErrInvalidToken
	}

	var iat, exp time.Time
	if claims.IssuedAt != nil {
		iat = claims.IssuedAt.Time
	}
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Time
	}

	return &ports.UserClaims{
		UUID:      claims.UUID,
		Email:     claims.Email,
		IssuedAt:  iat,
		ExpiresAt: exp,
	}, nil
}

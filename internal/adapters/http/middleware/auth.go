package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"pleyades-app/internal/ports"
)

type contextKey string

const userContextKey contextKey = "user_claims"

// ContextWithUser inyecta los claims verificados del usuario en el contexto
func ContextWithUser(ctx context.Context, claims *ports.UserClaims) context.Context {
	return context.WithValue(ctx, userContextKey, claims)
}

// UserFromContext extrae los claims del usuario desde el contexto de la petición
func UserFromContext(ctx context.Context) (*ports.UserClaims, bool) {
	claims, ok := ctx.Value(userContextKey).(*ports.UserClaims)
	return claims, ok
}

// AuthMiddleware es el interceptor que valida y renueva tokens JWT
type AuthMiddleware struct {
	tokenService     ports.TokenService
	renewalThreshold time.Duration
	sessionTTL       time.Duration
}

// NewAuthMiddleware inicializa el middleware de autenticación.
// renewalThreshold: tiempo restante mínimo para renovar el token automáticamente (Sliding Session).
// sessionTTL: duración del nuevo token al ser renovado.
func NewAuthMiddleware(ts ports.TokenService, renewalThreshold, sessionTTL time.Duration) *AuthMiddleware {
	return &AuthMiddleware{
		tokenService:     ts,
		renewalThreshold: renewalThreshold,
		sessionTTL:       sessionTTL,
	}
}

// Middleware orquesta la inspección del header Authorization, la validación y la inyección en Context
func (m *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString, err := extractBearerToken(r.Header.Get("Authorization"))
		if err != nil {
			respondAuthError(w, http.StatusUnauthorized, "cabecera de autorización ausente o inválida")
			return
		}

		claims, err := m.tokenService.Validate(tokenString)
		if err != nil {
			if errors.Is(err, ports.ErrExpiredToken) {
				respondAuthError(w, http.StatusUnauthorized, "token expirado por inactividad")
				return
			}
			respondAuthError(w, http.StatusUnauthorized, "token inválido o manipulado")
			return
		}

		// Ventana Deslizante (Sliding Session): Si le queda poco tiempo de vida, emitir token renovado
		if m.renewalThreshold > 0 && time.Until(claims.ExpiresAt) < m.renewalThreshold {
			if renewedToken, err := m.tokenService.Generate(claims.UUID, claims.Email, m.sessionTTL); err == nil {
				w.Header().Set("X-Renewed-Token", renewedToken)
			}
		}

		// Inyectar usuario verificado en el contexto de la petición
		ctx := ContextWithUser(r.Context(), claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func extractBearerToken(authHeader string) (string, error) {
	clean := strings.TrimSpace(authHeader)
	clean = strings.Trim(clean, `"'`) // Elimina comillas accidentales
	if clean == "" {
		return "", errors.New("cabecera authorization vacía")
	}

	// 1. Si viene con el prefijo "Bearer <token>" (estándar RFC 6750)
	if len(clean) > 7 && strings.EqualFold(clean[:7], "bearer ") {
		token := strings.TrimSpace(clean[7:])
		token = strings.Trim(token, `"'`)
		if token == "" {
			return "", errors.New("token bearer vacío")
		}
		return token, nil
	}

	// 2. Si el usuario pegó directamente el token JWT (muy común en Swagger UI o clientes HTTP)
	if strings.HasPrefix(clean, "eyJ") {
		return clean, nil
	}

	return "", errors.New("formato bearer inválido")
}

func respondAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

package httpadapter

import (
	"encoding/json"
	"errors"
	"net/http"

	"pleyades-app/internal/domain"
	"pleyades-app/internal/usecase"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// AuthHandler gestiona las peticiones HTTP de autenticación
type AuthHandler struct {
	loginUseCase *usecase.LoginUserUseCase
}

func NewAuthHandler(uc *usecase.LoginUserUseCase) *AuthHandler {
	return &AuthHandler{
		loginUseCase: uc,
	}
}

// Login godoc
// @Summary Iniciar sesión de usuario
// @Description Autentica las credenciales del usuario y emite un JWT Bearer Token (TTL 15 min, Sliding Session)
// @Tags Autenticación
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Credenciales de inicio de sesión"
// @Success 200 {object} LoginResponse "Token JWT emitido exitosamente"
// @Failure 400 {object} map[string]string "Formato de petición json inválido"
// @Failure 401 {object} map[string]string "Credenciales inválidas"
// @Failure 500 {object} map[string]string "Error interno del servidor"
// @Router /auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "formato de petición json inválido")
		return
	}

	output, err := h.loginUseCase.Execute(r.Context(), usecase.LoginUserInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			respondWithError(w, http.StatusUnauthorized, "credenciales inválidas")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "error interno del servidor")
		return
	}

	respondWithJSON(w, http.StatusOK, LoginResponse{
		AccessToken: output.AccessToken,
		TokenType:   output.TokenType,
		ExpiresIn:   output.ExpiresIn,
	})
}

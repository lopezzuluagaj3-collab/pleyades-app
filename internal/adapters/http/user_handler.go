package httpadapter

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"pleyades-app/internal/domain"
	"pleyades-app/internal/usecase"
)

// DTOs
type RegisterUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (r RegisterUserRequest) toInput() usecase.RegisterUserInput {
	return usecase.RegisterUserInput{
		UUID:     generateUUID(),
		Email:    r.Email,
		Password: r.Password,
		Name:     r.Name,
	}
}

type UserResponse struct {
	UUID      string    `json:"uuid"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func toUserResponse(user *domain.User) UserResponse {
	return UserResponse{
		UUID:      user.UUID,
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: user.CreatedAt,
	}
}

// Handler
type UserHandler struct {
	useCase *usecase.RegisterUserUseCase
}

func NewUserHandler(uc *usecase.RegisterUserUseCase) *UserHandler {
	return &UserHandler{
		useCase: uc,
	}
}

// Register godoc
// @Summary Registrar un nuevo usuario
// @Description Crea una cuenta de usuario con credenciales seguras protegidas con bcrypt
// @Tags Usuarios
// @Accept json
// @Produce json
// @Param request body RegisterUserRequest true "Datos de registro del usuario"
// @Success 201 {object} UserResponse "Usuario registrado exitosamente"
// @Failure 400 {object} map[string]string "Datos de entrada inválidos o JSON malformado"
// @Failure 409 {object} map[string]string "El correo electrónico ya se encuentra registrado"
// @Failure 500 {object} map[string]string "Error interno del servidor"
// @Router /users [post]
func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "formato de petición json inválido")
		return
	}

	user, err := h.useCase.Execute(r.Context(), req.toInput())
	if err != nil {
		status, msg := mapError(err)
		respondWithError(w, status, msg)
		return
	}

	respondWithJSON(w, http.StatusCreated, toUserResponse(user))
}

// Mapeo especializado de errores
func mapError(err error) (int, string) {
	if errors.Is(err, domain.ErrEmailAlreadyExists) {
		return http.StatusConflict, err.Error()
	}

	if isValidationError(err) {
		return http.StatusBadRequest, err.Error()
	}

	return http.StatusInternalServerError, "error interno del servidor"
}

func isValidationError(err error) bool {
	return errors.Is(err, domain.ErrEmptyEmail) ||
		errors.Is(err, domain.ErrArroaEmail) ||
		errors.Is(err, domain.ErrEmptyName) ||
		errors.Is(err, domain.ErrLongName) ||
		errors.Is(err, domain.ErrName) ||
		errors.Is(err, domain.ErrEmptyPass) ||
		errors.Is(err, domain.ErrValidatePass)
}

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

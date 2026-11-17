package httpadapter

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"pleyades-app/internal/adapters/http/middleware"
	"pleyades-app/internal/domain"
	"pleyades-app/internal/usecase"
)

// CreateTenantRequest define el payload para solicitar la creación de un nuevo inquilino
type CreateTenantRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// TenantResponse define la respuesta serializada al cliente
type TenantResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	OwnerUserID string    `json:"owner_user_id"`
	Status      string    `json:"status"`
	DBName      string    `json:"db_name"`
	CreatedAt   time.Time `json:"created_at"`
}

// TenantHandler orquesta las operaciones HTTP sobre la entidad Tenant
type TenantHandler struct {
	createUseCase *usecase.CreateTenantUseCase
}

// NewTenantHandler inicializa el manejador inyectando el caso de uso (DIP)
func NewTenantHandler(createUseCase *usecase.CreateTenantUseCase) *TenantHandler {
	return &TenantHandler{
		createUseCase: createUseCase,
	}
}

// Create maneja la petición POST /api/v1/tenants (Ruta protegida por JWT e Idempotency)
func (h *TenantHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok || claims == nil || claims.UUID == "" {
		respondWithError(w, http.StatusUnauthorized, "no autorizado: sesión ausente o inválida")
		return
	}

	var req CreateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "formato de petición json inválido")
		return
	}

	output, err := h.createUseCase.Execute(r.Context(), usecase.CreateTenantInput{
		Name:        req.Name,
		Slug:        req.Slug,
		OwnerUserID: claims.UUID,
	})
	if err != nil {
		status, msg := mapTenantError(err)
		respondWithError(w, status, msg)
		return
	}

	respondWithJSON(w, http.StatusCreated, TenantResponse{
		ID:          output.ID,
		Name:        output.Name,
		Slug:        output.Slug,
		OwnerUserID: output.OwnerUserID,
		Status:      output.Status,
		DBName:      output.DBName,
		CreatedAt:   output.CreatedAt,
	})
}

func mapTenantError(err error) (int, string) {
	if errors.Is(err, domain.ErrTenantSlugConflict) {
		return http.StatusConflict, err.Error()
	}

	if isTenantValidationError(err) {
		return http.StatusBadRequest, err.Error()
	}

	return http.StatusInternalServerError, "error interno del servidor"
}

func isTenantValidationError(err error) bool {
	return errors.Is(err, domain.ErrEmptyTenantName) ||
		errors.Is(err, domain.ErrLongTenantName) ||
		errors.Is(err, domain.ErrEmptyTenantSlug) ||
		errors.Is(err, domain.ErrLongTenantSlug) ||
		errors.Is(err, domain.ErrShortTenantSlug) ||
		errors.Is(err, domain.ErrInvalidTenantSlug) ||
		errors.Is(err, domain.ErrEmptyTenantOwnerID)
}

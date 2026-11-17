package httpadapter

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"pleyades-app/internal/adapters/http/middleware"
	"pleyades-app/internal/domain"
	"pleyades-app/internal/ports"
	"pleyades-app/internal/usecase"
)

// CreateTenantRequest define el payload para solicitar la creación de un nuevo inquilino
type CreateTenantRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// TenantDatabaseResponse expone de forma segura las credenciales de conexión del inquilino (Show-Once)
type TenantDatabaseResponse struct {
	Host             string `json:"host"`
	Port             int    `json:"port"`
	Name             string `json:"name"`
	User             string `json:"user"`
	Password         string `json:"password"`
	ConnectionString string `json:"connection_string"`
}

// TenantResponse define la respuesta serializada al cliente
type TenantResponse struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Slug        string                  `json:"slug"`
	OwnerUserID string                  `json:"owner_user_id"`
	Status      string                  `json:"status"`
	DBName      string                  `json:"db_name"`
	Database    *TenantDatabaseResponse `json:"database,omitempty"`
	CreatedAt   time.Time               `json:"created_at"`
}

// ResetPasswordResponse define la respuesta con la nueva contraseña generada (Show-Once)
type ResetPasswordResponse struct {
	TenantID string                  `json:"tenant_id"`
	Slug     string                  `json:"slug"`
	Database *TenantDatabaseResponse `json:"database"`
}

// TenantHandler orquesta las operaciones HTTP sobre la entidad Tenant
type TenantHandler struct {
	createUseCase *usecase.CreateTenantUseCase
	resetUseCase  *usecase.ResetTenantPasswordUseCase
}

// NewTenantHandler inicializa el manejador inyectando sus casos de uso (DIP)
func NewTenantHandler(
	createUseCase *usecase.CreateTenantUseCase,
	resetUseCase *usecase.ResetTenantPasswordUseCase,
) *TenantHandler {
	return &TenantHandler{
		createUseCase: createUseCase,
		resetUseCase:  resetUseCase,
	}
}

// Create godoc
// @Summary Crear un nuevo inquilino (Tenant)
// @Description Aprovisiona un nuevo tenant con base de datos PostgreSQL aislada y credenciales de conexión efímeras (Show-Once Pattern)
// @Tags Inquilinos
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Idempotency-Key header string false "Clave única UUID para idempotencia en reintentos de red"
// @Param request body CreateTenantRequest true "Datos para la creación del inquilino"
// @Success 201 {object} TenantResponse "Inquilino creado exitosamente con credenciales Show-Once de base de datos"
// @Failure 400 {object} map[string]string "Datos de entrada inválidos o slug con formato no permitido"
// @Failure 401 {object} map[string]string "No autorizado: sesión JWT ausente o inválida"
// @Failure 409 {object} map[string]string "El slug solicitado ya existe en el sistema"
// @Failure 500 {object} map[string]string "Error interno del servidor durante el aprovisionamiento"
// @Router /tenants [post]
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
		Database:    toDatabaseResponse(output.Database),
		CreatedAt:   output.CreatedAt,
	})
}

// ResetPassword godoc
// @Summary Rotar contraseña de base de datos del inquilino
// @Description Regenera la contraseña de PostgreSQL para el tenant indicado devolviendo las nuevas credenciales (Show-Once Pattern). Requiere ser propietario del tenant.
// @Tags Inquilinos
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID del inquilino"
// @Param Idempotency-Key header string false "Clave única UUID para idempotencia en reintentos de red"
// @Success 200 {object} ResetPasswordResponse "Contraseña rotada exitosamente con nuevas credenciales Show-Once"
// @Failure 400 {object} map[string]string "ID de inquilino requerido o inválido"
// @Failure 401 {object} map[string]string "No autorizado: sesión JWT ausente o inválida"
// @Failure 403 {object} map[string]string "Prohibido: el usuario autenticado no es el propietario del inquilino"
// @Failure 404 {object} map[string]string "Inquilino no encontrado"
// @Failure 500 {object} map[string]string "Error interno del servidor"
// @Router /tenants/{id}/reset-password [post]
func (h *TenantHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok || claims == nil || claims.UUID == "" {
		respondWithError(w, http.StatusUnauthorized, "no autorizado: sesión ausente o inválida")
		return
	}

	tenantID := r.PathValue("id")
	if tenantID == "" {
		respondWithError(w, http.StatusBadRequest, "el ID del inquilino es requerido")
		return
	}

	output, err := h.resetUseCase.Execute(r.Context(), usecase.ResetTenantPasswordInput{
		TenantID:        tenantID,
		RequesterUserID: claims.UUID,
	})
	if err != nil {
		status, msg := mapTenantError(err)
		respondWithError(w, status, msg)
		return
	}

	respondWithJSON(w, http.StatusOK, ResetPasswordResponse{
		TenantID: output.TenantID,
		Slug:     output.Slug,
		Database: toDatabaseResponse(output.Database),
	})
}

func toDatabaseResponse(creds *ports.TenantDBCredentials) *TenantDatabaseResponse {
	if creds == nil {
		return nil
	}
	return &TenantDatabaseResponse{
		Host:             creds.Host,
		Port:             creds.Port,
		Name:             creds.DBName,
		User:             creds.Username,
		Password:         creds.Password,
		ConnectionString: creds.ConnectionString,
	}
}

func mapTenantError(err error) (int, string) {
	if errors.Is(err, domain.ErrTenantNotFound) {
		return http.StatusNotFound, err.Error()
	}

	if errors.Is(err, domain.ErrForbiddenTenantAccess) {
		return http.StatusForbidden, err.Error()
	}

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
		errors.Is(err, domain.ErrEmptyTenantOwnerID) ||
		errors.Is(err, domain.ErrTenantIDRequired)
}

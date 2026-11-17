package usecase

import (
	"context"
	"fmt"
	"strings"

	"pleyades-app/internal/domain"
	"pleyades-app/internal/ports"
)

// ResetTenantPasswordInput contiene los parámetros requeridos para rotar las credenciales
type ResetTenantPasswordInput struct {
	TenantID        string
	RequesterUserID string
}

// ResetTenantPasswordOutput contiene la respuesta con la nueva contraseña generada (Show-Once)
type ResetTenantPasswordOutput struct {
	TenantID string                     `json:"tenant_id"`
	Slug     string                     `json:"slug"`
	Database *ports.TenantDBCredentials `json:"database"`
}

// ResetTenantPasswordUseCase orquesta la rotación segura de contraseñas de bases de datos de inquilinos
type ResetTenantPasswordUseCase struct {
	tenantRepo  ports.TenantRepository
	provisioner ports.TenantPasswordResetter
}

// NewResetTenantPasswordUseCase inicializa el caso de uso inyectando sus dependencias (DIP)
func NewResetTenantPasswordUseCase(
	tenantRepo ports.TenantRepository,
	provisioner ports.TenantPasswordResetter,
) *ResetTenantPasswordUseCase {
	return &ResetTenantPasswordUseCase{
		tenantRepo:  tenantRepo,
		provisioner: provisioner,
	}
}

// Execute verifica pertenencia y rota las credenciales en PostgreSQL
func (uc *ResetTenantPasswordUseCase) Execute(ctx context.Context, input ResetTenantPasswordInput) (*ResetTenantPasswordOutput, error) {
	tenantID := strings.TrimSpace(input.TenantID)
	if tenantID == "" {
		return nil, domain.ErrTenantIDRequired
	}

	requesterID := strings.TrimSpace(input.RequesterUserID)
	if requesterID == "" {
		return nil, domain.ErrEmptyTenantOwnerID
	}

	// 1. Buscar inquilino en metadatos
	tenant, err := uc.tenantRepo.FindByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// 2. Control de Acceso: Solo el propietario legítimo puede rotar la contraseña (OWASP BOLA/IDOR prevention)
	if tenant.OwnerUserID != requesterID {
		return nil, domain.ErrForbiddenTenantAccess
	}

	// 3. Rotar credenciales en el motor de base de datos (PostgreSQL / Crossplane)
	creds, err := uc.provisioner.ResetPassword(ctx, tenant.Slug)
	if err != nil {
		return nil, fmt.Errorf("fallo al rotar credenciales de base de datos del inquilino: %w", err)
	}

	return &ResetTenantPasswordOutput{
		TenantID: tenant.ID,
		Slug:     tenant.Slug,
		Database: creds,
	}, nil
}

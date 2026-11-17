package usecase

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"pleyades-app/internal/domain"
	"pleyades-app/internal/ports"
)

// CreateTenantInput contiene los parámetros requeridos para crear un nuevo inquilino
type CreateTenantInput struct {
	ID          string
	Name        string
	Slug        string
	OwnerUserID string
}

// CreateTenantOutput contiene la respuesta estructurada tras crear un inquilino
type CreateTenantOutput struct {
	ID          string
	Name        string
	Slug        string
	OwnerUserID string
	Status      string
	DBName      string
	CreatedAt   time.Time
}

// CreateTenantUseCase orquesta la validación, aprovisionamiento físico y persistencia de un nuevo tenant
type CreateTenantUseCase struct {
	tenantRepo  ports.TenantRepository
	provisioner ports.TenantProvisioner
}

// NewCreateTenantUseCase inicializa el caso de uso inyectando sus dependencias (DIP)
func NewCreateTenantUseCase(tenantRepo ports.TenantRepository, provisioner ports.TenantProvisioner) *CreateTenantUseCase {
	return &CreateTenantUseCase{
		tenantRepo:  tenantRepo,
		provisioner: provisioner,
	}
}

// Execute ejecuta la lógica de creación atómica de un inquilino
func (uc *CreateTenantUseCase) Execute(ctx context.Context, input CreateTenantInput) (*CreateTenantOutput, error) {
	slug := strings.ToLower(strings.TrimSpace(input.Slug))
	name := strings.TrimSpace(input.Name)
	ownerID := strings.TrimSpace(input.OwnerUserID)

	id := input.ID
	if id == "" {
		id = generateUUID()
	}

	dbName := fmt.Sprintf("tenant_%s", strings.ReplaceAll(slug, "-", "_"))

	now := time.Now()
	tenant := domain.Tenant{
		ID:          id,
		Name:        name,
		Slug:        slug,
		OwnerUserID: ownerID,
		Status:      domain.TenantStatusActive,
		DBName:      dbName,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// 1. Validar invariantes del dominio
	if err := tenant.Validate(); err != nil {
		return nil, err
	}

	// 2. Verificar que el slug no exista previamente
	existing, err := uc.tenantRepo.FindBySlug(ctx, tenant.Slug)
	if err != nil && !errors.Is(err, domain.ErrTenantNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, domain.ErrTenantSlugConflict
	}

	// 3. Aprovisionar infraestructura aislada para el tenant (DDL / Crossplane)
	if _, err := uc.provisioner.Provision(ctx, tenant.Slug); err != nil {
		return nil, fmt.Errorf("fallo al aprovisionar base de datos para el inquilino: %w", err)
	}

	// 4. Persistir metadatos del tenant en la base de datos principal
	if err := uc.tenantRepo.Save(ctx, tenant); err != nil {
		return nil, fmt.Errorf("fallo al guardar metadatos del inquilino: %w", err)
	}

	return &CreateTenantOutput{
		ID:          tenant.ID,
		Name:        tenant.Name,
		Slug:        tenant.Slug,
		OwnerUserID: tenant.OwnerUserID,
		Status:      tenant.Status,
		DBName:      tenant.DBName,
		CreatedAt:   tenant.CreatedAt,
	}, nil
}

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

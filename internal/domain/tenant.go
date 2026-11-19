package domain

import (
	"regexp"
	"strings"
	"time"
)

var slugRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`)

const (
	TenantStatusActive    = "ACTIVE"
	TenantStatusSuspended = "SUSPENDED"
)

// Tenant representa la entidad de dominio de un inquilino multi-tenant
type Tenant struct {
	ID          string
	Name        string
	Slug        string
	OwnerUserID string
	Status      string
	DBName      string
	PoolID      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ValidateName verifica que el nombre no esté vacío ni exceda 50 caracteres
func (t Tenant) ValidateName() error {
	trimmed := strings.TrimSpace(t.Name)
	if trimmed == "" {
		return ErrEmptyTenantName
	}
	if len(t.Name) > 50 {
		return ErrLongTenantName
	}
	return nil
}

// ValidateSlug valida que el slug cumpla con los estándares de nombres de base de datos PostgreSQL
func (t Tenant) ValidateSlug() error {
	trimmed := strings.TrimSpace(t.Slug)
	if trimmed == "" {
		return ErrEmptyTenantSlug
	}
	if len(t.Slug) < 3 {
		return ErrShortTenantSlug
	}
	if len(t.Slug) > 30 {
		return ErrLongTenantSlug
	}
	if !slugRegex.MatchString(t.Slug) {
		return ErrInvalidTenantSlug
	}
	return nil
}

// ValidateOwnerUserID valida la presencia obligatoria del UUID del usuario propietario
func (t Tenant) ValidateOwnerUserID() error {
	if strings.TrimSpace(t.OwnerUserID) == "" {
		return ErrEmptyTenantOwnerID
	}
	return nil
}

// Validate orquesta la verificación completa de invariantes de negocio de la entidad Tenant
func (t Tenant) Validate() error {
	if err := t.ValidateName(); err != nil {
		return err
	}
	if err := t.ValidateSlug(); err != nil {
		return err
	}
	if err := t.ValidateOwnerUserID(); err != nil {
		return err
	}
	return nil
}

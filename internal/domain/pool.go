package domain

import (
	"errors"
	"strings"
	"time"
)

const (
	PoolStatusActive      = "ACTIVE"
	PoolStatusFull        = "FULL"
	PoolStatusMaintenance = "MAINTENANCE"

	DefaultMaxTenantsPerPool  = 7
	PoolScaleThresholdTenants = 5 // Umbral de advertencia para aprovisionar servidor en caliente
)

// DatabasePool representa un servidor gestionado de Azure Database for PostgreSQL Flexible Server
type DatabasePool struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Host           string    `json:"host"`
	Port           int       `json:"port"`
	AdminUser      string    `json:"admin_user"`
	AdminPassword  string    `json:"-"`
	MaxTenants     int       `json:"max_tenants"`
	CurrentTenants int       `json:"current_tenants"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// CanAcceptTenant valida si el pool tiene capacidad disponible para albergar otro inquilino
func (p DatabasePool) CanAcceptTenant() bool {
	return p.Status == PoolStatusActive && p.CurrentTenants < p.MaxTenants
}

// ShouldTriggerScaleOut evalúa si el pool alcanzó el umbral para disparar el siguiente servidor
func (p DatabasePool) ShouldTriggerScaleOut() bool {
	return p.Status == PoolStatusActive && p.CurrentTenants >= PoolScaleThresholdTenants
}

// Validate valida las invariantes del pool de base de datos
func (p DatabasePool) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return ErrInvalidPoolName
	}
	if strings.TrimSpace(p.Host) == "" {
		return ErrInvalidPoolHost
	}
	if p.Port <= 0 || p.Port > 65535 {
		return errors.New("el puerto del pool debe estar entre 1 y 65535")
	}
	if p.MaxTenants <= 0 {
		return errors.New("la capacidad máxima del pool debe ser mayor a 0")
	}
	if p.CurrentTenants < 0 {
		return errors.New("la cantidad actual de inquilinos no puede ser negativa")
	}
	return nil
}

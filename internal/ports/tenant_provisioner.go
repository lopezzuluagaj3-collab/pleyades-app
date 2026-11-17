package ports

import (
	"context"
	"fmt"
)

// TenantDBCredentials encapsula los metadatos y credenciales de conexión de la base de datos aislada del tenant
type TenantDBCredentials struct {
	Host             string `json:"host"`
	Port             int    `json:"port"`
	DBName           string `json:"name"`
	Username         string `json:"user"`
	Password         string `json:"password"`
	ConnectionString string `json:"connection_string"`
}

// BuildConnectionString construye la URI formal de conexión a PostgreSQL
func (c *TenantDBCredentials) BuildConnectionString(sslMode string) string {
	if sslMode == "" {
		sslMode = "disable"
	}
	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=%s",
		c.Username,
		c.Password,
		c.Host,
		c.Port,
		c.DBName,
		sslMode,
	)
}

// TenantDatabaseCreator define la creación inicial de infraestructura aislada
type TenantDatabaseCreator interface {
	Provision(ctx context.Context, slug string) (*TenantDBCredentials, error)
}

// TenantPasswordResetter define la regeneración y rotación de credenciales en PostgreSQL
type TenantPasswordResetter interface {
	ResetPassword(ctx context.Context, slug string) (*TenantDBCredentials, error)
}

// TenantProvisioner compone las operaciones de ciclo de vida de bases de datos para tenants (ISP)
type TenantProvisioner interface {
	TenantDatabaseCreator
	TenantPasswordResetter
}

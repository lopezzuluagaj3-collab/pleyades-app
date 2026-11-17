package ports

import (
	"context"
)

// TenantDBCredentials encapsula los metadatos y credenciales de conexión de la base de datos aislada del tenant
type TenantDBCredentials struct {
	Host     string
	Port     int
	DBName   string
	Username string
	Password string
}

// TenantProvisioner define el contrato agnóstico para la creación física de infraestructura de base de datos.
// En local, un adaptador PostgreSQL ejecuta sentencias DDL directamente en Docker.
// En Kubernetes/Azure, un adaptador interactúa con Custom Resources de Crossplane.
type TenantProvisioner interface {
	Provision(ctx context.Context, slug string) (*TenantDBCredentials, error)
}

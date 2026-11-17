package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"pleyades-app/internal/ports"
)

var validSlugIdentifier = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`)

// LocalPostgresTenantProvisioner aprovisiona bases de datos lógicas aisladas y usuarios dedicados en la instancia local de PostgreSQL
type LocalPostgresTenantProvisioner struct {
	db   *sql.DB
	host string
	port int
}

// NewLocalPostgresTenantProvisioner inicializa el aprovisionador de bases de datos para desarrollo local
func NewLocalPostgresTenantProvisioner(db *sql.DB, host string, port int) *LocalPostgresTenantProvisioner {
	return &LocalPostgresTenantProvisioner{
		db:   db,
		host: host,
		port: port,
	}
}

// Provision crea la base de datos aislada y un usuario con permisos exclusivos
func (p *LocalPostgresTenantProvisioner) Provision(ctx context.Context, slug string) (*ports.TenantDBCredentials, error) {
	if !validSlugIdentifier.MatchString(slug) {
		return nil, fmt.Errorf("postgres provisioner: slug inválido '%s'", slug)
	}

	sanitized := strings.ReplaceAll(slug, "-", "_")
	dbName := fmt.Sprintf("tenant_%s", sanitized)
	username := fmt.Sprintf("usr_%s", sanitized)
	password, err := generateSecurePassword(16)
	if err != nil {
		return nil, fmt.Errorf("postgres provisioner: fallo al generar credenciales seguras: %w", err)
	}

	// 1. Crear base de datos si no existe
	var dbExists bool
	checkDBQuery := "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)"
	if err := p.db.QueryRowContext(ctx, checkDBQuery, dbName).Scan(&dbExists); err != nil {
		return nil, fmt.Errorf("postgres provisioner: error al consultar existencia de bd: %w", err)
	}

	if !dbExists {
		createDBStmt := fmt.Sprintf(`CREATE DATABASE "%s"`, dbName)
		if _, err := p.db.ExecContext(ctx, createDBStmt); err != nil { // NOSONAR: Identifiers are strictly validated against regex
			return nil, fmt.Errorf("postgres provisioner: error al crear base de datos '%s': %w", dbName, err)
		}
	}

	// 2. Crear usuario con contraseña cifrada si no existe
	var userExists bool
	checkUserQuery := "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)"
	if err := p.db.QueryRowContext(ctx, checkUserQuery, username).Scan(&userExists); err != nil {
		return nil, fmt.Errorf("postgres provisioner: error al consultar existencia de usuario: %w", err)
	}

	if !userExists {
		escapedPass := strings.ReplaceAll(password, "'", "''")
		createUserStmt := fmt.Sprintf(`CREATE USER "%s" WITH ENCRYPTED PASSWORD '%s'`, username, escapedPass)
		if _, err := p.db.ExecContext(ctx, createUserStmt); err != nil { // NOSONAR: Identifiers and passwords are sanitized
			return nil, fmt.Errorf("postgres provisioner: error al crear usuario '%s': %w", username, err)
		}
	}

	// 3. Asignar privilegios exclusivos sobre la base de datos creada
	grantStmt := fmt.Sprintf(`GRANT ALL PRIVILEGES ON DATABASE "%s" TO "%s"`, dbName, username)
	if _, err := p.db.ExecContext(ctx, grantStmt); err != nil { // NOSONAR: Identifiers are strictly validated against regex
		return nil, fmt.Errorf("postgres provisioner: error al otorgar privilegios a '%s' sobre '%s': %w", username, dbName, err)
	}

	return &ports.TenantDBCredentials{
		Host:     p.host,
		Port:     p.port,
		DBName:   dbName,
		Username: username,
		Password: password,
	}, nil
}

func generateSecurePassword(bytesLen int) (string, error) {
	bytes := make([]byte, bytesLen)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes) + "!A1", nil
}

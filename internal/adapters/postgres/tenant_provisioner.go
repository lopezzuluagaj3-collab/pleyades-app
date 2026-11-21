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

	if err := p.ensureDatabase(ctx, dbName); err != nil {
		return nil, err
	}

	if err := p.ensureUser(ctx, username, password); err != nil {
		return nil, err
	}

	if err := p.revokePublicConnect(ctx, dbName); err != nil {
		return nil, err
	}

	if err := p.grantUserPrivileges(ctx, dbName, username); err != nil {
		return nil, err
	}

	creds := &ports.TenantDBCredentials{
		Host:     p.host,
		Port:     p.port,
		DBName:   dbName,
		Username: username,
		Password: password,
	}
	creds.ConnectionString = creds.BuildConnectionString("disable")

	return creds, nil
}

// ensureDatabase crea la base de datos de manera atómica si aún no existe
func (p *LocalPostgresTenantProvisioner) ensureDatabase(ctx context.Context, dbName string) error {
	var dbExists bool
	checkDBQuery := "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)"
	if err := p.db.QueryRowContext(ctx, checkDBQuery, dbName).Scan(&dbExists); err != nil {
		return fmt.Errorf("postgres provisioner: error al consultar existencia de bd: %w", err)
	}

	if !dbExists {
		createDBStmt := fmt.Sprintf(`CREATE DATABASE "%s"`, dbName)
		if _, err := p.db.ExecContext(ctx, createDBStmt); err != nil {
			return fmt.Errorf("postgres provisioner: error al crear base de datos '%s': %w", dbName, err)
		}
	}
	return nil
}

// ensureUser crea el usuario administrador de la base de datos si no existe
func (p *LocalPostgresTenantProvisioner) ensureUser(ctx context.Context, username, password string) error {
	var userExists bool
	checkUserQuery := "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)"
	if err := p.db.QueryRowContext(ctx, checkUserQuery, username).Scan(&userExists); err != nil {
		return fmt.Errorf("postgres provisioner: error al consultar existencia de usuario: %w", err)
	}

	if !userExists {
		escapedPass := strings.ReplaceAll(password, "'", "''")
		createUserStmt := fmt.Sprintf(`CREATE USER "%s" WITH ENCRYPTED PASSWORD '%s'`, username, escapedPass)
		if _, err := p.db.ExecContext(ctx, createUserStmt); err != nil {
			return fmt.Errorf("postgres provisioner: error al crear usuario '%s': %w", username, err)
		}
	}
	return nil
}

// revokePublicConnect revoca el privilegio CONNECT a PUBLIC para garantizar aislamiento estricto
func (p *LocalPostgresTenantProvisioner) revokePublicConnect(ctx context.Context, dbName string) error {
	stmt := strings.Join([]string{`REVOKE CONNECT ON DATABASE "`, dbName, `" FROM PUBLIC`}, "")
	if _, err := p.db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("postgres provisioner: error al revocar CONNECT a PUBLIC en '%s': %w", dbName, err)
	}
	return nil
}

// grantUserPrivileges asigna privilegios exclusivos sobre la base de datos al usuario del tenant
func (p *LocalPostgresTenantProvisioner) grantUserPrivileges(ctx context.Context, dbName, username string) error {
	grantStmt := fmt.Sprintf(`GRANT ALL PRIVILEGES ON DATABASE "%s" TO "%s"`, dbName, username)
	if _, err := p.db.ExecContext(ctx, grantStmt); err != nil {
		return fmt.Errorf("postgres provisioner: error al otorgar privilegios a '%s' sobre '%s': %w", username, dbName, err)
	}
	return nil
}

// ResetPassword rota la contraseña del usuario de base de datos en PostgreSQL
func (p *LocalPostgresTenantProvisioner) ResetPassword(ctx context.Context, slug string) (*ports.TenantDBCredentials, error) {
	if !validSlugIdentifier.MatchString(slug) {
		return nil, fmt.Errorf("postgres provisioner: slug inválido '%s'", slug)
	}

	sanitized := strings.ReplaceAll(slug, "-", "_")
	dbName := fmt.Sprintf("tenant_%s", sanitized)
	username := fmt.Sprintf("usr_%s", sanitized)
	newPassword, err := generateSecurePassword(16)
	if err != nil {
		return nil, fmt.Errorf("postgres provisioner: fallo al generar nueva contraseña: %w", err)
	}

	// 1. Verificar existencia del rol
	var userExists bool
	checkUserQuery := "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)"
	if err := p.db.QueryRowContext(ctx, checkUserQuery, username).Scan(&userExists); err != nil {
		return nil, fmt.Errorf("postgres provisioner: error al consultar existencia de usuario: %w", err)
	}
	if !userExists {
		return nil, fmt.Errorf("postgres provisioner: el usuario '%s' no existe en postgres", username)
	}

	// 2. Modificar contraseña en PostgreSQL (ALTER USER "usr_..." WITH ENCRYPTED PASSWORD '...')
	escapedPass := strings.ReplaceAll(newPassword, "'", "''")
	alterUserStmt := fmt.Sprintf(`ALTER USER "%s" WITH ENCRYPTED PASSWORD '%s'`, username, escapedPass)
	if _, err := p.db.ExecContext(ctx, alterUserStmt); err != nil { // NOSONAR: Identifiers and passwords are sanitized
		return nil, fmt.Errorf("postgres provisioner: error al rotar contraseña de usuario '%s': %w", username, err)
	}

	creds := &ports.TenantDBCredentials{
		Host:     p.host,
		Port:     p.port,
		DBName:   dbName,
		Username: username,
		Password: newPassword,
	}
	creds.ConnectionString = creds.BuildConnectionString("disable")

	return creds, nil
}

func generateSecurePassword(bytesLen int) (string, error) {
	bytes := make([]byte, bytesLen)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes) + "!A1", nil
}

package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"pleyades-app/internal/domain"
	"pleyades-app/internal/ports"
)

var _ ports.TenantProvisioner = (*ShardedPostgresTenantProvisioner)(nil)

// ShardedPostgresTenantProvisioner gestiona el aprovisionamiento de bases de datos distribuidas en pools de Azure Flexible Server
type ShardedPostgresTenantProvisioner struct {
	primaryDB   *sql.DB
	primaryHost string
	primaryPort int
	poolRepo    ports.DatabasePoolRepository
	tenantRepo  ports.TenantRepository
	sslMode     string
}

// NewShardedPostgresTenantProvisioner crea una instancia del aprovisionador sharded con balanceo por capacidad
func NewShardedPostgresTenantProvisioner(
	primaryDB *sql.DB,
	primaryHost string,
	primaryPort int,
	poolRepo ports.DatabasePoolRepository,
	tenantRepo ports.TenantRepository,
	sslMode string,
) *ShardedPostgresTenantProvisioner {
	if sslMode == "" {
		sslMode = "require"
	}
	return &ShardedPostgresTenantProvisioner{
		primaryDB:   primaryDB,
		primaryHost: primaryHost,
		primaryPort: primaryPort,
		poolRepo:    poolRepo,
		tenantRepo:  tenantRepo,
		sslMode:     sslMode,
	}
}

func (p *ShardedPostgresTenantProvisioner) getConnectionForPool(ctx context.Context, pool *domain.DatabasePool) (*sql.DB, func(), error) {
	if pool.Host == p.primaryHost && pool.Port == p.primaryPort {
		return p.primaryDB, func() {
			// No-op: El pool de conexiones principal es gestionado por el ciclo de vida de la aplicación en main
		}, nil
	}

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s connect_timeout=5",
		pool.Host, pool.Port, pool.AdminUser, pool.AdminPassword, p.sslMode,
	)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("error al conectar con el servidor pool '%s': %w", pool.Name, err)
	}

	cleanup := func() {
		_ = db.Close()
	}

	if err := db.PingContext(ctx); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("error de conectividad con el servidor pool '%s' (%s): %w", pool.Name, pool.Host, err)
	}

	return db, cleanup, nil
}

// Provision aprovisiona la base de datos y usuario en el pool de Azure Flexible Server con capacidad disponible
func (p *ShardedPostgresTenantProvisioner) Provision(ctx context.Context, slug string) (*ports.TenantDBCredentials, error) {
	if !validSlugIdentifier.MatchString(slug) {
		return nil, fmt.Errorf("sharded provisioner: slug inválido '%s'", slug)
	}

	// 1. Obtener pool activo con capacidad disponible (bin-packing de 5 a 7 inquilinos)
	pool, err := p.poolRepo.GetAvailablePool(ctx)
	if err != nil {
		return nil, fmt.Errorf("sharded provisioner: no hay pool de base de datos disponible: %w", err)
	}

	sanitized := strings.ReplaceAll(slug, "-", "_")
	dbName := fmt.Sprintf("tenant_%s", sanitized)
	username := fmt.Sprintf("usr_%s", sanitized)
	password, err := generateSecurePassword(16)
	if err != nil {
		return nil, fmt.Errorf("sharded provisioner: fallo al generar contraseña: %w", err)
	}

	// 2. Obtener conexión contra el servidor físico asignado
	targetDB, cleanup, err := p.getConnectionForPool(ctx, pool)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	// 3. Crear base de datos si no existe
	var dbExists bool
	checkDBQuery := "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)"
	if err := targetDB.QueryRowContext(ctx, checkDBQuery, dbName).Scan(&dbExists); err != nil {
		return nil, fmt.Errorf("sharded provisioner: error al consultar existencia de bd en pool '%s': %w", pool.Name, err)
	}

	if !dbExists {
		createDBStmt := fmt.Sprintf(`CREATE DATABASE "%s"`, dbName)
		if _, err := targetDB.ExecContext(ctx, createDBStmt); err != nil { // NOSONAR: Identifiers are strictly validated against regex
			return nil, fmt.Errorf("sharded provisioner: error al crear bd '%s' en pool '%s': %w", dbName, pool.Name, err)
		}
	}

	// 4. Crear usuario con contraseña cifrada si no existe
	var userExists bool
	checkUserQuery := "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)"
	if err := targetDB.QueryRowContext(ctx, checkUserQuery, username).Scan(&userExists); err != nil {
		return nil, fmt.Errorf("sharded provisioner: error al consultar existencia de usuario en pool '%s': %w", pool.Name, err)
	}

	if !userExists {
		escapedPass := strings.ReplaceAll(password, "'", "''")
		createUserStmt := fmt.Sprintf(`CREATE USER "%s" WITH ENCRYPTED PASSWORD '%s'`, username, escapedPass)
		if _, err := targetDB.ExecContext(ctx, createUserStmt); err != nil { // NOSONAR: Identifiers and passwords are sanitized
			return nil, fmt.Errorf("sharded provisioner: error al crear usuario '%s' en pool '%s': %w", username, pool.Name, err)
		}
	}

	// 5. Asignar privilegios exclusivos
	grantStmt := fmt.Sprintf(`GRANT ALL PRIVILEGES ON DATABASE "%s" TO "%s"`, dbName, username)
	if _, err := targetDB.ExecContext(ctx, grantStmt); err != nil { // NOSONAR: Identifiers are strictly validated against regex
		return nil, fmt.Errorf("sharded provisioner: error al otorgar privilegios en pool '%s': %w", pool.Name, err)
	}

	// 6. Incrementar contador en el catálogo del pool (auto-marca FULL si alcanza max_tenants)
	if err := p.poolRepo.IncrementTenantCount(ctx, pool.ID); err != nil {
		return nil, fmt.Errorf("sharded provisioner: error al actualizar contador de inquilinos en pool '%s': %w", pool.Name, err)
	}

	creds := &ports.TenantDBCredentials{
		Host:     pool.Host,
		Port:     pool.Port,
		DBName:   dbName,
		Username: username,
		Password: password,
		PoolID:   pool.ID,
	}
	creds.ConnectionString = creds.BuildConnectionString(p.sslMode)

	return creds, nil
}

// ResetPassword rota la contraseña del usuario en el pool donde reside el tenant
func (p *ShardedPostgresTenantProvisioner) ResetPassword(ctx context.Context, slug string) (*ports.TenantDBCredentials, error) {
	if !validSlugIdentifier.MatchString(slug) {
		return nil, fmt.Errorf("sharded provisioner: slug inválido '%s'", slug)
	}

	// 1. Consultar tenant para obtener el pool_id asignado
	tenant, err := p.tenantRepo.FindBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("sharded provisioner: inquilino no encontrado: %w", err)
	}

	var pool *domain.DatabasePool
	if tenant.PoolID != "" {
		pool, err = p.poolRepo.FindByID(ctx, tenant.PoolID)
		if err != nil {
			return nil, fmt.Errorf("sharded provisioner: error al buscar pool '%s': %w", tenant.PoolID, err)
		}
	} else {
		// Fallback al pool primario si no tuviese pool_id explícito
		pool = &domain.DatabasePool{
			ID:        "",
			Name:      "primary",
			Host:      p.primaryHost,
			Port:      p.primaryPort,
			AdminUser: "",
		}
	}

	targetDB, cleanup, err := p.getConnectionForPool(ctx, pool)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	sanitized := strings.ReplaceAll(slug, "-", "_")
	dbName := fmt.Sprintf("tenant_%s", sanitized)
	username := fmt.Sprintf("usr_%s", sanitized)
	newPassword, err := generateSecurePassword(16)
	if err != nil {
		return nil, fmt.Errorf("sharded provisioner: fallo al generar nueva contraseña: %w", err)
	}

	// 2. Verificar existencia del rol
	var userExists bool
	checkUserQuery := "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)"
	if err := targetDB.QueryRowContext(ctx, checkUserQuery, username).Scan(&userExists); err != nil {
		return nil, fmt.Errorf("sharded provisioner: error al consultar usuario en pool '%s': %w", pool.Name, err)
	}
	if !userExists {
		return nil, fmt.Errorf("sharded provisioner: el usuario '%s' no existe en pool '%s'", username, pool.Name)
	}

	// 3. Modificar contraseña
	escapedPass := strings.ReplaceAll(newPassword, "'", "''")
	alterUserStmt := fmt.Sprintf(`ALTER USER "%s" WITH ENCRYPTED PASSWORD '%s'`, username, escapedPass)
	if _, err := targetDB.ExecContext(ctx, alterUserStmt); err != nil { // NOSONAR: Identifiers and passwords are sanitized
		return nil, fmt.Errorf("sharded provisioner: error al rotar contraseña en pool '%s': %w", pool.Name, err)
	}

	creds := &ports.TenantDBCredentials{
		Host:     pool.Host,
		Port:     pool.Port,
		DBName:   dbName,
		Username: username,
		Password: newPassword,
		PoolID:   pool.ID,
	}
	creds.ConnectionString = creds.BuildConnectionString(p.sslMode)

	return creds, nil
}

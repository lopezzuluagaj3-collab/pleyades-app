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
	primaryDB    *sql.DB
	primaryHost  string
	primaryPort  int
	poolRepo     ports.DatabasePoolRepository
	tenantRepo   ports.TenantRepository
	scaleTrigger ports.PoolScaleTrigger
	sslMode      string
}

// NewShardedPostgresTenantProvisioner crea una instancia del aprovisionador sharded con balanceo por capacidad
func NewShardedPostgresTenantProvisioner(
	primaryDB *sql.DB,
	primaryHost string,
	primaryPort int,
	poolRepo ports.DatabasePoolRepository,
	tenantRepo ports.TenantRepository,
	scaleTrigger ports.PoolScaleTrigger,
	sslMode string,
) *ShardedPostgresTenantProvisioner {
	if sslMode == "" {
		sslMode = "require"
	}
	return &ShardedPostgresTenantProvisioner{
		primaryDB:    primaryDB,
		primaryHost:  primaryHost,
		primaryPort:  primaryPort,
		poolRepo:     poolRepo,
		tenantRepo:   tenantRepo,
		scaleTrigger: scaleTrigger,
		sslMode:      sslMode,
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
	if len(slug) < 3 || len(slug) > 30 || !validSlugIdentifier.MatchString(slug) {
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

	// 3. Aprovisionar base de datos, usuario, aislamiento y privilegios de forma atómica (SRP)
	if err := p.ensureDatabase(ctx, targetDB, dbName, pool.Name); err != nil {
		return nil, err
	}

	if err := p.ensureUser(ctx, targetDB, username, password, pool.Name); err != nil {
		return nil, err
	}

	if err := p.revokePublicConnect(ctx, targetDB, dbName, pool.Name); err != nil {
		return nil, err
	}

	if err := p.grantUserPrivileges(ctx, targetDB, dbName, username, pool.Name); err != nil {
		return nil, err
	}

	// 4. Incrementar contador en el catálogo del pool (auto-marca FULL si alcanza max_tenants)
	if err := p.poolRepo.IncrementTenantCount(ctx, pool.ID); err != nil {
		return nil, fmt.Errorf("sharded provisioner: error al actualizar contador de inquilinos en pool '%s': %w", pool.Name, err)
	}

	// 5. Evaluar umbral de advertencia para disparar el aprovisionamiento de servidor en caliente (Warm Pool)
	if p.scaleTrigger != nil && pool.CurrentTenants+1 >= domain.PoolScaleThresholdTenants {
		nextPoolName := "psql-pleyades-dev-02"
		_ = p.scaleTrigger.TriggerWarmPool(ctx, nextPoolName)
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

// ensureDatabase crea la base de datos de manera atómica si aún no existe
func (p *ShardedPostgresTenantProvisioner) ensureDatabase(ctx context.Context, targetDB *sql.DB, dbName, poolName string) error {
	var dbExists bool
	checkDBQuery := "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)"
	if err := targetDB.QueryRowContext(ctx, checkDBQuery, dbName).Scan(&dbExists); err != nil {
		return fmt.Errorf("sharded provisioner: error al consultar existencia de bd en pool '%s': %w", poolName, err)
	}

	if !dbExists {
		createDBStmt := fmt.Sprintf(`CREATE DATABASE "%s"`, dbName)
		if _, err := targetDB.ExecContext(ctx, createDBStmt); err != nil { // NOSONAR: Identifiers are strictly validated against regex
			return fmt.Errorf("sharded provisioner: error al crear bd '%s' en pool '%s': %w", dbName, poolName, err)
		}
	}
	return nil
}

// ensureUser crea el usuario administrador de la base de datos si no existe
func (p *ShardedPostgresTenantProvisioner) ensureUser(ctx context.Context, targetDB *sql.DB, username, password, poolName string) error {
	var userExists bool
	checkUserQuery := "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)"
	if err := targetDB.QueryRowContext(ctx, checkUserQuery, username).Scan(&userExists); err != nil {
		return fmt.Errorf("sharded provisioner: error al consultar existencia de usuario en pool '%s': %w", poolName, err)
	}

	if !userExists {
		createUserStmt := BuildCreateUserDDL(username, password, DefaultTenantConnectionLimit)
		if _, err := targetDB.ExecContext(ctx, createUserStmt); err != nil { // NOSONAR: Identifiers and passwords are sanitized
			return fmt.Errorf("sharded provisioner: error al crear usuario '%s' en pool '%s': %w", username, poolName, err)
		}
	} else {
		alterRoleStmt := BuildAlterUserConnectionLimitDDL(username, DefaultTenantConnectionLimit)
		if _, err := targetDB.ExecContext(ctx, alterRoleStmt); err != nil { // NOSONAR: Identifiers are strictly validated against regex
			return fmt.Errorf("sharded provisioner: error al actualizar límite de conexiones a '%s' en pool '%s': %w", username, poolName, err)
		}
	}
	return nil
}

// revokePublicConnect revoca el privilegio CONNECT a PUBLIC para garantizar aislamiento estricto
func (p *ShardedPostgresTenantProvisioner) revokePublicConnect(ctx context.Context, targetDB *sql.DB, dbName, poolName string) error {
	stmt := BuildRevokePublicConnectDDL(dbName)
	if _, err := targetDB.ExecContext(ctx, stmt); err != nil { // NOSONAR: Identifiers are strictly validated against regex
		return fmt.Errorf("sharded provisioner: error al revocar CONNECT a PUBLIC en '%s' (pool '%s'): %w", dbName, poolName, err)
	}
	return nil
}

// grantUserPrivileges asigna privilegios exclusivos sobre la base de datos al usuario del tenant
func (p *ShardedPostgresTenantProvisioner) grantUserPrivileges(ctx context.Context, targetDB *sql.DB, dbName, username, poolName string) error {
	grantStmt := BuildGrantPrivilegesDDL(dbName, username)
	if _, err := targetDB.ExecContext(ctx, grantStmt); err != nil { // NOSONAR: Identifiers are strictly validated against regex
		return fmt.Errorf("sharded provisioner: error al otorgar privilegios en pool '%s': %w", poolName, err)
	}
	return nil
}

// ResetPassword rota la contraseña del usuario en el pool donde reside el tenant
func (p *ShardedPostgresTenantProvisioner) ResetPassword(ctx context.Context, slug string) (*ports.TenantDBCredentials, error) {
	if len(slug) < 3 || len(slug) > 30 || !validSlugIdentifier.MatchString(slug) {
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

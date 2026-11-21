package postgres

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/jackc/pgx/v5/stdlib"
	"time"
)

type DBConfig struct {
	Host         string
	Port         string
	User         string
	Password     string
	DBName       string
	SSLMode      string
	MaxOpenConns int
	MaxIdleConns int
}

// NewConnection abre el pool de conexiones a PostgreSQL y aplica buenas prácticas cloud-native
func NewConnection(cfg DBConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
		cfg.SSLMode,
	)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: error al abrir la conexión: %w", err)
	}

	// Dimensionamiento defensivo del Pool frente al HPA en Kubernetes:
	// En Azure Database for PostgreSQL Flexible Server (SKU Standard_B1ms), max_connections = 50 (~47 disponibles).
	// Fijar 5 conexiones por pod garantiza que hasta 7 pods en rolling update consuman un máximo de 35 conexiones,
	// dejando un margen de seguridad > 12 conexiones para mantenimiento, administradores y procesos del sistema.
	maxOpen := cfg.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 5
	}
	maxIdle := cfg.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = maxOpen
	}

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	// Verificación de conectividad inmediata con timeout defensivo
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("postgres: error al verificar ping: %w", err)
	}

	return db, nil
}

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/bcrypt"
	"pleyades-app/internal/adapters/crypto"
	"pleyades-app/internal/adapters/http"
	"pleyades-app/internal/adapters/http/middleware"
	"pleyades-app/internal/adapters/postgres"
	"pleyades-app/internal/config"
	"pleyades-app/internal/usecase"
)

func main() {
	// 1. Cargar configuración (Fail-Fast si faltan variables)
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("error fatal al cargar la configuración: %v", err)
	}

	// 2. Abrir Pool de Conexiones a PostgreSQL
	db, err := postgres.NewConnection(cfg.DB)
	if err != nil {
		log.Fatalf("error fatal al conectar con postgres: %v", err)
	}
	defer func() {
		_ = db.Close()
		log.Println("Pool de conexiones a PostgreSQL cerrado limpiamente.")
	}()

	// 3. Inyección de Dependencias
	metrics := middleware.NewMetrics(prometheus.DefaultRegisterer)
	rateLimiter := middleware.NewRateLimiter(20.0, 50.0, 10*time.Minute)
	hasher := crypto.NewBcryptHasher(bcrypt.DefaultCost)
	tokenService := crypto.NewJWTTokenService(cfg.JWTSecret)
	authMiddleware := middleware.NewAuthMiddleware(tokenService, 5*time.Minute, 15*time.Minute)
	idempotency := middleware.NewIdempotencyMiddleware(10 * time.Minute)

	userRepo := postgres.NewUserRepository(db)
	tenantRepo := postgres.NewTenantRepository(db)

	// Inicializar esquema de tenants de forma defensiva
	initCtx, initCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := tenantRepo.InitSchema(initCtx); err != nil {
		initCancel()
		log.Fatalf("error fatal al inicializar esquema de tenants: %v", err)
	}
	initCancel()

	dbPort, _ := strconv.Atoi(cfg.DB.Port)
	if dbPort == 0 {
		dbPort = 5432
	}
	tenantProvisioner := postgres.NewLocalPostgresTenantProvisioner(db, cfg.DB.Host, dbPort)

	userUseCase := usecase.NewRegisterUserUseCase(userRepo, hasher)
	loginUseCase := usecase.NewLoginUserUseCase(userRepo, hasher, tokenService, 15*time.Minute)
	tenantUseCase := usecase.NewCreateTenantUseCase(tenantRepo, tenantProvisioner)

	userHandler := httpadapter.NewUserHandler(userUseCase)
	authHandler := httpadapter.NewAuthHandler(loginUseCase)
	tenantHandler := httpadapter.NewTenantHandler(tenantUseCase)

	router := httpadapter.NewRouter(
		userHandler,
		authHandler,
		tenantHandler,
		metrics,
		rateLimiter,
		authMiddleware,
		idempotency,
	)

	// 4. Configurar Servidor HTTP con Timeouts Cloud-Native
	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	// 5. Canal para atrapar errores de inicio del servidor
	serverErrors := make(chan error, 1)

	// Iniciar servidor HTTP en una goroutine (no bloquea el hilo principal)
	go func() {
		log.Printf("Pleyades microservicio iniciado en el puerto %s\n", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()

	// 6. Configurar captura de señales del Sistema Operativo (Graceful Shutdown)
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Bloquear hasta recibir una señal de apagado o un error crítico de arranque
	select {
	case err := <-serverErrors:
		log.Fatalf("error fatal al iniciar el servidor: %v", err)

	case sig := <-shutdown:
		log.Printf("señal de apagado recibida (%s). Iniciando Graceful Shutdown...\n", sig)

		// Contexto con timeout de gracia de 10 segundos para completar peticiones en vuelo
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Detener recepción de nuevas peticiones y esperar a las que están en curso
		if err := server.Shutdown(ctx); err != nil {
			_ = server.Close()
			log.Fatalf("error durante el apagado forzado del servidor: %v", err)
		}

		log.Println("Servidor HTTP apagado limpiamente.")
	}
}

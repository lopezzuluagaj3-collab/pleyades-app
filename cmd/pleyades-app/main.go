package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
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
	hasher := crypto.NewBcryptHasher(bcrypt.DefaultCost)
	userRepo := postgres.NewUserRepository(db)
	userUseCase := usecase.NewRegisterUserUseCase(userRepo, hasher)
	userHandler := httpadapter.NewUserHandler(userUseCase)
	router := httpadapter.NewRouter(userHandler, metrics)

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

package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"pleyades-app/internal/adapters/http"
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
	}()

	// 3. Inyección de Dependencias
	userRepo := postgres.NewUserRepository(db)
	userUseCase := usecase.NewRegisterUserUseCase(userRepo)
	userHandler := httpadapter.NewUserHandler(userUseCase)
	router := httpadapter.NewRouter(userHandler)

	// 4. Configurar Servidor HTTP con Timeouts Cloud-Native
	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	log.Printf("Pleyades microservicio iniciado en el puerto %s\n", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("error fatal en el servidor: %v", err)
	}
}

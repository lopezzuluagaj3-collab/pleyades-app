package main

import (
	"log"
	"net/http"
	httpadapter "pleyades-app/internal/adapters/http"
	"time"
)

func main() {
	router := httpadapter.NewRouter()

	server := &http.Server{
		Addr:         ":8080",
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	log.Println("Pleyades microservicio iniciado en el puerto :8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("error fatal en el servidor: %v", err)
	}
}

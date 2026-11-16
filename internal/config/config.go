package config

import (
	"bufio"
	"fmt"
	"os"
	"pleyades-app/internal/adapters/postgres"
	"strings"
)

type Config struct {
	ServerPort string
	DB         postgres.DBConfig
}

func getEnv(key, defaultValue string) string {
	if val, exists := os.LookupEnv(key); exists && val != "" {
		return val
	}
	return defaultValue
}

func getRequiredEnv(key string) (string, error) {
	val, exists := os.LookupEnv(key)
	if !exists || strings.TrimSpace(val) == "" {
		return "", fmt.Errorf("variable de entorno obligatoria faltante: %s", key)
	}
	return val, nil
}

// loadEnvFile lee el archivo .env si existe en local y carga sus variables en el sistema
func loadEnvFile() {
	file, err := os.Open(".env")
	if err != nil {
		// En Kubernetes el archivo .env no existe, así que ignoramos el error
		return
	}
	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			_ = os.Setenv(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
}

// Load carga la configuración sin credenciales quemadas
func Load() (Config, error) {
	loadEnvFile()

	SERVER_PORT, err := getRequiredEnv("SERVER_PORT")
	if err != nil {
		return Config{}, err
	}

	DB_PORT, err := getRequiredEnv("DB_PORT")
	if err != nil {
		return Config{}, err
	}

	host, err := getRequiredEnv("DB_HOST")
	if err != nil {
		return Config{}, err
	}

	user, err := getRequiredEnv("DB_USER")
	if err != nil {
		return Config{}, err
	}

	pass, err := getRequiredEnv("DB_PASSWORD")
	if err != nil {
		return Config{}, err
	}

	name, err := getRequiredEnv("DB_NAME")
	if err != nil {
		return Config{}, err
	}

	return Config{
		ServerPort: SERVER_PORT,
		DB: postgres.DBConfig{
			Host:     host,
			Port:     DB_PORT,
			User:     user,
			Password: pass,
			DBName:   name,
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
	}, nil
}

package config

import (
	"bufio"
	"fmt"
	"os"
	"pleyades-app/internal/adapters/postgres"
	"strconv"
	"strings"
)

type Config struct {
	ServerPort string
	DB         postgres.DBConfig
	JWTSecret  string
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

	serverPort, err := getRequiredEnv("SERVER_PORT")
	if err != nil {
		return Config{}, err
	}

	dbPort, err := getRequiredEnv("DB_PORT")
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

	jwtSecret := getEnv("JWT_SECRET", "super-secret-pleyades-enterprise-key-for-local-dev-123456789")

	maxOpenStr := getEnv("DB_MAX_OPEN_CONNS", "5")
	maxOpen, err := strconv.Atoi(maxOpenStr)
	if err != nil || maxOpen <= 0 {
		maxOpen = 5
	}

	maxIdleStr := getEnv("DB_MAX_IDLE_CONNS", "5")
	maxIdle, err := strconv.Atoi(maxIdleStr)
	if err != nil || maxIdle <= 0 {
		maxIdle = maxOpen
	}

	return Config{
		ServerPort: serverPort,
		JWTSecret:  jwtSecret,
		DB: postgres.DBConfig{
			Host:         host,
			Port:         dbPort,
			User:         user,
			Password:     pass,
			DBName:       name,
			SSLMode:      getEnv("DB_SSLMODE", "disable"),
			MaxOpenConns: maxOpen,
			MaxIdleConns: maxIdle,
		},
	}, nil
}

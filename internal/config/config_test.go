package config_test

import (
	"os"
	"testing"

	"pleyades-app/internal/config"
)

// TestConfigSuite orquesta las pruebas unitarias del cargador de configuración
func TestConfigSuite(t *testing.T) {
	t.Run("Fail-Fast on Missing Required Env", testConfigFailFast)
	t.Run("Success Loading Complete Env", testConfigSuccess)
}

func testConfigFailFast(t *testing.T) {
	_ = os.Unsetenv("SERVER_PORT")
	_ = os.Unsetenv("DB_PORT")
	_ = os.Unsetenv("DB_HOST")
	_ = os.Unsetenv("DB_USER")
	_ = os.Unsetenv("DB_PASSWORD")
	_ = os.Unsetenv("DB_NAME")

	_, err := config.Load()
	if err == nil {
		t.Fatalf("expected error on missing required env, got nil")
	}
}

func testConfigSuccess(t *testing.T) {
	_ = os.Setenv("SERVER_PORT", "8080")
	_ = os.Setenv("DB_PORT", "5432")
	_ = os.Setenv("DB_HOST", "localhost")
	_ = os.Setenv("DB_USER", "pleyades")
	_ = os.Setenv("DB_PASSWORD", "secret")
	_ = os.Setenv("DB_NAME", "pleyades_main")
	_ = os.Setenv("DB_SSLMODE", "disable")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.ServerPort != "8080" {
		t.Errorf("expected ServerPort 8080, got %s", cfg.ServerPort)
	}
	if cfg.DB.Host != "localhost" {
		t.Errorf("expected DB Host localhost, got %s", cfg.DB.Host)
	}
	if cfg.JWTSecret == "" {
		t.Errorf("expected non-empty default JWTSecret")
	}
}

package crypto_test

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
	"pleyades-app/internal/adapters/crypto"
)

// TestBcryptHasherSuite orquesta las pruebas unitarias del adaptador de hashing
func TestBcryptHasherSuite(t *testing.T) {
	t.Run("Hash and Compare Success", testBcryptHashAndCompareSuccess)
	t.Run("Compare Mismatch Failure", testBcryptCompareMismatch)
	t.Run("Custom Cost Boundary", testBcryptCustomCost)
	t.Run("CompareDummy Execution", testBcryptCompareDummy)
}

func testBcryptHashAndCompareSuccess(t *testing.T) {
	hasher := crypto.NewBcryptHasher(bcrypt.DefaultCost)
	rawPass := "MySecretPass123!"

	hash, err := hasher.Hash(rawPass)
	if err != nil {
		t.Fatalf("unexpected hash error: %v", err)
	}

	if err := hasher.Compare(hash, rawPass); err != nil {
		t.Fatalf("expected compare to succeed, got error: %v", err)
	}
}

func testBcryptCompareMismatch(t *testing.T) {
	hasher := crypto.NewBcryptHasher(bcrypt.DefaultCost)
	hash, _ := hasher.Hash("CorrectPass123!")

	if err := hasher.Compare(hash, "WrongPass123!"); err == nil {
		t.Errorf("expected compare to fail on password mismatch, got nil")
	}
}

func testBcryptCustomCost(t *testing.T) {
	hasher := crypto.NewBcryptHasher(1)
	if hasher == nil {
		t.Fatalf("expected hasher instance, got nil")
	}
}

func testBcryptCompareDummy(t *testing.T) {
	hasher := crypto.NewBcryptHasher(bcrypt.DefaultCost)
	// CompareDummy siempre debe ejecutar la comparación y arrojar error contra contraseñas arbitrarias
	if err := hasher.CompareDummy("AnyPassword123!"); err == nil {
		t.Errorf("expected dummy compare to fail against synthetic hash, got nil")
	}
}

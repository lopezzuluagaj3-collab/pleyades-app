package crypto

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// BcryptHasher implementa el puerto ports.PasswordHasher utilizando el algoritmo bcrypt
type BcryptHasher struct {
	cost      int
	dummyHash []byte
}

// NewBcryptHasher crea una instancia con coste configurable y baseline sintético en memoria
func NewBcryptHasher(cost int) *BcryptHasher {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = bcrypt.DefaultCost
	}
	// Se genera un hash sintético dinámicamente en tiempo de inicio para mitigar timing attacks sin almacenar credenciales estáticas
	dummyHash, _ := bcrypt.GenerateFromPassword([]byte("pleyades_synthetic_timing_baseline"), cost)
	return &BcryptHasher{
		cost:      cost,
		dummyHash: dummyHash,
	}
}

func (b *BcryptHasher) Hash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), b.cost)
	if err != nil {
		return "", fmt.Errorf("crypto: error al hashear la contraseña: %w", err)
	}
	return string(bytes), nil
}

func (b *BcryptHasher) Compare(hashedPassword, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)); err != nil {
		return fmt.Errorf("crypto: la contraseña no coincide: %w", err)
	}
	return nil
}

// CompareDummy realiza una comparación con coste computacional idéntico (~70ms) para neutralizar timing attacks
func (b *BcryptHasher) CompareDummy(password string) error {
	if err := bcrypt.CompareHashAndPassword(b.dummyHash, []byte(password)); err != nil {
		return fmt.Errorf("crypto: dummy mismatch: %w", err)
	}
	return nil
}

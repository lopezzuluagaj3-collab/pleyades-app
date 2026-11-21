package ports

// PasswordHasher define el contrato para el hashing y verificación segura de contraseñas
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hashedPassword, password string) error
	// CompareDummy ejecuta una comparación de coste equivalente para mitigar timing attacks
	CompareDummy(password string) error
}

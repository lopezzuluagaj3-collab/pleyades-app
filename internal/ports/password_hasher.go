package ports

// PasswordHasher define el contrato para el hashing y verificación segura de contraseñas
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hashedPassword, password string) error
}

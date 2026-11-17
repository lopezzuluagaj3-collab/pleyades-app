package domain_test

import (
	"errors"
	"testing"

	"pleyades-app/internal/domain"
)

// TestUserValidationSuite es el orquestador principal de las pruebas de invariantes del dominio
func TestUserValidationSuite(t *testing.T) {
	t.Run("Email Validation", testEmailValidation)
	t.Run("Name Validation", testNameValidation)
	t.Run("Password Validation", testPasswordValidation)
	t.Run("Master Invariants Validation", testMasterValidation)
}

func testEmailValidation(t *testing.T) {
	tests := []struct {
		name        string
		email       string
		expectedErr error
	}{
		{
			name:        "Empty email should fail",
			email:       "",
			expectedErr: domain.ErrEmptyEmail,
		},
		{
			name:        "Email without arroba should fail",
			email:       "invalid-email.com",
			expectedErr: domain.ErrArroaEmail,
		},
		{
			name:        "Valid email should pass",
			email:       "user@pleyades.io",
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := domain.User{Email: tt.email}
			err := u.ValidateEmail()
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("ValidateEmail() error = %v, expectedErr = %v", err, tt.expectedErr)
			}
		})
	}
}

func testNameValidation(t *testing.T) {
	tests := []struct {
		name        string
		userName    string
		expectedErr error
	}{
		{
			name:        "Empty name should fail",
			userName:    "",
			expectedErr: domain.ErrEmptyName,
		},
		{
			name:        "Name longer than 50 characters should fail",
			userName:    "EsteEsUnNombreExtremadamenteLargoQueSuperaLosCincuentaCaracteresPermitidosPorElDominio",
			expectedErr: domain.ErrLongName,
		},
		{
			name:        "Name with numbers should fail",
			userName:    "Usuario123",
			expectedErr: domain.ErrName,
		},
		{
			name:        "Valid alphabetic name should pass",
			userName:    "Arquitecto Cloud",
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := domain.User{Name: tt.userName}
			err := u.ValidateName()
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("ValidateName() error = %v, expectedErr = %v", err, tt.expectedErr)
			}
		})
	}
}

func testPasswordValidation(t *testing.T) {
	tests := []struct {
		name        string
		password    string
		expectedErr error
	}{
		{
			name:        "Empty password should fail",
			password:    "",
			expectedErr: domain.ErrEmptyPass,
		},
		{
			name:        "Password missing uppercase should fail",
			password:    "password123!",
			expectedErr: domain.ErrValidatePass,
		},
		{
			name:        "Password missing numbers should fail",
			password:    "Password!@#",
			expectedErr: domain.ErrValidatePass,
		},
		{
			name:        "Password missing special characters should fail",
			password:    "Password123",
			expectedErr: domain.ErrValidatePass,
		},
		{
			name:        "Valid strong password should pass",
			password:    "Password123!",
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := domain.User{Password: tt.password}
			err := u.ValidatePassword()
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("ValidatePassword() error = %v, expectedErr = %v", err, tt.expectedErr)
			}
		})
	}
}

func testMasterValidation(t *testing.T) {
	tests := []struct {
		name        string
		user        domain.User
		expectedErr error
	}{
		{
			name: "User with invalid email fails master validation",
			user: domain.User{
				Email:    "bad-email",
				Name:     "Cloud Dev",
				Password: "Password123!",
			},
			expectedErr: domain.ErrArroaEmail,
		},
		{
			name: "User with invalid name fails master validation",
			user: domain.User{
				Email:    "dev@pleyades.io",
				Name:     "Cloud123",
				Password: "Password123!",
			},
			expectedErr: domain.ErrName,
		},
		{
			name: "User with invalid password fails master validation",
			user: domain.User{
				Email:    "dev@pleyades.io",
				Name:     "Cloud Dev",
				Password: "weak",
			},
			expectedErr: domain.ErrValidatePass,
		},
		{
			name: "Valid user passes master validation",
			user: domain.User{
				Email:    "dev@pleyades.io",
				Name:     "Cloud Dev",
				Password: "Password123!",
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.user.Validate()
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("Validate() error = %v, expectedErr = %v", err, tt.expectedErr)
			}
		})
	}
}

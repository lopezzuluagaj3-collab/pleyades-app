package domain

import "errors"

var (
	ErrEmptyEmail         = errors.New("el correo electrónico no puede estar vacío")
	ErrArroaEmail         = errors.New("el correo electrónico debe tener '@'")
	ErrEmailAlreadyExists = errors.New("el correo electrónico ya está registrado")
	ErrName               = errors.New("el nombre solo puede contener letras")
	ErrEmptyName          = errors.New("el nombre no puede estar vacío")
	ErrLongName           = errors.New("el nombre no puede ser mayor a 50 caracteres")
	ErrEmptyPass          = errors.New("la contraseña no puede estar vacía")
	ErrUserNotFound       = errors.New("el usuario no fue encontrado")
	ErrValidatePass       = errors.New("la contraseña ingresada debe contener un carácter especial, números y una mayúscula")
)

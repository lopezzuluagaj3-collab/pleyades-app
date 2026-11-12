package domain

import "errors"

var (
	ErrEmptyEmail = errors.New("El Email no puede estar vacio")
	ErrArroaEmail = errors.New("El Email debe tener '@'")

	ErrName = errors.New("El nombre solo puede tener letras")
	ErrEmptyName = errors.New("El nombre no puede estar vacio")
	ErrLongName = errors.New("El nombre no puede ser mayor que 50 caracteres")

	ErrEmptyPass = errors.New("La contraseña no puede estar vacia")
	ErrValidatePass = errors.New("La contrasela ingresada debe tener un caracter especial, numeros y una mayuscula")
)
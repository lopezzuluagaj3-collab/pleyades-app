package domain

import (

	"strings"
	"time"
)



type User struct {
	UUID      string
	Email     string
	Password  string
	Name      string
	TenantID  string
	CreatedAt time.Time
	UpdatedAt time.Time
}


func (u User) ValidateEmail() error {
	if u.Email == "" {
		return ErrEmptyEmail
	}
	if !strings.Contains(u.Email, "@") {
		return ErrArroaEmail
	}
	return nil
}

func (u User) ValidateName() error {
	if u.Name == "" {
		return ErrEmptyName
	}
	if len(u.Name) > 50{
		return ErrLongName
	}
	if strings.ContainsAny(u.Name, "1234567890"){
		return ErrName
	}
	return nil
}

func (u User) ValidatePassword() error {
	if u.Password == "" {
		return ErrEmptyPass
	}
	if !strings.ContainsAny(u.Password, "!@#$%^&*()-_=+[]{}|;:,.<>?") ||
	   !strings.ContainsAny(u.Password, "1234567890") ||
	   !strings.ContainsAny(u.Password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return ErrValidatePass
	}
	return nil
}

func (u User) Validate() error {
	if err := u.ValidateEmail(); err != nil{
		return err
	}
	if err := u.ValidateName(); err != nil{
		return  err
	}
	if err := u.ValidatePassword(); err != nil{
		return  err
	}
	return nil
}
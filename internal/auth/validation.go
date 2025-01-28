package auth

import (
	"net/mail"
)

// IsValidEmail checks if the provided email address is valid
func IsValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

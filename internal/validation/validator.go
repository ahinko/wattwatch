// Package validation provides custom validators for the application
package validation

import (
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// Initialize registers all custom validators
func Initialize() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		err := v.RegisterValidation("nospaces", validateNoSpaces)
		if err != nil {
			panic(err)
		}
	}
}

// validateNoSpaces checks if a string contains non-space characters
func validateNoSpaces(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	return strings.TrimSpace(value) != ""
}

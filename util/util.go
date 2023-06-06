// Package util provides utility functions.
package util

import (
	"os"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/thalesfsp/validation"
)

//////
// Helpers.
//////

// GetValidator returns the validator instance. Use that, for example, to add
// custom validators.
func GetValidator() *validator.Validate {
	return validation.Get()
}

// GenerateUUID generates a RFC4122 UUID and DCE 1.1: Authentication and
// Security Services.
func GenerateUUID() string {
	return uuid.New().String()
}

// GetZeroControlChar returns the zero control character used to set the default
// value of a field. It's needed because if the tag is empty, there's no way to
// know if the user wants to set the field to the default value or if the field
// is not set. Set CONFIGURER_ZERO_CONTROL_CHAR to change the default value
// which is "zero".
func GetZeroControlChar() string {
	zeroControlChar := "zero"

	if os.Getenv("CONFIGURER_ZERO_CONTROL_CHAR") != "" {
		zeroControlChar = os.Getenv("CONFIGURER_ZERO_CONTROL_CHAR")
	}

	return zeroControlChar
}

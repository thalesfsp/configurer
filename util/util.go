// Package util provides utility functions.
package util

import (
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/thalesfsp/configurer/internal/validation"
)

//////
// Helpers.
//////

// Parse map.
func parseMap(s string) map[string]interface{} {
	if s == "" {
		return nil
	}

	m := make(map[string]interface{})

	for _, pair := range strings.Split(s, ",") {
		parts := strings.Split(pair, ":")

		if len(parts) == 2 {
			key := parts[0]
			value := parts[1]

			// Convert value to its type.
			if value == "true" {
				m[key] = true
			} else if value == "false" {
				m[key] = false
			} else if asInt, err := strconv.Atoi(value); err == nil {
				m[key] = asInt
			} else if asFloat, err := strconv.ParseFloat(value, 64); err == nil {
				m[key] = asFloat
			} else {
				m[key] = value
			}
		}
	}

	return m
}

// GetValidator returns the validator instance. Use that, for example, to add
// custom validators.
func GetValidator() *validator.Validate {
	return validation.Get()
}

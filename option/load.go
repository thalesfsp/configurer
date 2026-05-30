// Package option provides a set of options for the providers.
package option

import (
	"fmt"
	"strings"

	"github.com/iancoleman/strcase"
)

const (
	// Camel is the camel case.
	Camel = "camel"

	// Kebab is the kebab case.
	Kebab = "kebab"

	// Lower is the lower case.
	Lower = "lower"

	// Snake is the snake case.
	Snake = "snake"

	// Upper is the upper case.
	Upper = "upper"
)

// AllowedCases is the list of allowed cases.
var AllowedCases = []string{Camel, Kebab, Lower, Snake, Upper}

// LoadKeyFunc allows to specify loading options.
type LoadKeyFunc func(key string) string

// WithKeyPrefixer adds a prefix to the key.
func WithKeyPrefixer(prefix string) LoadKeyFunc {
	return func(key string) string {
		return fmt.Sprintf("%s%s", prefix, key)
	}
}

// WithKeySuffixer adds a suffix to the key.
func WithKeySuffixer(suffix string) LoadKeyFunc {
	return func(key string) string {
		return fmt.Sprintf("%s%s", key, suffix)
	}
}

// WithKeyCaser changes the case of the key using the `strcase` package.
//
// SEE: github.com/iancoleman/strcase.
func WithKeyCaser(caseType string) LoadKeyFunc {
	return func(key string) string {
		// Do nothing if the case type is not one of the allowed cases (handled
		// by the default branch below).
		switch caseType {
		case Snake:
			return strcase.ToSnake(key)
		case Camel:
			return strcase.ToCamel(key)
		case Kebab:
			return strcase.ToKebab(key)
		case Lower:
			return strings.ToLower(key)
		case Upper:
			return strings.ToUpper(key)
		default:
			return key
		}
	}
}

// WithKeyReplacer dynamically replaces the key.
func WithKeyReplacer(replacer LoadKeyFunc) LoadKeyFunc {
	return func(key string) string {
		return replacer(key)
	}
}

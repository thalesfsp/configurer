// Package option provides a set of options for the providers.
package option

import (
	"fmt"
	"strings"
)

// Casing is the type of casing.
type Casing string

const (
	// Lowercase lower case.
	Lowercase Casing = "lower"

	// Uppercase upper case.
	Uppercase Casing = "upper"
)

// KeyFunc allows to specify loading options.
type KeyFunc func(key string) string

// WithKeyPrefixer adds a prefix to the key.
func WithKeyPrefixer(prefix string) KeyFunc {
	return func(key string) string {
		return fmt.Sprintf("%s%s", prefix, key)
	}
}

// WithKeyCaser changes the case of the key. `caseType` can be "lower" or
// "upper".
func WithKeyCaser(caseType Casing) KeyFunc {
	return func(key string) string {
		switch caseType {
		case Lowercase:
			return strings.ToLower(key)
		case Uppercase:
			return strings.ToUpper(key)
		default:
			return key
		}
	}
}

// WithKeyReplacer dynamically replaces the key.
func WithKeyReplacer(replacer KeyFunc) KeyFunc {
	return func(key string) string {
		return replacer(key)
	}
}

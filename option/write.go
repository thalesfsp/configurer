// Package option provides a set of options for the providers.
package option

import (
	"github.com/thalesfsp/customerror"
)

// Write definition.
type Write struct {
	// Environment is the environment to be used.
	Environment string
}

// WriteFunc allows to specify loading options.
type WriteFunc func(o *Write) error

// WithEnvironment specifies the environment to be used.
func WithEnvironment(environment string) WriteFunc {
	return func(o *Write) error {
		if environment == "" {
			return customerror.NewInvalidError("environment, can't be empty")
		}

		o.Environment = environment

		return nil
	}
}

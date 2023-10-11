// Package option provides a set of options for the providers.
package option

import (
	"github.com/thalesfsp/customerror"
)

// Write definition.
type Write struct {
	// Environment is the environment to be used.
	Environment string

	// Target to write configuration. Default is "action".
	Target string

	// Variable indicates it's a variable instead of secret.
	Variable bool
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

// WithVariable specifies that it's a variable instead of secret to be used.
func WithVariable(variable bool) WriteFunc {
	return func(o *Write) error {
		o.Variable = variable

		return nil
	}
}

func WithTarget(target string) WriteFunc {
	return func(o *Write) error {
		if target == "" {
			return customerror.NewInvalidError("target, can't be empty")
		}

		o.Target = target

		return nil
	}
}

// Package noop provider. Runs a command with the environment variables from
// the environment variables.
package noop

import (
	"context"
	"os"
	"strings"

	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/provider"
	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/validation"
)

// Name of the provider.
const Name = "noop"

// NoOp provider definition.
type NoOp struct {
	*provider.Provider `json:"-" validate:"required"`
}

// Split splits the supplied environment variable value into a key/value pair.
//
// If v is of the form:
//   - KEY, returns (KEY, "")
//   - KEY=, returns (KEY, "")
//   - KEY=VALUE, returns (KEY, VALUE)
//
//nolint:gomnd
func split(v string) (string, string) {
	parts := strings.SplitN(v, "=", 2)

	var (
		key   string
		value string
	)

	switch len(parts) {
	case 2:
		value = parts[1]

		fallthrough
	case 1:
		key = parts[0]
	}

	return key, value
}

// Load retrieves the configuration, and exports it to the environment.
func (n *NoOp) Load(ctx context.Context, opts ...option.LoadKeyFunc) (map[string]string, error) {
	finalValues := make(map[string]string)

	// Should export secrets to the environment.
	for _, envVar := range os.Environ() {
		key, value := split(envVar)

		// Should allow to specify options.
		for _, opt := range opts {
			key = opt(key)
		}

		finalValue, err := provider.ExportToEnvVar(n, key, value)
		if err != nil {
			return nil, err
		}

		finalValues[key] = finalValue
	}

	return finalValues, nil
}

// Write stores a new secret.
//
// NOTE: Not all providers support writing secrets.
func (n *NoOp) Write(ctx context.Context, values map[string]interface{}, opts ...option.WriteFunc) error {
	// Ensure the secret values are not nil.
	if values == nil {
		return customerror.NewRequiredError("values")
	}

	// Process the options.
	var options option.Write

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return err
		}
	}

	return nil
}

// New sets up a new NoOp provider.
func New(override, rawValue bool) (provider.IProvider, error) {
	provider, err := provider.New("noop", override, rawValue)
	if err != nil {
		return nil, err
	}

	noop := &NoOp{
		Provider: provider,
	}

	if err := validation.Validate(noop); err != nil {
		return nil, err
	}

	return noop, nil
}

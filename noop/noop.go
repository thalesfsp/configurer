// Package noop provider. Runs a command with the environment variables from
// the environment variables.
package noop

import (
	"context"
	"os"
	"strings"

	"github.com/thalesfsp/configurer/internal/validation"
	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/provider"
)

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
func (n *NoOp) Load(ctx context.Context, opts ...option.KeyFunc) (map[string]string, error) {
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

// New sets up a new NoOp provider.
func New(override bool) (provider.IProvider, error) {
	provider, err := provider.New("noop", override)
	if err != nil {
		return nil, err
	}

	noop := &NoOp{
		Provider: provider,
	}

	if err := validation.ValidateStruct(noop); err != nil {
		return nil, err
	}

	return noop, nil
}

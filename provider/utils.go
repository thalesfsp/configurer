package provider

import (
	"fmt"
	"os"

	"github.com/thalesfsp/customerror"
)

// ExportToEnvVar exports the given key and value to the environment.
//
// NOTE: If override is `true`, it'll override existing environment variables!
func ExportToEnvVar(p IProvider, key string, value interface{}) (string, error) {
	fromOsEnvValue := os.Getenv(key)

	// Should export to the environment.
	finalValue := fmt.Sprintf("%v", value)

	// Should allow to don't overwrite existing environment variables.
	if fromOsEnvValue != "" && !p.GetOverride() {
		finalValue = fromOsEnvValue
	}

	if err := os.Setenv(key, finalValue); err != nil {
		return "", customerror.NewFailedToError(
			fmt.Sprintf("export %s env var", key),
			customerror.WithError(err),
		)
	}

	p.GetLogger().Tracelnf("Exported key %s with value %s", key, finalValue)

	return finalValue, nil
}

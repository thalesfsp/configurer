// Package dotenv provides a `.env` provider.
package dotenv

import (
	"context"

	"github.com/thalesfsp/configurer/internal/validation"
	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/provider"
	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/godotenv"
)

// DotEnv provider definition.
type DotEnv struct {
	*provider.Provider `json:"-" validate:"required"`

	// FilePaths is the list of file paths to load.
	FilePaths []string `json:"filePaths" validate:"required,gte=1"`
}

// Load retrieves the configuration, and exports it to the environment.
func (d *DotEnv) Load(ctx context.Context, opts ...option.KeyFunc) (map[string]string, error) {
	envMap, err := godotenv.Read(d.FilePaths...)
	if err != nil {
		return nil, customerror.NewFailedToError("read path", customerror.WithError(err))
	}

	finalValues := make(map[string]string)

	// Should export secrets to the environment.
	for key, value := range envMap {
		// Should allow to specify options.
		for _, opt := range opts {
			key = opt(key)
		}

		finalValue, err := provider.ExportToEnvVar(d, key, value)
		if err != nil {
			return nil, err
		}

		finalValues[key] = finalValue
	}

	return finalValues, nil
}

// New sets up a new DotEnv provider.
func New(override bool, files ...string) (provider.IProvider, error) {
	provider, err := provider.New("dotenv", override)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, customerror.NewRequiredError("filePaths")
	}

	dotEnv := &DotEnv{
		Provider: provider,

		FilePaths: files,
	}

	if err := validation.ValidateStruct(dotEnv); err != nil {
		return nil, err
	}

	return dotEnv, nil
}

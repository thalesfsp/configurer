// Package dotenv provides a `.env` provider.
package dotenv

import (
	"context"
	"fmt"

	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/provider"
	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/godotenv"
	"github.com/thalesfsp/validation"
)

// Name of the provider.
const Name = "env"

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

// Write stores a new secret.
//
// NOTE: Not all providers support writing secrets.
func (d *DotEnv) Write(ctx context.Context, values map[string]interface{}) error {
	// This operation is 1:1.
	if len(d.FilePaths) > 1 {
		return customerror.NewInvalidError("filePaths, for the Write operation only one file should be used")
	}

	convertedMap := make(map[string]string)

	// Merge the existing .env file content with the new values
	for key, value := range values {
		convertedMap[key] = fmt.Sprintf("%v", value)
	}

	// Write the updated values back to the .env file
	if err := godotenv.Write(convertedMap, d.FilePaths[0]); err != nil {
		return customerror.NewFailedToError("write path", customerror.WithError(err))
	}

	return nil
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

	if err := validation.Validate(dotEnv); err != nil {
		return nil, err
	}

	return dotEnv, nil
}

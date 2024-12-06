package config

import (
	"os"
	"path/filepath"

	"github.com/thalesfsp/customerror"
	"gopkg.in/yaml.v3"
)

const (
	fileperm = 0o600
	dirperm  = 0o755
)

// LoadConfiguration is a generic (T) function that handles reading, and writing
// configuration - currently only in YAML format. If the provided file does not
// exist, it will write the default configuration to the file, otherwise it will
// read the file and unmarshal it into the provided configuration struct.
// `defaultConfiguration` is required. `appName` is only required if no
// `filePath` is provided.
func LoadConfiguration[T any](
	filePath string,
	appName string,
	defaultConfiguration *T,
) (*T, error) {
	if defaultConfiguration == nil {
		return nil, customerror.NewRequiredError("defaultConfiguration")
	}

	// appName is required only if no filePath is provided.
	if filePath == "" && appName == "" {
		return nil, customerror.NewRequiredError("appName")
	}

	c := new(T)

	// If no filePath provided, use default path.
	if filePath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, customerror.NewFailedToError(
				"get home directory: %w",
				customerror.WithError(err),
			)
		}

		filePath = filepath.Join(
			home,
			".config",
			appName,
			"config.yaml",
		)

		// Create config directory if using home dir.
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, dirperm); err != nil {
			return nil, customerror.NewFailedToError(
				"create config directory: %w",
				customerror.WithError(err),
			)
		}
	}

	// Try to read config file.
	data, err := os.ReadFile(filePath)
	if err != nil {
		// File doesn't exist, proceed to write default config.
		if !os.IsNotExist(err) {
			return nil, customerror.NewFailedToError(
				"read config file: %w",
				customerror.WithError(err),
			)
		}

		// Convert default config to YAML.
		data, err := yaml.Marshal(defaultConfiguration)
		if err != nil {
			return nil, customerror.NewFailedToError(
				"marshal default config: %w",
				customerror.WithError(err),
			)
		}

		// Write yaml to file.
		if err := os.WriteFile(filePath, data, fileperm); err != nil {
			return nil, customerror.NewFailedToError(
				"write default config: %w",
				customerror.WithError(err),
			)
		}

		return defaultConfiguration, nil
	}

	if len(data) < 1 {
		// Convert default config to YAML.
		data, err := yaml.Marshal(defaultConfiguration)
		if err != nil {
			return nil, customerror.NewFailedToError("marshal default config: %w",
				customerror.WithError(err),
			)
		}

		// Write yaml to file.
		if err := os.WriteFile(filePath, data, fileperm); err != nil {
			return nil, customerror.NewFailedToError("write default config: %w",
				customerror.WithError(err),
			)
		}

		c = defaultConfiguration
	}

	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, customerror.NewFailedToError("parse config file: %w",
			customerror.WithError(err),
		)
	}

	return c, nil
}

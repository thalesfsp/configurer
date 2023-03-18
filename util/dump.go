package util

import (
	"encoding/json"
	"os"
	"reflect"

	"github.com/thalesfsp/configurer/internal/validation"
	"github.com/thalesfsp/customerror"
	"gopkg.in/yaml.v2"
)

//////
// Exported feature(s).
//////

// Dump the configuration from the environment variables into `v`. It:
// - Set values from environment variables using the `env` field tag.
// - Set default values using the `default` field tag.
// - Validating the values using the `validate` field tag.
//
// Order of operations:
// 1. Set default values
// 2. Set values from environment variables
// 3. Validate values
//
// NOTE: `v` must be a pointer to a struct.
// NOTE: It only sets default values for fields that are not set.
// NOTE: It'll set the value from env vars even if it's not empty (precedence).
func Dump(v any) error {
	if err := Process("default", v, func(v reflect.Value, field reflect.StructField, tag string) error {
		if err := setValueFromTag(v, field, tag, tag); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	if err := Process("env", v, func(v reflect.Value, field reflect.StructField, tag string) error {
		if err := setValueFromTag(v, field, tag, os.Getenv(tag)); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	// if err := SetEnv(v); err != nil {
	// 	return err
	// }

	if err := SetID(v); err != nil {
		return err
	}

	return validation.ValidateStruct(v)
}

// DumpToEnv dumps `finalValue` to a `.env` file.
func DumpToEnv(file *os.File, content map[string]string) error {
	for key, value := range content {
		if _, err := file.WriteString(key + "=" + value + "\n"); err != nil {
			return customerror.NewFailedToError("write to .env file", customerror.WithError(err))
		}
	}

	// Flush the file.
	if err := file.Sync(); err != nil {
		return customerror.NewFailedToError("flush "+file.Name()+" file", customerror.WithError(err))
	}

	return nil
}

// DumpToJSON dumps `finalValue` to a `configurer.json` file.
func DumpToJSON(file *os.File, content map[string]string) error {
	b, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return customerror.NewFailedToError("marshal final values to json", customerror.WithError(err))
	}

	if _, err := file.Write(b); err != nil {
		return customerror.NewFailedToError("write to configurer.json file", customerror.WithError(err))
	}

	// Flush the file.
	// Flush the file.
	if err := file.Sync(); err != nil {
		return customerror.NewFailedToError("flush "+file.Name()+" file", customerror.WithError(err))
	}

	return nil
}

// DumpToYAML dumps `finalValue` to a `configurer.yaml` file.
func DumpToYAML(file *os.File, content map[string]string) error {
	b, err := yaml.Marshal(content)
	if err != nil {
		return customerror.NewFailedToError("marshal final values to yaml", customerror.WithError(err))
	}

	if _, err := file.Write(b); err != nil {
		return customerror.NewFailedToError("write to configurer.yaml file", customerror.WithError(err))
	}

	// Flush the file.
	// Flush the file.
	if err := file.Sync(); err != nil {
		return customerror.NewFailedToError("flush "+file.Name()+" file", customerror.WithError(err))
	}

	return nil
}

package util

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/validation"
	"gopkg.in/yaml.v2"
)

//////
// Exported feature(s).
//////

// SetDefault For a given struct `v`, set default values based on the struct
// field tags (`default`).
//
// NOTE: It only sets default values for fields that are not set.
//
// NOTE: Like the built-in `json` tag, it'll ignore the field if it isn't
// exported, and if tag is set to `-`.
func SetDefault(v any) error {
	if err := process("default", v, func(v reflect.Value, field reflect.StructField, tag string) error {
		if err := setValueFromTag(v, field, tag, tag, false); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

// SetEnv For a given struct `v`, set values based on the struct field tags
// (`env`) and the environment variables.
//
// WARN: It will set the value of the field even if it's not empty.
//
// NOTE: Like the built-in `json` tag, it'll ignore the field if it isn't
// exported, and if tag is set to `-`.
func SetEnv(v any) error {
	if err := process("env", v, func(v reflect.Value, field reflect.StructField, tag string) error {
		valueFromEnvVar := os.Getenv(tag)

		// Should do nothing if the environment variable is not set.
		if valueFromEnvVar == "" {
			return nil
		}

		if err := setValueFromTag(v, field, tag, valueFromEnvVar, true); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

// SetID For a given struct `v`, set field with the specified ID type.
//
// NOTE: Currently only UUID is supported.
//
// NOTE: It only sets default values for fields that are not set.
//
// NOTE: Like the built-in `json` tag, it'll ignore the field if it isn't
// exported, and if tag is set to `-`.
func SetID(v any) error {
	if err := process("id", v, func(v reflect.Value, field reflect.StructField, tag string) error {
		var finalID string

		switch tag {
		case "uuid":
			finalID = GenerateUUID()
		default:
			return customerror.NewInvalidError("invalid ID type")
		}

		if err := setValueFromTag(v, field, tag, finalID, false); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

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
//
// NOTE: Like the built-in `json` tag, it'll ignore the field if it isn't
// exported, and if tag is set to `-`.
func Dump(v any) error {
	if err := SetDefault(v); err != nil {
		return err
	}

	if err := SetEnv(v); err != nil {
		return err
	}

	if err := SetID(v); err != nil {
		return err
	}

	return validation.Validate(v)
}

// Process `v`:
// - Set default values using the `default` field tag.
// - Set values from environment variables using the `env` field tag.
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
//
// NOTE: Like the built-in `json` tag, it'll ignore the field if it isn't
// exported, and if tag is set to `-`.
func Process(v any) error {
	return Dump(v)
}

// DumpToEnv dumps `finalValue` to a `.env` file.
func DumpToEnv(file *os.File, content map[string]string, rawValue bool) error {
	for key, value := range content {
		if rawValue {
			if _, err := fmt.Fprintf(file, "%s=%#v\n", key, value); err != nil {
				return customerror.NewFailedToError("write to .env file", customerror.WithError(err))
			}
		} else {
			if _, err := file.WriteString(key + "=" + value + "\n"); err != nil {
				return customerror.NewFailedToError("write to .env file", customerror.WithError(err))
			}
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
	if err := file.Sync(); err != nil {
		return customerror.NewFailedToError("flush "+file.Name()+" file", customerror.WithError(err))
	}

	return nil
}

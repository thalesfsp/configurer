// Package util provides utility functions.
package util

import (
	"os"
	"reflect"
	"strconv"

	"github.com/thalesfsp/configurer/internal/validation"
	"github.com/thalesfsp/customerror"
)

//////
// Helpers.
//////

// setDefault For a given struct `v`, set default values based on the struct
// field tags (`default`).
//
// NOTE: It only sets default values for fields that are not set.
func setDefault(v any) error {
	tagName := "default"

	if v == nil {
		return nil
	}

	val := reflect.ValueOf(v)

	// If it's an interface or a pointer, unwrap it.
	if val.Kind() == reflect.Ptr && val.Elem().Kind() == reflect.Struct {
		val = val.Elem()
	} else {
		return customerror.NewInvalidError("`v` must be a pointer to a struct")
	}

	valNumFields := val.NumField()

	for i := 0; i < valNumFields; i++ {
		field := val.Field(i)
		fieldKind := field.Kind()

		// Check if it's a pointer to a struct.
		if fieldKind == reflect.Ptr && field.Elem().Kind() == reflect.Struct {
			if field.CanInterface() {
				// Recurse using an interface of the field.
				err := setDefault(field.Interface())
				if err != nil {
					return err
				}
			}

			// Move onto the next field.
			continue
		}

		// Check if it's a struct value.
		if fieldKind == reflect.Struct {
			if field.CanAddr() && field.Addr().CanInterface() {
				// Recurse using an interface of the pointer value of the field.
				err := setDefault(field.Addr().Interface())
				if err != nil {
					return err
				}
			}

			// Move onto the next field.
			continue
		}

		//////
		// Start setting values here.
		//////

		// Check if it's a string or a pointer to a string.
		if fieldKind == reflect.String || (fieldKind == reflect.Ptr && field.Elem().Kind() == reflect.String) {
			typeField := val.Type().Field(i)

			// Get the field tag value.
			tag := typeField.Tag.Get(tagName)

			// Skip if tag is not defined or ignored.
			if tag == "" || tag == "-" {
				continue
			}

			// Set the string value to the sanitized string if it's allowed.
			// It should always be allowed at this point.
			if field.CanSet() {
				// Only set if the field is empty.
				if fieldKind == reflect.String && field.String() == "" {
					field.SetString(tag)
				}
			}

			continue
		}

		// Check if it's a bool or a pointer to a bool.
		if fieldKind == reflect.Bool || (fieldKind == reflect.Ptr && field.Elem().Kind() == reflect.Bool) {
			typeField := val.Type().Field(i)

			// Get the field tag value.
			tag := typeField.Tag.Get(tagName)

			// Skip if tag is not defined or ignored.
			if tag == "" || tag == "-" {
				continue
			}

			// Set the bool value to the sanitized bool if it's allowed.
			// It should always be allowed at this point.
			if field.CanSet() {
				// Only set if the field is empty.
				if fieldKind == reflect.Bool && !field.Bool() {
					field.SetBool(tag == "true")
				}
			}

			continue
		}

		// Check if it's an int or a pointer to an int.
		if fieldKind == reflect.Int || (fieldKind == reflect.Ptr && field.Elem().Kind() == reflect.Int) {
			typeField := val.Type().Field(i)

			// Get the field tag value.
			tag := typeField.Tag.Get(tagName)

			// Skip if tag is not defined or ignored.
			if tag == "" || tag == "-" {
				continue
			}

			// Set the int value to the sanitized int if it's allowed.
			// It should always be allowed at this point.
			if field.CanSet() {
				// Only set if the field is empty.
				if fieldKind == reflect.Int && field.Int() == 0 {
					if asInt, err := strconv.Atoi(tag); err == nil {
						field.SetInt(int64(asInt))
					}
				}
			}

			continue
		}

		// Check if it's a float64 or a pointer to a float64.
		if fieldKind == reflect.Float64 || (fieldKind == reflect.Ptr && field.Elem().Kind() == reflect.Float64) {
			typeField := val.Type().Field(i)

			// Get the field tag value.
			tag := typeField.Tag.Get(tagName)

			// Skip if tag is not defined or ignored.
			if tag == "" || tag == "-" {
				continue
			}

			// Set the float64 value to the sanitized float64 if it's allowed.
			// It should always be allowed at this point.
			if field.CanSet() {
				// Only set if the field is empty.
				if fieldKind == reflect.Float64 && field.Float() == 0 {
					if asFloat, err := strconv.ParseFloat(tag, 64); err == nil {
						field.SetFloat(asFloat)
					}
				}
			}

			continue
		}
	}

	return nil
}

// setEnv For a given struct `v`, set values based on the struct
// field tags (`env`) and the environment variables.
//
// NOTE: It will set the value of the field even if it's not empty.
func setEnv(v any) error {
	tagName := "env"

	if v == nil {
		return nil
	}

	val := reflect.ValueOf(v)

	// If it's an interface or a pointer, unwrap it.
	if val.Kind() == reflect.Ptr && val.Elem().Kind() == reflect.Struct {
		val = val.Elem()
	} else {
		return customerror.NewInvalidError("`v` must be a pointer to a struct")
	}

	valNumFields := val.NumField()

	for i := 0; i < valNumFields; i++ {
		field := val.Field(i)
		fieldKind := field.Kind()

		// Check if it's a pointer to a struct.
		if fieldKind == reflect.Ptr && field.Elem().Kind() == reflect.Struct {
			if field.CanInterface() {
				// Recurse using an interface of the field.
				err := setDefault(field.Interface())
				if err != nil {
					return err
				}
			}

			// Move onto the next field.
			continue
		}

		// Check if it's a struct value.
		if fieldKind == reflect.Struct {
			if field.CanAddr() && field.Addr().CanInterface() {
				// Recurse using an interface of the pointer value of the field.
				err := setDefault(field.Addr().Interface())
				if err != nil {
					return err
				}
			}

			// Move onto the next field.
			continue
		}

		//////
		// Start setting values here.
		//////

		// Check if it's a string or a pointer to a string.
		if fieldKind == reflect.String || (fieldKind == reflect.Ptr && field.Elem().Kind() == reflect.String) {
			typeField := val.Type().Field(i)

			// Get the field tag value.
			tag := typeField.Tag.Get(tagName)

			// Skip if tag is not defined or ignored.
			if tag == "" || tag == "-" {
				continue
			}

			if field.CanSet() {
				// Only set if the field is empty.
				if fieldKind == reflect.String {
					field.SetString(os.Getenv(tag))
				}
			}

			continue
		}

		// Check if it's a bool or a pointer to a bool.
		if fieldKind == reflect.Bool || (fieldKind == reflect.Ptr && field.Elem().Kind() == reflect.Bool) {
			typeField := val.Type().Field(i)

			// Get the field tag value.
			tag := typeField.Tag.Get(tagName)

			// Skip if tag is not defined or ignored.
			if tag == "" || tag == "-" {
				continue
			}

			// Set the bool value to the sanitized bool if it's allowed.
			// It should always be allowed at this point.
			if field.CanSet() {
				// Only set if the field is empty.
				if fieldKind == reflect.Bool {
					field.SetBool(os.Getenv(tag) == "true")
				}
			}

			continue
		}

		// Check if it's an int or a pointer to an int.
		if fieldKind == reflect.Int || (fieldKind == reflect.Ptr && field.Elem().Kind() == reflect.Int) {
			typeField := val.Type().Field(i)

			// Get the field tag value.
			tag := typeField.Tag.Get(tagName)

			// Skip if tag is not defined or ignored.
			if tag == "" || tag == "-" {
				continue
			}

			// Set the int value to the sanitized int if it's allowed.
			// It should always be allowed at this point.
			if field.CanSet() {
				// Only set if the field is empty.
				if fieldKind == reflect.Int {
					if asInt, err := strconv.Atoi(os.Getenv(tag)); err == nil {
						field.SetInt(int64(asInt))
					}
				}
			}

			continue
		}

		// Check if it's a float64 or a pointer to a float64.
		if fieldKind == reflect.Float64 || (fieldKind == reflect.Ptr && field.Elem().Kind() == reflect.Float64) {
			typeField := val.Type().Field(i)

			// Get the field tag value.
			tag := typeField.Tag.Get(tagName)

			// Skip if tag is not defined or ignored.
			if tag == "" || tag == "-" {
				continue
			}

			// Set the float64 value to the sanitized float64 if it's allowed.
			// It should always be allowed at this point.
			if field.CanSet() {
				// Only set if the field is empty.
				if fieldKind == reflect.Float64 {
					if asFloat, err := strconv.ParseFloat(os.Getenv(tag), 64); err == nil {
						field.SetFloat(asFloat)
					}
				}
			}

			continue
		}
	}

	return nil
}

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
	if err := setDefault(v); err != nil {
		return err
	}

	if err := setEnv(v); err != nil {
		return err
	}

	return validation.ValidateStruct(v)
}

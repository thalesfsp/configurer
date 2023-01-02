// Package util provides utility functions.
package util

import (
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/thalesfsp/configurer/internal/validation"
	"github.com/thalesfsp/customerror"
	"gopkg.in/yaml.v2"
)

//////
// Helpers.
//////

// Parse map.
func parseMap(s string) map[string]interface{} {
	if s == "" {
		return nil
	}

	m := make(map[string]interface{})
	for _, pair := range strings.Split(s, ",") {
		parts := strings.Split(pair, ":")

		if len(parts) == 2 {
			key := parts[0]
			value := parts[1]

			// Convert value to its type.
			if value == "true" {
				m[key] = true
			} else if value == "false" {
				m[key] = false
			} else if asInt, err := strconv.Atoi(value); err == nil {
				m[key] = asInt
			} else if asFloat, err := strconv.ParseFloat(value, 64); err == nil {
				m[key] = asFloat
			} else {
				m[key] = value
			}
		}
	}

	return m
}

// SetDefault For a given struct `v`, set default values based on the struct
// field tags (`default`).
//
// NOTE: It only sets default values for fields that are not set.
func SetDefault(v any) error {
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
				err := SetDefault(field.Interface())
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
				err := SetDefault(field.Addr().Interface())
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

		// Check if it's a duration or a pointer to a duration.
		if fieldKind == reflect.Int64 || (fieldKind == reflect.Ptr && field.Elem().Kind() == reflect.Int64) {
			typeField := val.Type().Field(i)

			// Get the field tag value.
			tag := typeField.Tag.Get(tagName)

			// Skip if tag is not defined or ignored.
			if tag == "" || tag == "-" {
				continue
			}

			// Set the duration value to the sanitized duration if it's allowed.
			// It should always be allowed at this point.
			if field.CanSet() {
				// Only set if the field is empty.
				if fieldKind == reflect.Int64 {
					if asDuration, err := time.ParseDuration(os.Getenv(tag)); err == nil {
						field.SetInt(int64(asDuration))
					}
				}
			}

			continue
		}

		// Check if it's a slice or a pointer to a slice.
		if fieldKind == reflect.Slice || (fieldKind == reflect.Ptr && field.Elem().Kind() == reflect.Slice) {
			typeField := val.Type().Field(i)

			// Get the field tag value.
			tag := typeField.Tag.Get(tagName)

			// Skip if tag is not defined or ignored.
			if tag == "" || tag == "-" {
				continue
			}

			// Set the slice value to the sanitized slice if it's allowed.
			// It should always be allowed at this point.
			if field.CanSet() {
				// Only set if the field is empty.
				if fieldKind == reflect.Slice {

					valuesAsString := strings.Split(tag, ",")
					valueAsString := valuesAsString[0]

					if _, err := strconv.Atoi(valueAsString); err == nil {
						// If valueAsString is a int, then we can assume that the slice is a slice of ints.
						// Iterate over valuesAsString and convert them to ints.
						var ints []int

						for _, v := range valuesAsString {
							if asInt, err := strconv.Atoi(v); err == nil {
								ints = append(ints, asInt)
							}
						}

						field.Set(reflect.ValueOf(ints))
					} else if _, err := strconv.ParseFloat(valueAsString, 64); err == nil {
						// If valueAsString is a float64, then we can assume that the slice is a slice of float64s.
						// Iterate over valuesAsString and convert them to float64s.
						var float64s []float64

						for _, v := range valuesAsString {
							if asFloat, err := strconv.ParseFloat(v, 64); err == nil {
								float64s = append(float64s, asFloat)
							}
						}

						field.Set(reflect.ValueOf(float64s))
					} else if _, err := strconv.ParseBool(valueAsString); err == nil {
						// If valueAsString is a bool, then we can assume that the slice is a slice of bools.
						// Iterate over valuesAsString and convert them to bools.
						var bools []bool

						for _, v := range valuesAsString {
							if asBool, err := strconv.ParseBool(v); err == nil {
								bools = append(bools, asBool)
							}
						}

						field.Set(reflect.ValueOf(bools))
					} else {
						field.Set(reflect.ValueOf(valuesAsString))
					}
				}
			}

			continue
		}

		// Check if it's a map or a pointer to a map.
		if fieldKind == reflect.Map || (fieldKind == reflect.Ptr && field.Elem().Kind() == reflect.Map) {
			typeField := val.Type().Field(i)

			// Get the field tag value.
			tag := typeField.Tag.Get(tagName)

			// Skip if tag is not defined or ignored.
			if tag == "" || tag == "-" {
				continue
			}

			// Set the map value to the sanitized map if it's allowed.
			// It should always be allowed at this point.
			if field.CanSet() {
				// Only set if the field is empty.
				if fieldKind == reflect.Map {
					if asMap := parseMap(tag); asMap != nil {
						field.Set(reflect.ValueOf(asMap))
					}
				}
			}

			continue
		}
	}

	return nil
}

// SetEnv For a given struct `v`, set values based on the struct
// field tags (`env`) and the environment variables.
//
// NOTE: It will set the value of the field even if it's not empty.
func SetEnv(v any) error {
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
				err := SetDefault(field.Interface())
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
				err := SetDefault(field.Addr().Interface())
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
					if os.Getenv(tag) != "" {
						field.SetString(os.Getenv(tag))
					}
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

		// Check if it's a duration or a pointer to a duration.
		if fieldKind == reflect.Int64 || (fieldKind == reflect.Ptr && field.Elem().Kind() == reflect.Int64) {
			typeField := val.Type().Field(i)

			// Get the field tag value.
			tag := typeField.Tag.Get(tagName)

			// Skip if tag is not defined or ignored.
			if tag == "" || tag == "-" {
				continue
			}

			// Set the duration value to the sanitized duration if it's allowed.
			// It should always be allowed at this point.
			if field.CanSet() {
				// Only set if the field is empty.
				if fieldKind == reflect.Int64 {
					if asDuration, err := time.ParseDuration(os.Getenv(tag)); err == nil {
						field.SetInt(int64(asDuration))
					}
				}
			}

			continue
		}

		// Check if it's a slice or a pointer to a slice.
		if fieldKind == reflect.Slice || (fieldKind == reflect.Ptr && field.Elem().Kind() == reflect.Slice) {
			typeField := val.Type().Field(i)

			// Get the field tag value.
			tag := typeField.Tag.Get(tagName)

			// Skip if tag is not defined or ignored.
			if tag == "" || tag == "-" {
				continue
			}

			// Set the slice value to the sanitized slice if it's allowed.
			// It should always be allowed at this point.
			if field.CanSet() {
				// Only set if the field is empty.
				if fieldKind == reflect.Slice {
					tagValueAsString := os.Getenv(tag)

					if tagValueAsString == "" {
						continue
					}

					valuesAsString := strings.Split(tagValueAsString, ",")
					valueAsString := valuesAsString[0]

					if _, err := strconv.Atoi(valueAsString); err == nil {
						// If valueAsString is a int, then we can assume that the slice is a slice of ints.
						// Iterate over valuesAsString and convert them to ints.
						var ints []int

						for _, v := range valuesAsString {
							if asInt, err := strconv.Atoi(v); err == nil {
								ints = append(ints, asInt)
							}
						}

						field.Set(reflect.ValueOf(ints))
					} else if _, err := strconv.ParseFloat(valueAsString, 64); err == nil {
						// If valueAsString is a float64, then we can assume that the slice is a slice of float64s.
						// Iterate over valuesAsString and convert them to float64s.
						var float64s []float64

						for _, v := range valuesAsString {
							if asFloat, err := strconv.ParseFloat(v, 64); err == nil {
								float64s = append(float64s, asFloat)
							}
						}

						field.Set(reflect.ValueOf(float64s))
					} else if _, err := strconv.ParseBool(valueAsString); err == nil {
						// If valueAsString is a bool, then we can assume that the slice is a slice of bools.
						// Iterate over valuesAsString and convert them to bools.
						var bools []bool

						for _, v := range valuesAsString {
							if asBool, err := strconv.ParseBool(v); err == nil {
								bools = append(bools, asBool)
							}
						}

						field.Set(reflect.ValueOf(bools))
					} else {
						field.Set(reflect.ValueOf(valuesAsString))
					}
				}
			}

			continue
		}

		// Check if it's a map or a pointer to a map.
		if fieldKind == reflect.Map || (fieldKind == reflect.Ptr && field.Elem().Kind() == reflect.Map) {
			typeField := val.Type().Field(i)

			// Get the field tag value.
			tag := typeField.Tag.Get(tagName)

			// Skip if tag is not defined or ignored.
			if tag == "" || tag == "-" {
				continue
			}

			// Set the map value to the sanitized map if it's allowed.
			// It should always be allowed at this point.
			if field.CanSet() {
				// Only set if the field is empty.
				if fieldKind == reflect.Map {
					if asMap := parseMap(os.Getenv(tag)); asMap != nil {
						field.Set(reflect.ValueOf(asMap))
					}
				}
			}

			continue
		}
	}

	return nil
}

// GetValidator returns the validator instance. Use that, for example, to add
// custom validators.
func GetValidator() *validator.Validate {
	return validation.Get()
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
	if err := SetDefault(v); err != nil {
		return err
	}

	if err := SetEnv(v); err != nil {
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

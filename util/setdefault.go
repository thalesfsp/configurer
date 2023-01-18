package util

import (
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/thalesfsp/customerror"
)

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
					if asDuration, err := time.ParseDuration(tag); err == nil {
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

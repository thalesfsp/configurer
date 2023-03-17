package util

import (
	"reflect"
	"time"

	"github.com/thalesfsp/customerror"
)

// SetDefault For a given struct `v`, set default values based on the struct
// field tags (`default`).
//
// NOTE: It only sets default values for fields that are not set.
func SetDefault(v any) error {
	tagName := "default"

	// Do nothing if `v` is nil.
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
		fieldType := field.Type()

		// Check if the field is nil and can be set.
		if field.Kind() == reflect.Ptr && field.IsNil() && field.CanSet() {
			// Set the field to its zero value.
			field.Set(reflect.New(fieldType.Elem()))

			// Recurse using an interface of the field.
			if err := SetDefault(field.Interface()); err != nil {
				return err
			}
		}

		// Get the field tag value.
		typeField := val.Type().Field(i)
		tag := typeField.Tag.Get(tagName)

		// Check if it's a pointer to a struct.
		if fieldKind == reflect.Ptr && field.Elem().Kind() == reflect.Struct {
			if field.CanInterface() {
				// Recurse using an interface of the field.
				if err := SetDefault(field.Interface()); err != nil {
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
				if err := SetDefault(field.Addr().Interface()); err != nil {
					return err
				}

				// Check if it's a time.Time.
				if fieldType == reflect.TypeOf(time.Time{}) {
					setTimeValue(field, tag, tag)
				}
			}
		}

		switch fieldKind {
		// Check if it's a string.
		case reflect.String:
			if field.String() == "" {
				setStringValue(field, tag, tag)
			}

		// Check if it's a bool.
		case reflect.Bool:
			if !field.Bool() {
				setBoolValue(field, tag, tag)
			}

		// Check if it's an int.
		case reflect.Int:
			if field.Int() == 0 {
				setIntValue(field, tag, tag)
			}

		// Check if it's a float64.
		case reflect.Float64:
			if field.Float() == 0 {
				setFloat64Value(field, tag, tag)
			}

		// Check if it's a time.Duration.
		case reflect.Int64:
			if fieldType == reflect.TypeOf(time.Duration(0)) {
				setDurationValue(field, tag, tag)
			}

		// Check if it's a slice.
		case reflect.Slice:
			if field.IsNil() {
				setSliceValue(field, tag, tag)
			}

		// Check if it's a map.
		case reflect.Map:
			if field.IsNil() {
				setMapValue(field, tag, tag)
			}
		}
	}

	return nil
}

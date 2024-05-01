package util

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/thalesfsp/customerror"
)

// Func is the callback function type.
type Func func(v reflect.Value, field reflect.StructField, tag string) error

func parseIntValue(v reflect.Value, str string) error {
	// If the type of the field is time.Duration, we need to parse it as a duration.
	if v.Type() == reflect.TypeOf(time.Duration(0)) {
		return parseDurationValue(v, str)
	}

	if str == GetZeroControlChar() {
		// If str is the "zero" control char, set the value to field's zero
		// value.
		v.Set(reflect.Zero(v.Type()))

		return nil
	}

	i, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return err
	}

	v.SetInt(i)

	return nil
}

func parseUintValue(v reflect.Value, str string) error {
	if str == GetZeroControlChar() {
		// If str is the "zero" control char, set the value to field's zero
		// value.
		v.Set(reflect.Zero(v.Type()))

		return nil
	}

	u, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		return err
	}

	v.SetUint(u)

	return nil
}

func parseFloatValue(v reflect.Value, str string) error {
	if str == GetZeroControlChar() {
		// If str is the "zero" control char, set the value to field's zero
		// value.
		v.Set(reflect.Zero(v.Type()))

		return nil
	}

	f, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return err
	}

	v.SetFloat(f)

	return nil
}

func parseBoolValue(v reflect.Value, str string) error {
	if str == GetZeroControlChar() {
		// If str is the "zero" control char, set the value to field's zero
		// value.
		v.Set(reflect.Zero(v.Type()))

		return nil
	}

	b, err := strconv.ParseBool(str)
	if err != nil {
		return err
	}

	v.SetBool(b)

	return nil
}

func parseStringValue(v reflect.Value, str string) {
	if str == GetZeroControlChar() {
		// If str is the "zero" control char, set the value to field's zero
		// value.
		v.Set(reflect.Zero(v.Type()))

		return
	}

	v.SetString(str)
}

func parseTimeValue(v reflect.Value, str string) error {
	if str == GetZeroControlChar() {
		// If str is the "zero" control char, set the value to field's zero
		// value.
		v.Set(reflect.Zero(v.Type()))

		return nil
	}

	var value time.Time

	if str == "now" {
		value = time.Now()
	} else {
		v, err := dateparse.ParseAny(str)
		if err != nil {
			return err
		}

		value = v
	}

	v.Set(reflect.ValueOf(value))

	return nil
}

func parseDurationValue(v reflect.Value, str string) error {
	if str == GetZeroControlChar() {
		// If str is the "zero" control char, set the value to field's zero
		// value.
		v.Set(reflect.Zero(v.Type()))

		return nil
	}

	d, err := time.ParseDuration(str)
	if err != nil {
		return err
	}

	v.Set(reflect.ValueOf(d))

	return nil
}

func parseSingleValue(v reflect.Value, t reflect.Type, str string) error {
	switch t.Kind() {
	case reflect.String:
		parseStringValue(v, str)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return parseIntValue(v, str)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return parseUintValue(v, str)
	case reflect.Float32, reflect.Float64:
		return parseFloatValue(v, str)
	case reflect.Bool:
		return parseBoolValue(v, str)
	case reflect.Struct:
		if t == reflect.TypeOf(time.Time{}) {
			return parseTimeValue(v, str)
		}

		return fmt.Errorf("unsupported struct type: %s", t)
	default:
		return fmt.Errorf("unsupported type: %s", t)
	}

	return nil
}

func parseMap(valueType reflect.Type, tag string) (interface{}, error) {
	if tag == GetZeroControlChar() {
		return reflect.MakeMap(valueType).Interface(), nil
	}

	tagPairs := strings.Split(tag, ",")

	mapValue := reflect.MakeMap(valueType)

	for _, pair := range tagPairs {
		kv := strings.Split(pair, ":")

		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid map key-value pair: %s", pair)
		}

		key, err := parseValue(valueType.Key(), kv[0])
		if err != nil {
			return nil, fmt.Errorf("failed to parse key %s: %w", kv[0], err)
		}

		if valueType.Elem().Kind() == reflect.Interface {
			value, err := parseValueForInterface(kv[1])
			if err != nil {
				return nil, fmt.Errorf("failed to parse value %s: %w", kv[1], err)
			}

			// Convert the value to the correct type.
			value = reflect.ValueOf(value).Convert(valueType.Elem()).Interface()

			mapValue.SetMapIndex(reflect.ValueOf(key), reflect.ValueOf(value))
		} else {
			value, err := parseValue(valueType.Elem(), kv[1])
			if err != nil {
				return nil, fmt.Errorf("failed to parse value %s: %w", kv[1], err)
			}

			mapValue.SetMapIndex(reflect.ValueOf(key), reflect.ValueOf(value))
		}
	}

	return mapValue.Interface(), nil
}

//nolint:unparam
func parseValueForInterface(str string) (interface{}, error) {
	if str != "0" {
		// Check for time.Duration
		if d, err := time.ParseDuration(str); err == nil {
			return d, nil
		}
	}

	// Check for int first
	if i, err := strconv.Atoi(str); err == nil {
		return i, nil
	}

	// Check for int64 first, as ParseInt handles integers correctly
	if i, err := strconv.ParseInt(str, 10, 64); err == nil {
		return i, nil
	}

	// Check for uint64 next
	if u, err := strconv.ParseUint(str, 10, 64); err == nil {
		return u, nil
	}

	// Check for float64 next
	if f, err := strconv.ParseFloat(str, 64); err == nil {
		return f, nil
	}

	// Check for bool next
	if b, err := strconv.ParseBool(str); err == nil {
		return b, nil
	}

	// Check for time.Time
	if t, err := dateparse.ParseAny(str); err == nil {
		return t, nil
	}

	// If no other type matched, assume it's a string
	return str, nil
}

func parseSlice(v reflect.Value, elemType reflect.Type, tag string) error {
	if tag == GetZeroControlChar() {
		// If str is the "zero" control char, set the value to field's zero
		// value.
		v.Set(reflect.MakeSlice(v.Type(), 0, 0))

		return nil
	}

	tagElements := strings.Split(tag, ",")

	slice := reflect.MakeSlice(reflect.SliceOf(elemType), len(tagElements), len(tagElements))

	for i, elemStr := range tagElements {
		elem := slice.Index(i)

		if err := parseSingleValue(elem, elemType, elemStr); err != nil {
			return fmt.Errorf("failed to parse slice element: %w", err)
		}
	}

	v.Set(slice)

	return nil
}

func parseValue(t reflect.Type, str string) (interface{}, error) {
	var v reflect.Value

	switch t.Kind() {
	case reflect.Bool:
		b, err := strconv.ParseBool(str)
		if err != nil {
			return nil, err
		}

		v = reflect.ValueOf(b)

	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return nil, err
		}

		v = reflect.ValueOf(f).Convert(t)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if t == reflect.TypeOf(time.Duration(0)) {
			d, err := time.ParseDuration(str)
			if err != nil {
				return nil, err
			}

			v = reflect.ValueOf(d)
		} else {
			i, err := strconv.ParseInt(str, 10, 64)
			if err != nil {
				return nil, err
			}

			v = reflect.ValueOf(i).Convert(t)
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(str, 10, 64)
		if err != nil {
			return nil, err
		}

		v = reflect.ValueOf(u).Convert(t)

	case reflect.Struct:
		if t == reflect.TypeOf(time.Time{}) {
			value, err := dateparse.ParseAny(str)
			if err != nil {
				return nil, err
			}

			v = reflect.ValueOf(value)
		} else {
			return nil, fmt.Errorf("unsupported struct type: %s", t)
		}

	case reflect.String:
		v = reflect.ValueOf(str)

	default:
		return nil, fmt.Errorf("unsupported type: %s", t)
	}

	return v.Interface(), nil
}

//nolint:unparam
func setValueFromTag(v reflect.Value, field reflect.StructField, tag string, content string, override bool) error {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			if v.Type().Elem().Kind() == reflect.Struct {
				return nil // Skip nil pointers to structs
			}

			v.Set(reflect.New(v.Type().Elem()))
		}

		v = v.Elem()
	}

	if tag == "" {
		return nil
	}

	if !v.CanSet() {
		return errors.New("cannot set value")
	}

	// Should set the value if override is true or the value is zero.
	if !override && (!v.IsZero() || (v.Kind() == reflect.Ptr && !v.IsNil())) {
		return nil
	}

	finalContent := tag

	if content != "" {
		finalContent = content
	}

	switch v.Kind() {
	case reflect.String:
		parseStringValue(v, finalContent)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return parseIntValue(v, finalContent)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return parseUintValue(v, finalContent)
	case reflect.Float32, reflect.Float64:
		return parseFloatValue(v, finalContent)
	case reflect.Bool:
		return parseBoolValue(v, finalContent)
	case reflect.Slice, reflect.Array:
		return parseSlice(v, v.Type().Elem(), finalContent)
	case reflect.Map:
		m, err := parseMap(v.Type(), finalContent)
		if err != nil {
			return err
		}

		v.Set(reflect.ValueOf(m))
	case reflect.Struct:
		if v.Type() == reflect.TypeOf(time.Time{}) {
			return parseTimeValue(v, finalContent)
		}
	default:
		return errors.New("unsupported type")
	}

	return nil
}

// process a struct and its fields. Use it to build your own custom tag handler.
//
//nolint:errcheck
func process(tagName string, s any, cb Func) error {
	v := reflect.ValueOf(s)

	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return customerror.NewInvalidError("`s`, it must be set, and be a pointer")
	}

	v = v.Elem()

	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields like `json` tag.
		if field.PkgPath != "" {
			continue
		}

		value := v.Field(i)

		customtag := field.Tag.Get(tagName)

		// Skip ignored fields like `json:"-"`.
		if customtag == "-" {
			continue
		}

		if customtag != "" {
			cb(value, field, customtag) // Pass the value directly.
		}

		if value.Kind() == reflect.Ptr && !value.IsNil() {
			elem := value.Elem()

			if elem.Kind() == reflect.Struct {
				if err := process(tagName, value.Interface(), cb); err != nil {
					return err
				}
			}
		} else if value.Kind() == reflect.Struct {
			if err := process(tagName, value.Addr().Interface(), cb); err != nil {
				return err
			}
		}
	}

	return nil
}

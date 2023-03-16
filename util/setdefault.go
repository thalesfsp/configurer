package util

import (
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
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
					setTimeValue(field, tag)
				}
			}
		}

		switch fieldKind {

		// Check if it's a string.
		case reflect.String:
			if field.String() == "" {
				setStringValue(field, tag)
			}

		// Check if it's a bool.
		case reflect.Bool:
			if !field.Bool() {
				setBoolValue(field, tag)
			}

		// Check if it's an int.
		case reflect.Int:
			if field.Int() == 0 {
				setIntValue(field, tag)
			}

		// Check if it's a float64.
		case reflect.Float64:
			if field.Float() == 0 {
				setFloat64Value(field, tag)
			}

		// Check if it's a time.Duration.
		case reflect.Int64:
			if fieldType == reflect.TypeOf(time.Duration(0)) {
				setDurationValue(field, tag)
			}

		// Check if it's a slice.
		case reflect.Slice:
			if field.IsNil() {
				setSliceValue(field, tag)
			}

		// Check if it's a map.
		case reflect.Map:
			if field.IsNil() {
				setMapValue(field, tag)
			}
		}
	}

	return nil
}

func setStringValue(field reflect.Value, tag string) {
	if field.CanSet() {
		if tag == "" || tag == "-" {
			field.SetString("")
		} else {
			field.SetString(tag)
		}
	}
}

func setBoolValue(field reflect.Value, tag string) {
	if field.CanSet() {
		if tag == "" || tag == "-" {
			field.SetBool(false)
		} else {
			field.SetBool(tag == "true")
		}
	}
}

func setIntValue(field reflect.Value, tag string) {
	if field.CanSet() {
		if tag == "" || tag == "-" {
			field.SetInt(0)
		} else if asInt, err := strconv.Atoi(tag); err == nil {
			field.SetInt(int64(asInt))
		}
	}
}

func setFloat64Value(field reflect.Value, tag string) {
	if field.CanSet() {
		if tag == "" || tag == "-" {
			field.SetFloat(0)
		} else if asFloat, err := strconv.ParseFloat(tag, 64); err == nil {
			field.SetFloat(asFloat)
		}
	}
}

func setDurationValue(field reflect.Value, tag string) {
	if field.CanSet() {
		if tag == "" || tag == "-" {
			field.SetInt(0)
		} else if asDuration, err := time.ParseDuration(tag); err == nil {
			field.SetInt(int64(asDuration))
		}
	}
}

func setSliceValue(field reflect.Value, tag string) {
	if field.CanSet() {
		if tag == "" || tag == "-" {
			field.Set(reflect.MakeSlice(field.Type(), 0, 0))
		} else {
			values := strings.Split(tag, ",")
			value := values[0]

			switch {
			case isInt(value):
				field.Set(intSlice(values))
			case isFloat64(value):
				field.Set(float64Slice(values))
			case isBool(value):
				field.Set(boolSlice(values))
			case isTime(value):
				field.Set(timeSlice(values))
			case isDuration(value):
				field.Set(durationSlice(values))
			default:
				field.Set(reflect.ValueOf(values))
			}
		}
	}
}

func setMapValue(field reflect.Value, tag string) {
	if field.CanSet() {
		if tag == "" || tag == "-" {
			field.Set(reflect.MakeMap(field.Type()))
		} else if asMap := parseMap(tag); asMap != nil {
			field.Set(reflect.ValueOf(asMap))
		}
	}
}

func setTimeValue(field reflect.Value, tag string) {
	if field.CanSet() {
		if tag == "" || tag == "-" {
			field.Set(reflect.ValueOf(time.Now()))
		} else {
			// Normal parse.  Equivalent Timezone rules as time.Parse()
			t, err := dateparse.ParseAny(tag)
			if err != nil {
				panic(err)
			}

			field.Set(reflect.ValueOf(t))
		}
	}
}

func isInt(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func isFloat64(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

func isBool(s string) bool {
	_, err := strconv.ParseBool(s)
	return err == nil
}

func intSlice(strings []string) reflect.Value {
	ints := make([]int, len(strings))
	for i, v := range strings {
		ints[i], _ = strconv.Atoi(v)
	}
	return reflect.ValueOf(ints)
}

func float64Slice(strings []string) reflect.Value {
	float64s := make([]float64, len(strings))
	for i, v := range strings {
		float64s[i], _ = strconv.ParseFloat(v, 64)
	}
	return reflect.ValueOf(float64s)
}

func boolSlice(strings []string) reflect.Value {
	bools := make([]bool, len(strings))
	for i, v := range strings {
		bools[i], _ = strconv.ParseBool(v)
	}
	return reflect.ValueOf(bools)
}

func isTime(value string) bool {
	_, err := dateparse.ParseLocal(value)
	return err == nil
}

func isDuration(value string) bool {
	_, err := time.ParseDuration(value)
	return err == nil
}

func timeSlice(values []string) reflect.Value {
	var times []time.Time
	for _, v := range values {
		parsedTime, err := dateparse.ParseLocal(v)
		if err == nil {
			times = append(times, parsedTime)
		}
	}
	return reflect.ValueOf(times)
}

func durationSlice(values []string) reflect.Value {
	var durations []time.Duration
	for _, v := range values {
		parsedDuration, err := time.ParseDuration(v)
		if err == nil {
			durations = append(durations, parsedDuration)
		}
	}
	return reflect.ValueOf(durations)
}

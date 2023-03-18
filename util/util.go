// Package util provides utility functions.
package util

import (
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/thalesfsp/configurer/internal/validation"
)

//////
// Helpers.
//////

// GetValidator returns the validator instance. Use that, for example, to add
// custom validators.
func GetValidator() *validator.Validate {
	return validation.Get()
}

// GenerateUUID generates a RFC4122 UUID and DCE 1.1: Authentication and
// Security Services.
func GenerateUUID() string {
	return uuid.New().String()
}

func setStringValue(field reflect.Value, tag string, content string) {
	if field.CanSet() {
		finalContent := tag

		if content != "" {
			finalContent = content
		}

		if finalContent == "" || finalContent == "-" {
			field.SetString("")
		} else {
			field.SetString(finalContent)
		}
	}
}

func setBoolValue(field reflect.Value, tag string, content string) {
	if field.CanSet() {
		finalContent := tag

		if content != "" {
			finalContent = content
		}

		if finalContent == "" || finalContent == "-" {
			field.SetBool(false)
		} else {
			field.SetBool(finalContent == "true")
		}
	}
}

func setIntValue(field reflect.Value, tag string, content string) {
	if field.CanSet() {
		finalContent := tag

		if content != "" {
			finalContent = content
		}

		if finalContent == "" || finalContent == "-" {
			field.SetInt(0)
		} else if asInt, err := strconv.Atoi(finalContent); err == nil {
			field.SetInt(int64(asInt))
		}
	}
}

func setFloat64Value(field reflect.Value, tag string, content string) {
	if field.CanSet() {
		finalContent := tag

		if content != "" {
			finalContent = content
		}

		if finalContent == "" || finalContent == "-" {
			field.SetFloat(0)
		} else if asFloat, err := strconv.ParseFloat(finalContent, 64); err == nil {
			field.SetFloat(asFloat)
		}
	}
}

func setDurationValue(field reflect.Value, tag string, content string) {
	if field.CanSet() {
		finalContent := tag

		if content != "" {
			finalContent = content
		}

		if finalContent == "" || finalContent == "-" {
			field.SetInt(0)
		} else if asDuration, err := time.ParseDuration(finalContent); err == nil {
			field.SetInt(int64(asDuration))
		}
	}
}

func setSliceValue(field reflect.Value, tag string, content string) {
	if field.CanSet() {
		finalContent := tag

		if content != "" {
			finalContent = content
		}

		if finalContent == "" || finalContent == "-" {
			field.Set(reflect.MakeSlice(field.Type(), 0, 0))
		} else {
			values := strings.Split(finalContent, ",")
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

// It should be able to convert a field tag such as: "name:John,surname:Peter"
// to a map[string]string and "age:1,year:1983" to a map[string]int and so on.
func setMapValue(field reflect.Value, tag string, content string) {
	if field.CanSet() {
		finalContent := tag

		if content != "" {
			finalContent = content
		}

		if finalContent == "" || finalContent == "-" {
			field.Set(reflect.MakeMap(field.Type()))
		} else {
			// Split the final content into pairs
			pairs := strings.Split(finalContent, ",")

			// Create a new map of the same type as the field
			mapType := field.Type()
			newMap := reflect.MakeMap(mapType)

			// Iterate through the pairs and add them to the new map
			for _, pair := range pairs {
				kv := strings.Split(pair, ":")

				// If the pair is not in the format key:value, skip it.
				if len(kv) != 2 {
					continue
				}

				key := reflect.ValueOf(kv[0])
				valueStr := kv[1]

				// Depending on the type of the map's value, parse and set the value.
				// Cast if necessary.
				valueType := mapType.Elem()

				// If valueType is an interface, we need to check the value of the
				// tag to determine the type of the value.
				if valueType.Kind() == reflect.Interface {
					switch {
					case isInt(valueStr):
						valueType = reflect.TypeOf(0)
					case isFloat64(valueStr):
						valueType = reflect.TypeOf(0.0)
					case isBool(valueStr):
						valueType = reflect.TypeOf(false)
					case isTime(valueStr):
						valueType = reflect.TypeOf(time.Time{})
					case isDuration(valueStr):
						valueType = reflect.TypeOf(time.Duration(0))
					default:
						valueType = reflect.TypeOf("")
					}
				}

				switch valueType.Kind() {
				case reflect.Bool:
					value := reflect.ValueOf(valueStr == "true")
					newMap.SetMapIndex(key, value)
				case reflect.Int:
					value, _ := strconv.Atoi(valueStr)
					newMap.SetMapIndex(key, reflect.ValueOf(value))
				case reflect.Int64:
					// Need to check if the value is a duration.
					if valueType == reflect.TypeOf(time.Duration(0)) {
						value, _ := time.ParseDuration(valueStr)
						newMap.SetMapIndex(key, reflect.ValueOf(value))
					} else {
						value, _ := strconv.ParseInt(valueStr, 10, 64)
						newMap.SetMapIndex(key, reflect.ValueOf(value))
					}
				case reflect.Float64:
					value, _ := strconv.ParseFloat(valueStr, 64)
					newMap.SetMapIndex(key, reflect.ValueOf(value))
				case reflect.String:
					newMap.SetMapIndex(key, reflect.ValueOf(valueStr))
				case reflect.Struct:
					if valueType == reflect.TypeOf(time.Time{}) {
						value, _ := dateparse.ParseAny(valueStr)
						newMap.SetMapIndex(key, reflect.ValueOf(value))
					}
				case reflect.Slice:
					if valueType.Elem().Kind() == reflect.Int {
						values := strings.Split(valueStr, ";")
						newMap.SetMapIndex(key, intSlice(values))
					}
				}
			}

			// Set the new map to the field
			field.Set(newMap)
		}
	}
}

func setTimeValue(field reflect.Value, tag string, content string) {
	if field.CanSet() {
		finalContent := tag

		if content != "" {
			finalContent = content
		}

		if finalContent == "" || finalContent == "-" {
			field.Set(reflect.ValueOf(time.Now()))
		} else {
			t, err := dateparse.ParseAny(finalContent)
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

// GetZeroControlChar returns the zero control character used to set the default
// value of a field. It's needed because if the tag is empty, there's no way to
// know if the user wants to set the field to the default value or if the field
// is not set. Set CONFIGURER_ZERO_CONTROL_CHAR to change the default value
// which is "zero".
func GetZeroControlChar() string {
	zeroControlChar := "zero"

	if os.Getenv("CONFIGURER_ZERO_CONTROL_CHAR") != "" {
		zeroControlChar = os.Getenv("CONFIGURER_ZERO_CONTROL_CHAR")
	}

	return zeroControlChar
}

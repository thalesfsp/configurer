package util

// SetEnv For a given struct `v`, set values based on the struct field tags
// (`env`) and the environment variables.
//
// WARN: It will set the value of the field even if it's not empty.
// func SetEnv(v any) error {
// 	tagName := "env"

// 	// Do nothing if `v` is nil.
// 	if v == nil {
// 		return nil
// 	}

// 	val := reflect.ValueOf(v)

// 	// If it's an interface or a pointer, unwrap it.
// 	if val.Kind() == reflect.Ptr && val.Elem().Kind() == reflect.Struct {
// 		val = val.Elem()
// 	} else {
// 		return customerror.NewInvalidError("`v` must be a pointer to a struct")
// 	}

// 	valNumFields := val.NumField()

// 	for i := 0; i < valNumFields; i++ {
// 		field := val.Field(i)
// 		fieldKind := field.Kind()
// 		fieldType := field.Type()

// 		// Check if the field is nil and can be set.
// 		if field.Kind() == reflect.Ptr && field.IsNil() && field.CanSet() {
// 			// Set the field to its zero value.
// 			field.Set(reflect.New(fieldType.Elem()))

// 			// Recurse using an interface of the field.
// 			if err := SetDefault(field.Interface()); err != nil {
// 				return err
// 			}
// 		}

// 		// Get the field tag value.
// 		typeField := val.Type().Field(i)

// 		// Check if tag is present
// 		if _, ok := typeField.Tag.Lookup(tagName); !ok {
// 			continue
// 		}

// 		tag := typeField.Tag.Get(tagName)

// 		// Check if it's a pointer to a struct.
// 		if fieldKind == reflect.Ptr && field.Elem().Kind() == reflect.Struct {
// 			if field.CanInterface() {
// 				// Recurse using an interface of the field.
// 				if err := SetDefault(field.Interface()); err != nil {
// 					return err
// 				}
// 			}

// 			// Move onto the next field.
// 			continue
// 		}

// 		// If no value is retrieved from the env var, move onto the next field.
// 		if os.Getenv(tag) == "" {
// 			continue
// 		}

// 		// Check if it's a struct value.
// 		if fieldKind == reflect.Struct {
// 			if field.CanAddr() && field.Addr().CanInterface() {
// 				// Recurse using an interface of the pointer value of the field.
// 				if err := SetDefault(field.Addr().Interface()); err != nil {
// 					return err
// 				}

// 				// Check if it's a time.Time.
// 				if fieldType == reflect.TypeOf(time.Time{}) {
// 					setTimeValue(field, tag, os.Getenv(tag))
// 				}
// 			}
// 		}

// 		switch fieldKind {
// 		// Check if it's a string.
// 		case reflect.String:
// 			setStringValue(field, tag, os.Getenv(tag))

// 		// Check if it's a bool.
// 		case reflect.Bool:
// 			setBoolValue(field, tag, os.Getenv(tag))

// 		// Check if it's an int.
// 		case reflect.Int:
// 			setIntValue(field, tag, os.Getenv(tag))

// 		// Check if it's a float64.
// 		case reflect.Float64:
// 			setFloat64Value(field, tag, os.Getenv(tag))

// 		// Check if it's a time.Duration.
// 		case reflect.Int64:
// 			setDurationValue(field, tag, os.Getenv(tag))

// 		// Check if it's a slice.
// 		case reflect.Slice:
// 			setSliceValue(field, tag, os.Getenv(tag))

// 		// Check if it's a map.
// 		case reflect.Map:
// 			setMapValue(field, tag, os.Getenv(tag))
// 		}
// 	}

// 	return nil
// }

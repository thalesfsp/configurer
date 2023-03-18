package util

// SetID For a given struct `v`, set field with a random generated ID.
//
// NOTE: It only sets default values for fields that are not set.
// func SetID(v any) error {
// 	tagName := "id"

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

// 		// Check if it's a pointer to a struct.
// 		if fieldKind == reflect.Ptr && field.Elem().Kind() == reflect.Struct {
// 			if field.CanInterface() {
// 				// Recurse using an interface of the field.
// 				err := SetDefault(field.Interface())
// 				if err != nil {
// 					return err
// 				}
// 			}

// 			// Move onto the next field.
// 			continue
// 		}

// 		// Check if it's a struct value.
// 		if fieldKind == reflect.Struct {
// 			if field.CanAddr() && field.Addr().CanInterface() {
// 				// Recurse using an interface of the pointer value of the field.
// 				err := SetDefault(field.Addr().Interface())
// 				if err != nil {
// 					return err
// 				}
// 			}

// 			// Move onto the next field.
// 			continue
// 		}

// 		//////
// 		// Start setting values here.
// 		//////

// 		// Check if it's a string or a pointer to a string.
// 		if fieldKind == reflect.String || (fieldKind == reflect.Ptr && field.Elem().Kind() == reflect.String) {
// 			typeField := val.Type().Field(i)

// 			// Get the field tag value.
// 			tag := typeField.Tag.Get(tagName)

// 			// Set the string value to the sanitized string if it's allowed.
// 			// It should always be allowed at this point.
// 			if field.CanSet() {
// 				// Only set if the field is empty.
// 				if fieldKind == reflect.String && field.String() == "" {
// 					// Skip if tag is not defined or ignored.
// 					if tag == "" || tag == "-" {
// 						field.SetString(GenerateUUID())

// 						continue
// 					}

// 					field.SetString(tag)
// 				}
// 			}

// 			continue
// 		}
// 	}

// 	return nil
// }

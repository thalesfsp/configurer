// Package util provides utility functions.
package util

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"github.com/thalesfsp/customerror"
)

const bitSize = 64

// ExportToStruct exports the loaded configuration to the given struct.
func ExportToStruct(v any) error {
	m := make(map[string]interface{})

	for _, e := range os.Environ() {
		if i := strings.Index(e, "="); i >= 0 {
			val := e[i+1:]

			// Convert from string to bool, or int, or float, or string.
			switch v := val; v {
			case "true":
				m[e[:i]] = true
			case "false":
				m[e[:i]] = false
			default:
				if asInt, err := strconv.Atoi(v); err == nil {
					m[e[:i]] = asInt
				} else if asFloat64, err := strconv.ParseFloat(v, bitSize); err == nil {
					m[e[:i]] = asFloat64
				} else {
					m[e[:i]] = v
				}
			}
		}
	}

	jsonStr, err := json.Marshal(m)
	if err != nil {
		return customerror.NewFailedToError(
			"marshal map to json",
			customerror.WithError(err),
		)
	}

	if err := json.Unmarshal(jsonStr, v); err != nil {
		return customerror.NewFailedToError(
			"unmarshal json to struct",
			customerror.WithError(err),
		)
	}

	return nil
}

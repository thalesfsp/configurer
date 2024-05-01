// Package util provides utility functions.
package util

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/thalesfsp/configurer/parsers/env"
	"github.com/thalesfsp/configurer/parsers/jsonp"
	"github.com/thalesfsp/configurer/parsers/toml"
	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/validation"
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

// ParseFile parse file. Extension is used to determine the format.
func ParseFile(ctx context.Context, file *os.File) (map[string]any, error) {
	extension := filepath.Ext(file.Name())

	// Remove the . from the extension.
	extension = strings.Replace(extension, ".", "", 1)

	return ParseContent(ctx, extension, file)
}

// ParseFromText parses the string data in .env format.
func ParseFromText(ctx context.Context, format string, data string) (map[string]any, error) {
	return ParseContent(ctx, format, strings.NewReader(data))
}

// ParseContent parses the string data in .env format.
func ParseContent(ctx context.Context, format string, r io.Reader) (map[string]any, error) {
	switch {
	case format == "env":
		p, err := env.New()
		if err != nil {
			return nil, err
		}

		r, err := p.Read(ctx, r)
		if err != nil {
			return nil, err
		}

		return r, nil
	case format == "json":
		p, err := jsonp.New()
		if err != nil {
			return nil, err
		}

		r, err := p.Read(ctx, r)
		if err != nil {
			return nil, err
		}

		return r, nil
	case format == "yaml" || format == "yml":
		p, err := env.New()
		if err != nil {
			return nil, err
		}

		r, err := p.Read(ctx, r)
		if err != nil {
			return nil, err
		}

		return r, nil
	case format == "toml":
		t, err := toml.New()
		if err != nil {
			return nil, err
		}

		r, err := t.Read(ctx, r)
		if err != nil {
			return nil, err
		}

		return r, nil
	default:
		return nil, customerror.
			NewInvalidError("format, allowed: env.*, json, yaml | yml, toml")
	}
}

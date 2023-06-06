package env

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/thalesfsp/configurer/parser"
	"github.com/thalesfsp/validation"
)

//////
// Vars, consts, and types.
//////

// Name of the parser.
const Name = "env"

// ENV parser.
type ENV struct {
	*parser.Parser `validate:"required"`
}

//////
// Methods.
//////

// Read implementation of the Reader interface for ENV files.
func (e *ENV) Read(ctx context.Context, r io.Reader) (map[string]any, error) {
	values := make(map[string]any)
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid line: %s", line)
		}

		key := parts[0]
		value := parts[1]

		// Remove any \" from the beginning and end of the value.
		value = strings.TrimPrefix(value, "\"")
		value = strings.TrimSuffix(value, "\"")

		values[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return values, nil
}

//////
// Factory.
//////

// New creates a new converter.
func New() (*ENV, error) {
	// Enforces interface implementation.
	var _ parser.IParser = (*ENV)(nil)

	p, err := parser.New(Name)
	if err != nil {
		return nil, err
	}

	// Parser instance.
	pI := &ENV{
		Parser: p,
	}

	// Validation.
	if err := validation.Validate(pI); err != nil {
		return nil, err
	}

	return pI, nil
}

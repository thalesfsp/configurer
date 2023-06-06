package jsonp

import (
	"context"
	"encoding/json"
	"io"

	"github.com/thalesfsp/configurer/parser"
	"github.com/thalesfsp/validation"
)

//////
// Vars, consts, and types.
//////

// Name of the parser.
const Name = "json"

// JSON parser.
type JSON struct {
	*parser.Parser `validate:"required"`
}

//////
// Methods.
//////

// Read implementation of the Reader interface for JSON files.
func (e *JSON) Read(ctx context.Context, r io.Reader) (map[string]any, error) {
	values := make(map[string]any)

	if err := json.NewDecoder(r).Decode(&values); err != nil {
		return nil, err
	}

	return values, nil
}

//////
// Factory.
//////

// New creates a new converter.
func New() (*JSON, error) {
	// Enforces interface implementation.
	var _ parser.IParser = (*JSON)(nil)

	p, err := parser.New(Name)
	if err != nil {
		return nil, err
	}

	// Parser instance.
	pI := &JSON{
		Parser: p,
	}

	// Validation.
	if err := validation.Validate(pI); err != nil {
		return nil, err
	}

	return pI, nil
}

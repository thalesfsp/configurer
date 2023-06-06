package toml

import (
	"context"
	"io"

	"github.com/pelletier/go-toml"
	"github.com/thalesfsp/configurer/parser"
	"github.com/thalesfsp/validation"
)

//////
// Vars, consts, and types.
//////

// Name of the parser.
const Name = "toml"

// TOML parser.
type TOML struct {
	*parser.Parser `validate:"required"`
}

//////
// Methods.
//////

// Read implementation of the Reader interface for TOML files.
func (e *TOML) Read(ctx context.Context, r io.Reader) (map[string]any, error) {
	values := make(map[string]any)

	if err := toml.NewDecoder(r).Decode(&values); err != nil {
		return nil, err
	}

	return values, nil
}

//////
// Factory.
//////

// New creates a new converter.
func New() (*TOML, error) {
	// Enforces interface implementation.
	var _ parser.IParser = (*TOML)(nil)

	p, err := parser.New(Name)
	if err != nil {
		return nil, err
	}

	// Parser instance.
	pI := &TOML{
		Parser: p,
	}

	// Validation.
	if err := validation.Validate(pI); err != nil {
		return nil, err
	}

	return pI, nil
}

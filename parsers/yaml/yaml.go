package yaml

import (
	"context"
	"io"

	"github.com/thalesfsp/configurer/parser"
	"github.com/thalesfsp/validation"
	"gopkg.in/yaml.v3"
)

//////
// Vars, consts, and types.
//////

// Name of the parser.
const Name = "yaml"

// YAML parser.
type YAML struct {
	*parser.Parser `validate:"required"`
}

//////
// Methods.
//////

// Read implementation of the Reader interface for YAML files.
func (e *YAML) Read(ctx context.Context, r io.Reader) (map[string]any, error) {
	var result map[string]any

	if err := yaml.NewDecoder(r).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

//////
// Factory.
//////

// New creates a new converter.
func New() (*YAML, error) {
	// Enforces interface implementation.
	var _ parser.IParser = (*YAML)(nil)

	p, err := parser.New(Name)
	if err != nil {
		return nil, err
	}

	// Parser instance.
	pI := &YAML{
		Parser: p,
	}

	// Validation.
	if err := validation.Validate(pI); err != nil {
		return nil, err
	}

	return pI, nil
}

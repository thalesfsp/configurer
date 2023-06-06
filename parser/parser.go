package parser

import (
	"context"
	"io"

	"github.com/thalesfsp/configurer/internal/logging"
	"github.com/thalesfsp/validation"
)

//////
// Vars, consts, and types.
//////

// Type of the entity.
const Type = "parser"

// IParser defines what a Parser should do.
type IParser interface {
	// Read content and return a map of values.
	Read(ctx context.Context, content io.Reader) (map[string]any, error)
}

// Parser is able to read a content and return a map of values.
type Parser struct {
	// Logger is parser's logger.
	Logger *logging.Logger `json:"-" validate:"required"`

	// Name is the name of the parser.
	Name string `json:"name" validate:"required,gte=3,lte=50"`
}

//////
// Factory.
//////

// New creates a new converter.
func New(name string) (*Parser, error) {
	parser := &Parser{
		Logger: logging.Get().Child(name),
		Name:   name,
	}

	if err := validation.Validate(parser); err != nil {
		return nil, err
	}

	return parser, nil
}

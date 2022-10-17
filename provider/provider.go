// Package provider provides the building blocks for creating a provider.
package provider

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"github.com/thalesfsp/configurer/internal/logging"
	"github.com/thalesfsp/configurer/internal/validation"
	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/customerror"
)

const bitSize = 64

// IProvider defines what a provider does.
type IProvider interface {
	// ExportToStruct exports the loaded configuration to the given struct.
	ExportToStruct(v any) error

	// GetLogger returns the logger.
	GetLogger() *logging.Logger

	// GetOverride returns the override flag.
	GetOverride() bool

	// Load retrieves the configuration, and exports it to the environment.
	Load(ctx context.Context, opts ...option.KeyFunc) error
}

// Provider contains common settings for all providers.
type Provider struct {
	// Logger is provider's logger.
	Logger *logging.Logger `json:"-" validate:"required"`

	// Name is the name of the provider.
	Name string `json:"name" validate:"required,gte=3,lte=50"`

	// Override is the flag that indicates if the provider should override
	// existing environment variables. Default is `false`.
	Override bool `json:"override"`
}

// GetLogger returns the logger.
func (p *Provider) GetLogger() *logging.Logger {
	return p.Logger
}

// GetOverride returns the override flag.
func (p *Provider) GetOverride() bool {
	return p.Override
}

// ExportToStruct exports the loaded configuration to the given struct.
func (p *Provider) ExportToStruct(v any) error {
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

// New creates a new provider.
func New(name string, override bool) (*Provider, error) {
	provider := &Provider{
		Logger:   logging.Get().Child(name),
		Name:     name,
		Override: override,
	}

	if err := validation.ValidateStruct(provider); err != nil {
		return nil, err
	}

	return provider, nil
}

// Package provider provides the building blocks for creating a provider.
package provider

import (
	"context"

	"github.com/thalesfsp/configurer/internal/logging"
	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/util"
	"github.com/thalesfsp/validation"
)

//////
// Vars, consts, and types.
//////

// IProvider defines what a provider does.
type IProvider interface {
	// ExportToStruct exports the loaded configuration to the given struct.
	ExportToStruct(v any) error

	// GetName returns the name of the provider.
	GetName() string

	// GetLogger returns the logger.
	GetLogger() *logging.Logger

	// GetOverride returns the override flag.
	GetOverride() bool

	// Load retrieves the configuration, and exports it to the environment.
	Load(ctx context.Context, opts ...option.KeyFunc) (map[string]string, error)

	// Write stores a new secret in the Vault.
	Write(ctx context.Context, values map[string]interface{}) error
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

//////
// Methods.
//////

// GetName returns the name of the provider.
func (p *Provider) GetName() string {
	return p.Name
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
	return util.Dump(v)
}

// New creates a new provider.
func New(name string, override bool) (*Provider, error) {
	provider := &Provider{
		Logger:   logging.Get().Child(name),
		Name:     name,
		Override: override,
	}

	if err := validation.Validate(provider); err != nil {
		return nil, err
	}

	return provider, nil
}

// Package vault provides a Vault provider.
package vault

import (
	"context"
	"fmt"

	vault "github.com/hashicorp/vault/api"
	"github.com/thalesfsp/configurer/internal/validation"
	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/provider"
	"github.com/thalesfsp/customerror"
)

// Config is an alias to Vault configuration.
type Config = *vault.Config

// Auth is Vault authentication information.
type Auth struct {
	Address   string `json:"address" validate:"required"`
	AppRole   string `json:"-" validate:"omitempty,gte=1"`
	Namespace string `json:"-" validate:"omitempty,gte=1"`
	Token     string `json:"-" validate:"required"`
}

// SecretInformation is the information about a secret, where to retrieve it.
type SecretInformation struct {
	MountPath  string `json:"-" validate:"required"`
	SecretPath string `json:"-" validate:"required"`
}

// Vault provider definition.
type Vault struct {
	client             *vault.Client `json:"-" validate:"required,dive"`
	*provider.Provider `json:"-" validate:"required"`

	*Auth              `json:"-" validate:"required,dive"`
	*SecretInformation `json:"-" validate:"required,dive"`
}

// Load retrieves the configuration, and exports it to the environment.
func (v *Vault) Load(ctx context.Context, opts ...option.KeyFunc) (map[string]string, error) {
	secret, err := v.client.KVv2(v.SecretInformation.MountPath).Get(ctx, v.SecretInformation.SecretPath)
	if err != nil {
		return nil, customerror.NewFailedToError("get secret", customerror.WithError(err))
	}

	finalValues := make(map[string]string)

	// Should export secrets to the environment.
	for key, value := range secret.Data {
		// Should allow to specify options.
		for _, opt := range opts {
			key = opt(key)
		}

		finalValue, err := provider.ExportToEnvVar(v, key, value)
		if err != nil {
			return nil, err
		}

		finalValues[key] = finalValue
	}

	return finalValues, nil
}

// NewWithConfig is the same as New but allows to set/pass additional
// configuration to the Vault client. If `config` is set to `nil`,
// Vault will use configuration from `DefaultConfig()`, which is
// the recommended starting configuration.
func NewWithConfig(
	override bool,
	authInformation *Auth,
	secretInformation *SecretInformation,
	config Config,
) (provider.IProvider, error) {
	provider, err := provider.New("vault", override)
	if err != nil {
		return nil, err
	}

	v := &Vault{
		Provider: provider,

		Auth:              authInformation,
		SecretInformation: secretInformation,
	}

	if err := validation.ValidateStruct(v); err != nil {
		return nil, err
	}

	client, err := vault.NewClient(config)
	if err != nil {
		return nil, customerror.NewFailedToError("initialize Vault client", customerror.WithError(err))
	}

	if err := client.SetAddress(v.Auth.Address); err != nil {
		return nil, customerror.NewFailedToError("set Vault address", customerror.WithError(err))
	}

	client.SetToken(v.Auth.Token)

	// Should be able to specify namespace.
	if v.Auth.Namespace != "" {
		client.SetNamespace(v.Auth.Namespace)
	}

	v.client = client

	// Should allow to login with AppRole.
	if v.Auth.AppRole != "" {
		// Role ID.
		resp, err := client.Logical().Read(fmt.Sprintf("auth/approle/role/%s/role-id", v.Auth.AppRole))
		if err != nil {
			return nil, customerror.NewFailedToError("read role id", customerror.WithError(err))
		}

		if resp == nil {
			return nil, customerror.NewMissingError(fmt.Sprintf("role %s", v.Auth.AppRole), customerror.WithError(err))
		}

		roleID := resp.Data["role_id"]
		if roleID == "" {
			return nil, customerror.NewRequiredError("role id")
		}

		// Secret ID.
		data := map[string]interface{}{}

		resp, err = client.Logical().Write(fmt.Sprintf("auth/approle/role/%s/secret-id", v.Auth.AppRole), data)
		if err != nil {
			return nil, customerror.NewFailedToError("read secret id", customerror.WithError(err))
		}

		secretID := resp.Data["secret_id"]
		if secretID == "" {
			return nil, customerror.NewRequiredError("secret id")
		}

		// Login.
		secret, err := client.Logical().Write("auth/approle/login", map[string]interface{}{
			"role_id":   roleID,
			"secret_id": secretID,
		})
		if err != nil {
			return nil, customerror.NewFailedToError("login with approle", customerror.WithError(err))
		}

		if secret == nil {
			return nil, customerror.NewRequiredError("secret (login with approle)")
		}

		client.SetToken(secret.Auth.ClientToken)
	}

	return v, nil
}

// New sets up a new Vault provider. It'll pull secrets from Hashicorp Vault,
// and then exports to the environment.
//
// It supports the following authentication methods:
// - AppRole
// - Token
//
// The following environment variables can be used to configure the provider:
// - VAULT_ADDR: The address of the Vault server.
// - VAULT_APP_ROLE: The AppRole to use for authentication.
// - VAULT_NAMESPACE: The Vault namespace to use for authentication.
// - VAULT_TOKEN: The token to use for authentication.
//
// NOTE: If no app role is set, the provider will default to using token.
// NOTE: Already exported environment variables have precedence over
// loaded ones. Set the overwrite flag to true to override them.
func New(
	override bool,
	authInformation *Auth,
	secretInformation *SecretInformation,
) (provider.IProvider, error) {
	return NewWithConfig(override, authInformation, secretInformation, nil)
}

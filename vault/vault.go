// Package vault provides a Vault provider.
//
// Setting up the environment to run the application. There are two methods to
// set up the environment to run the application.
//
// Flags (not recommended)
//
// Values are set by specifying flags. In the following example, values are
// set and then the env command is run.
//
//	configurer l v \
//	  --address     "{address}" \
//	  --role-id     "xyz" \
//	  --app-role    "{project_name}" \
//	  --secret-id   "xyz" \
//	  --mount-path  "kv" \
//	  --namespace   "{namespace}" \
//	  --secret-path "/{project_name}/{environment}/{service_name}/main" -- env
//
// Environment Variables (this is the recommended, and preferred way)
//
// Setup values are set by specifying environment variables. In the following
// example, values are set and then the env command is run. It's cleaner and
// more secure.
//
//	export VAULT_ADDR="{address}"
//	export VAULT_APP_ROLE_ID="xyz"
//	export VAULT_APP_ROLE={project_name}
//	export VAULT_APP_SECRET_ID="xyz"
//	export VAULT_MOUNT_PATH="kv"
//	export VAULT_NAMESPACE="{namespace}"
//	export VAULT_SECRET_PATH="/{project_name}/{environment}/{service_name}/main"
//
//	configurer l v -- env
package vault

import (
	"context"

	vault "github.com/hashicorp/vault/api"
	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/provider"
	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/validation"
)

// Name of the provider.
const Name = "vault"

// Config is an alias to Vault configuration.
type Config = *vault.Config

// Auth is Vault authentication information.
type Auth struct {
	Address   string `json:"address" validate:"required"`
	AppRole   string `json:"-" validate:"omitempty,gte=1"`
	Namespace string `json:"-" validate:"omitempty,gte=1"`
	RoleID    string `json:"role_id" validate:"omitempty,gte=1"`
	SecretID  string `json:"secret_id" validate:"omitempty,gte=1"`
	Token     string `json:"-" validate:"omitempty,gte=1"`
}

// SecretInformation is the information about a secret, where to retrieve it.
type SecretInformation struct {
	MountPath  string `json:"-" validate:"required"`
	SecretPath string `json:"-" validate:"required"`
}

// Vault provider definition.
type Vault struct {
	client             *vault.Client `json:"-" validate:"required"`
	*provider.Provider `json:"-" validate:"required"`

	*Auth              `json:"-" validate:"required"`
	*SecretInformation `json:"-" validate:"required"`
}

// Load retrieves the configuration, and exports it to the environment.
func (v *Vault) Load(ctx context.Context, opts ...option.LoadKeyFunc) (map[string]string, error) {
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

// Write stores a new secret.
//
// NOTE: Not all providers support writing secrets.
func (v *Vault) Write(ctx context.Context, values map[string]interface{}, opts ...option.WriteFunc) error {
	// Ensure the secret values are not nil.
	if values == nil {
		return customerror.NewRequiredError("values")
	}

	// Process the options.
	var options option.Write

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return err
		}
	}

	// Write the secret to the Vault.
	if _, err := v.
		client.
		KVv2(v.SecretInformation.MountPath).
		Put(ctx, v.SecretInformation.SecretPath, values); err != nil {
		return customerror.NewFailedToError("write secret", customerror.WithError(err))
	}

	return nil
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

	if err := validation.Validate(v); err != nil {
		return nil, err
	}

	// Create a new Vault client.
	client, err := vault.NewClient(config)
	if err != nil {
		return nil, customerror.NewFailedToError("initialize Vault client", customerror.WithError(err))
	}

	// Sets the Vault address.
	if err := client.SetAddress(v.Auth.Address); err != nil {
		return nil, customerror.NewFailedToError("set Vault address", customerror.WithError(err))
	}

	// Should be able to specify namespace.
	if v.Auth.Namespace != "" {
		client.SetNamespace(v.Auth.Namespace)
	}

	// Should allow to login with AppRole.
	if v.Auth.AppRole != "" {
		if v.Auth.RoleID == "" || v.Auth.SecretID == "" {
			return nil, customerror.NewRequiredError("role_id and secret_id (login with approle)")
		}

		// Login.
		resp, err := client.Logical().Write("auth/approle/login", map[string]interface{}{
			"role_id":   v.Auth.RoleID,
			"secret_id": v.Auth.SecretID,
		})
		if err != nil {
			return nil, customerror.NewFailedToError("login with approle", customerror.WithError(err))
		}

		if resp == nil {
			return nil, customerror.NewMissingError("resp (login with approle)")
		}

		client.SetToken(resp.Auth.ClientToken)
	} else { // Otherwise, should login with token.
		if v.Auth.Token == "" {
			return nil, customerror.NewRequiredError("token (login with approle)")
		}

		client.SetToken(v.Auth.Token)
	}

	// Sets the Vault client.
	v.client = client

	return v, nil
}

// New sets up a new Vault provider. It'll pull secrets from Hashicorp Vault,
// and then exports to the environment.
//
// It supports the following authentication methods:
//
//   - AppRole
//   - Token
//
// The following environment variables can be used to configure the provider:
//
//   - VAULT_ADDR: The address of the Vault server.
//   - VAULT_APP_ROLE_ID: AppRole Role ID
//   - VAULT_APP_ROLE: The AppRole to use for authentication.
//   - VAULT_APP_SECRET_ID: AppRole Secret ID
//   - VAULT_NAMESPACE: The Vault namespace to use for authentication.
//   - VAULT_TOKEN: The token to use for authentication.
//
// NOTE: If no app role is set, the provider will default to using token.
//
// NOTE: Already exported environment variables have precedence over
// loaded ones. Set the overwrite flag to true to override them.
func New(
	override bool,
	authInformation *Auth,
	secretInformation *SecretInformation,
) (provider.IProvider, error) {
	return NewWithConfig(override, authInformation, secretInformation, nil)
}

package azkv

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/provider"
	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/validation"
)

//////
// Vars, consts, and types.
//////

// Name of the provider.
const Name = "azkv"

// Config contains Azure Key Vault configuration.
type Config struct {
	VaultURL    string   `json:"vault_url"    validate:"required,url"`
	SecretNames []string `json:"secret_names" validate:"omitempty,dive,required"`
}

// AZKV provider definition.
type AZKV struct {
	client             *azsecrets.Client `json:"-" validate:"required"`
	*provider.Provider `json:"-" validate:"required"`

	*Config `json:"-" validate:"required"`
}

//////
// Methods.
//////

// Load retrieves secrets from Azure Key Vault and exports them to the environment.
func (a *AZKV) Load(ctx context.Context, opts ...option.LoadKeyFunc) (map[string]string, error) {
	secretNames := a.Config.SecretNames

	if len(secretNames) == 0 {
		var err error

		secretNames, err = a.listSecretNames(ctx)
		if err != nil {
			return nil, err
		}
	}

	finalValues := make(map[string]string)

	for _, secretName := range secretNames {
		result, err := a.client.GetSecret(ctx, secretName, "", nil)
		if err != nil {
			return nil, customerror.NewFailedToError(
				fmt.Sprintf("get secret '%s'", secretName),
				customerror.WithError(err),
			)
		}

		if result.Value == nil {
			return nil, customerror.NewInvalidError(
				fmt.Sprintf("secret '%s' payload is missing value", secretName),
			)
		}

		if !a.GetRawValue() {
			secretData, isJSONObject := parseSecretData(*result.Value)
			if isJSONObject {
				for key, value := range secretData {
					if err := a.exportValue(finalValues, key, value, opts); err != nil {
						return nil, err
					}
				}

				continue
			}
		}

		key := strings.ReplaceAll(secretName, "-", "_")
		if err := a.exportValue(finalValues, key, *result.Value, opts); err != nil {
			return nil, err
		}
	}

	return finalValues, nil
}

func (a *AZKV) listSecretNames(ctx context.Context) ([]string, error) {
	var secretNames []string

	pager := a.client.NewListSecretPropertiesPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, customerror.NewFailedToError(
				"list secret properties",
				customerror.WithError(err),
			)
		}

		for _, secret := range page.Value {
			if secret == nil || secret.ID == nil {
				return nil, customerror.NewInvalidError("listed secret payload is missing ID")
			}

			name := secretNameFromID(string(*secret.ID))
			if name == "" {
				return nil, customerror.NewInvalidError("listed secret payload has an invalid ID")
			}

			secretNames = append(secretNames, name)
		}
	}

	return secretNames, nil
}

func secretNameFromID(id string) string {
	parts := strings.Split(strings.Trim(id, "/"), "/")
	for i, part := range parts {
		if part == "secrets" && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	return ""
}

func (a *AZKV) exportValue(
	finalValues map[string]string,
	key string,
	value interface{},
	opts []option.LoadKeyFunc,
) error {
	for _, opt := range opts {
		key = opt(key)
	}

	finalValue, err := provider.ExportToEnvVar(a, key, value)
	if err != nil {
		return err
	}

	finalValues[key] = finalValue

	return nil
}

// Write stores each value as an Azure Key Vault secret.
func (a *AZKV) Write(ctx context.Context, values map[string]interface{}, opts ...option.WriteFunc) error {
	if values == nil {
		return customerror.NewRequiredError("values")
	}

	var options option.Write

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return err
		}
	}

	for key, value := range values {
		if key == "" {
			return customerror.NewInvalidError("secret name can't be empty")
		}

		secretName := strings.ReplaceAll(key, "_", "-")
		secretValue := fmt.Sprintf("%v", value)

		if _, err := a.client.SetSecret(
			ctx,
			secretName,
			azsecrets.SetSecretParameters{Value: &secretValue},
			nil,
		); err != nil {
			return customerror.NewFailedToError(
				fmt.Sprintf("set secret '%s'", secretName),
				customerror.WithError(err),
			)
		}
	}

	return nil
}

//////
// Helpers.
//////

func parseSecretData(secretValue string) (map[string]interface{}, bool) {
	value := secretValue

	for range 4 {
		var secretData map[string]interface{}
		if err := json.Unmarshal([]byte(value), &secretData); err == nil && secretData != nil {
			return secretData, true
		}

		var inner string
		if err := json.Unmarshal([]byte(value), &inner); err != nil {
			return nil, false
		}

		value = inner
	}

	return nil, false
}

//////
// Constructors.
//////

// NewWithConfig creates an Azure Key Vault provider with a supplied credential
// and Azure client options.
func NewWithConfig(
	override, rawValue bool,
	config *Config,
	credential azcore.TokenCredential,
	clientOptions *azsecrets.ClientOptions,
) (provider.IProvider, error) {
	if config == nil {
		return nil, customerror.NewRequiredError("config")
	}

	if err := validation.Validate(config); err != nil {
		return nil, err
	}

	if credential == nil {
		return nil, customerror.NewRequiredError("credential")
	}

	baseProvider, err := provider.New(Name, override, rawValue)
	if err != nil {
		return nil, err
	}

	azkv := &AZKV{
		Provider: baseProvider,
		Config:   config,
	}

	if err := validation.Validate(azkv); err != nil {
		return nil, err
	}

	client, err := azsecrets.NewClient(config.VaultURL, credential, clientOptions)
	if err != nil {
		return nil, customerror.NewFailedToError(
			"initialize Azure Key Vault client",
			customerror.WithError(err),
		)
	}

	azkv.client = client

	return azkv, nil
}

// New creates an Azure Key Vault provider using Azure's default credential chain.
//
// The AZURE_KEY_VAULT_URL environment variable can be used by CLI callers to
// configure the vault URL. When SecretNames is empty, Load lists and retrieves
// every secret in the vault.
func New(
	override, rawValue bool,
	config *Config,
) (provider.IProvider, error) {
	if config == nil {
		return nil, customerror.NewRequiredError("config")
	}

	if err := validation.Validate(config); err != nil {
		return nil, err
	}

	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, customerror.NewFailedToError(
			"initialize Azure default credential",
			customerror.WithError(err),
		)
	}

	return NewWithConfig(override, rawValue, config, credential, nil)
}

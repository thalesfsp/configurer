// Package awssm provides an AWS Secrets Manager provider.
//
// Setting up the environment to run the application. There are two methods to
// set up the environment to run the application.
//
// Flags (not recommended)
//
// Values are set by specifying flags. In the following example, values are
// set and then the env command is run.
//
//	configurer l awssm \
//	  --region      "us-east-1" \
//	  --secret-name "myapp/prod/secrets" \
//	  --profile     "default" -- env
//
// Environment Variables (this is the recommended, and preferred way)
//
// Setup values are set by specifying environment variables. In the following
// example, values are set and then the env command is run. It's cleaner and
// more secure.
//
//	export AWS_REGION="us-east-1"
//	export AWS_PROFILE="default"
//	export AWSSM_SECRET_NAME="myapp/prod/secrets"
//
//	configurer l awssm -- env
package awssm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/provider"
	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/validation"
)

// Name of the provider.
const Name = "awssm"

// Config contains AWS configuration settings.
type Config struct {
	Region    string `json:"region"     validate:"omitempty,gte=1"`
	Profile   string `json:"profile"    validate:"omitempty,gte=1"`
	AccessKey string `json:"access_key" validate:"omitempty,gte=1"`
	SecretKey string `json:"secret_key" validate:"omitempty,gte=1"`
}

// SecretInformation contains information about which secrets to retrieve.
type SecretInformation struct {
	SecretNames []string `json:"secret_names" validate:"required,gte=1"`
}

// AWSSM provider definition.
type AWSSM struct {
	client             *secretsmanager.Client `json:"-" validate:"required"`
	*provider.Provider `json:"-" validate:"required"`

	*Config            `json:"-" validate:"required"`
	*SecretInformation `json:"-" validate:"required"`
}

// Load retrieves the configuration from AWS Secrets Manager and exports it to the environment.
func (a *AWSSM) Load(ctx context.Context, opts ...option.LoadKeyFunc) (map[string]string, error) {
	finalValues := make(map[string]string)

	// Iterate through all specified secret names
	for _, secretName := range a.SecretInformation.SecretNames {
		// Get the secret value from AWS Secrets Manager
		input := &secretsmanager.GetSecretValueInput{
			SecretId: aws.String(secretName),
		}

		result, err := a.client.GetSecretValue(ctx, input)
		if err != nil {
			return nil, customerror.NewFailedToError(
				fmt.Sprintf("get secret '%s'", secretName),
				customerror.WithError(err),
			)
		}

		if result.SecretString == nil {
			return nil, customerror.NewMissingError(
				fmt.Sprintf("secret string for '%s'", secretName),
			)
		}

		// Try to parse as JSON first, if it fails treat as plain string
		var secretData map[string]interface{}
		if err := json.Unmarshal([]byte(*result.SecretString), &secretData); err != nil {
			// If it's not JSON, treat the entire secret as a single key-value pair
			// Use the secret name (last part after /) as the key
			key := secretName
			if lastSlash := len(secretName) - 1; lastSlash >= 0 {
				for i := lastSlash; i >= 0; i-- {
					if secretName[i] == '/' {
						key = secretName[i+1:]
						break
					}
				}
			}

			// Apply key transformation options
			for _, opt := range opts {
				key = opt(key)
			}

			finalValue, err := provider.ExportToEnvVar(a, key, *result.SecretString)
			if err != nil {
				return nil, err
			}

			finalValues[key] = finalValue
		} else {
			// If it's JSON, export each key-value pair
			for key, value := range secretData {
				// Apply key transformation options
				for _, opt := range opts {
					key = opt(key)
				}

				finalValue, err := provider.ExportToEnvVar(a, key, value)
				if err != nil {
					return nil, err
				}

				finalValues[key] = finalValue
			}
		}
	}

	return finalValues, nil
}

// Write stores a new secret in AWS Secrets Manager.
//
// NOTE: Not all providers support writing secrets.
func (a *AWSSM) Write(ctx context.Context, values map[string]interface{}, opts ...option.WriteFunc) error {
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

	// For writing, we'll use the first secret name as the target
	if len(a.SecretInformation.SecretNames) == 0 {
		return customerror.NewRequiredError("secret_names for write operation")
	}

	secretName := a.SecretInformation.SecretNames[0]

	// Convert the values to JSON
	secretData, err := json.Marshal(values)
	if err != nil {
		return customerror.NewFailedToError("marshal secret data", customerror.WithError(err))
	}

	// Check if secret exists first
	_, err = a.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})

	if err != nil {
		// Secret doesn't exist, create it
		_, err = a.client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
			Name:         aws.String(secretName),
			SecretString: aws.String(string(secretData)),
		})
		if err != nil {
			return customerror.NewFailedToError("create secret", customerror.WithError(err))
		}
	} else {
		// Secret exists, update it
		_, err = a.client.UpdateSecret(ctx, &secretsmanager.UpdateSecretInput{
			SecretId:     aws.String(secretName),
			SecretString: aws.String(string(secretData)),
		})
		if err != nil {
			return customerror.NewFailedToError("update secret", customerror.WithError(err))
		}
	}

	return nil
}

// NewWithConfig creates a new AWS Secrets Manager provider with custom AWS configuration.
func NewWithConfig(
	override, rawValue bool,
	config *Config,
	secretInformation *SecretInformation,
	awsConfig aws.Config,
) (provider.IProvider, error) {
	// Validate input parameters
	if config == nil {
		return nil, customerror.NewRequiredError("config")
	}
	if secretInformation == nil {
		return nil, customerror.NewRequiredError("secret information")
	}

	provider, err := provider.New("awssm", override, rawValue)
	if err != nil {
		return nil, err
	}

	awssm := &AWSSM{
		Provider: provider,

		Config:            config,
		SecretInformation: secretInformation,
	}

	if err := validation.Validate(awssm); err != nil {
		return nil, err
	}

	// Create AWS Secrets Manager client
	awssm.client = secretsmanager.NewFromConfig(awsConfig)

	return awssm, nil
}

// New creates a new AWS Secrets Manager provider. It'll pull secrets from AWS Secrets Manager,
// and then exports to the environment.
//
// The following environment variables can be used to configure the provider:
//
//   - AWS_REGION: The AWS region where secrets are stored.
//   - AWS_PROFILE: The AWS profile to use for authentication.
//   - AWS_ACCESS_KEY_ID: The AWS access key ID (not recommended, use IAM roles instead).
//   - AWS_SECRET_ACCESS_KEY: The AWS secret access key (not recommended, use IAM roles instead).
//   - AWSSM_SECRET_NAME: Comma-separated list of secret names to load.
//
// NOTE: It's recommended to use IAM roles for authentication instead of access keys.
//
// NOTE: Already exported environment variables have precedence over
// loaded ones. Set the override flag to true to override them.
func New(
	override, rawValue bool,
	config *Config,
	secretInformation *SecretInformation,
) (provider.IProvider, error) {
	// Validate input parameters
	if config == nil {
		return nil, customerror.NewRequiredError("config")
	}
	if secretInformation == nil {
		return nil, customerror.NewRequiredError("secret information")
	}

	//////
	// Load AWS configuration
	//////

	var awsConfig aws.Config
	var err error

	if config.Profile != "" {
		awsConfig, err = awsconfig.LoadDefaultConfig(context.Background(),
			awsconfig.WithRegion(config.Region),
			awsconfig.WithSharedConfigProfile(config.Profile),
		)
	} else {
		awsConfig, err = awsconfig.LoadDefaultConfig(context.Background(),
			awsconfig.WithRegion(config.Region),
		)
	}

	if err != nil {
		return nil, customerror.NewFailedToError("load AWS config", customerror.WithError(err))
	}

	// If access keys are provided.
	if config.AccessKey != "" && config.SecretKey != "" {
		awsConfig.Credentials = aws.CredentialsProviderFunc(func(_ context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     config.AccessKey,
				SecretAccessKey: config.SecretKey,
			}, nil
		})
	}

	return NewWithConfig(override, rawValue, config, secretInformation, awsConfig)
}

package gcpsm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2"
	"github.com/thalesfsp/configurer/option"
	"github.com/thalesfsp/configurer/provider"
	"github.com/thalesfsp/customerror"
	"github.com/thalesfsp/validation"
	googleoption "google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//////
// Vars, consts, and types.
//////

// Name of the provider.
const Name = "gcpsm"

// Config contains Google Cloud configuration settings.
type Config struct {
	ProjectID string `json:"project_id" validate:"required,gte=1"`
}

// SecretInformation contains information about which secrets to retrieve.
type SecretInformation struct {
	SecretNames []string `json:"secret_names" validate:"required,gte=1,dive,required"`
}

type secretManagerClient interface {
	AccessSecretVersion(
		ctx context.Context,
		req *secretmanagerpb.AccessSecretVersionRequest,
		opts ...gax.CallOption,
	) (*secretmanagerpb.AccessSecretVersionResponse, error)
	AddSecretVersion(
		ctx context.Context,
		req *secretmanagerpb.AddSecretVersionRequest,
		opts ...gax.CallOption,
	) (*secretmanagerpb.SecretVersion, error)
	CreateSecret(
		ctx context.Context,
		req *secretmanagerpb.CreateSecretRequest,
		opts ...gax.CallOption,
	) (*secretmanagerpb.Secret, error)
}

// GCPSM provider definition.
type GCPSM struct {
	client             secretManagerClient `json:"-" validate:"required"`
	*provider.Provider `json:"-" validate:"required"`

	*Config            `json:"-" validate:"required"`
	*SecretInformation `json:"-" validate:"required"`
}

var newSMClient = func(ctx context.Context, opts ...googleoption.ClientOption) (secretManagerClient, error) {
	return secretmanager.NewClient(ctx, opts...)
}

//////
// Provider methods.
//////

// Load retrieves configuration from Google Cloud Secret Manager and exports it
// to the environment.
func (g *GCPSM) Load(ctx context.Context, opts ...option.LoadKeyFunc) (map[string]string, error) {
	finalValues := make(map[string]string)

	for _, secretName := range g.SecretInformation.SecretNames {
		resourceName := fmt.Sprintf(
			"projects/%s/secrets/%s/versions/latest",
			g.Config.ProjectID,
			secretName,
		)

		result, err := g.client.AccessSecretVersion(
			ctx,
			&secretmanagerpb.AccessSecretVersionRequest{Name: resourceName},
		)
		if err != nil {
			return nil, customerror.NewFailedToError(
				fmt.Sprintf("get secret '%s'", secretName),
				customerror.WithError(err),
			)
		}

		if result == nil || result.GetPayload() == nil {
			return nil, customerror.NewRequiredError(
				fmt.Sprintf("secret payload for '%s'", secretName),
			)
		}

		payload := result.GetPayload().GetData()
		secretData, isJSONObject := parseSecretData(string(payload))
		if !isJSONObject {
			key := secretKey(secretName)

			for _, opt := range opts {
				key = opt(key)
			}

			finalValue, err := provider.ExportToEnvVar(g, key, string(payload))
			if err != nil {
				return nil, err
			}

			finalValues[key] = finalValue

			continue
		}

		for key, value := range secretData {
			for _, opt := range opts {
				key = opt(key)
			}

			finalValue, err := provider.ExportToEnvVar(g, key, value)
			if err != nil {
				return nil, err
			}

			finalValues[key] = finalValue
		}
	}

	return finalValues, nil
}

// Write stores values as a new secret version. If the target secret does not
// exist, Write creates it with automatic replication first.
func (g *GCPSM) Write(ctx context.Context, values map[string]interface{}, opts ...option.WriteFunc) error {
	if values == nil {
		return customerror.NewRequiredError("values")
	}

	var options option.Write

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return err
		}
	}

	if len(g.SecretInformation.SecretNames) == 0 {
		return customerror.NewRequiredError("secret_names for write operation")
	}

	payload, err := json.Marshal(values)
	if err != nil {
		return customerror.NewFailedToError("marshal secret data", customerror.WithError(err))
	}

	secretName := g.SecretInformation.SecretNames[0]
	parent := fmt.Sprintf("projects/%s/secrets/%s", g.Config.ProjectID, secretName)

	if _, err := g.client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: parent,
		Payload: &secretmanagerpb.SecretPayload{
			Data: payload,
		},
	}); err == nil {
		return nil
	} else if !isNotFound(err) {
		return customerror.NewFailedToError("add secret version", customerror.WithError(err))
	}

	if _, err := g.client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent:   fmt.Sprintf("projects/%s", g.Config.ProjectID),
		SecretId: secretName,
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	}); err != nil && status.Code(err) != codes.AlreadyExists {
		return customerror.NewFailedToError("create secret", customerror.WithError(err))
	}

	if _, err := g.client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: parent,
		Payload: &secretmanagerpb.SecretPayload{
			Data: payload,
		},
	}); err != nil {
		return customerror.NewFailedToError("add secret version", customerror.WithError(err))
	}

	return nil
}

//////
// Constructors.
//////

// NewWithConfig creates a Google Cloud Secret Manager provider with additional
// Google API client options. When no options are supplied, Application Default
// Credentials are used.
func NewWithConfig(
	override, rawValue bool,
	config *Config,
	secretInformation *SecretInformation,
	clientOptions ...googleoption.ClientOption,
) (provider.IProvider, error) {
	if config == nil {
		return nil, customerror.NewRequiredError("config")
	}

	if secretInformation == nil {
		return nil, customerror.NewRequiredError("secret information")
	}

	baseProvider, err := provider.New(Name, override, rawValue)
	if err != nil {
		return nil, err
	}

	gcpsm := &GCPSM{
		Provider:          baseProvider,
		Config:            config,
		SecretInformation: secretInformation,
	}

	if err := validation.Validate(gcpsm); err != nil {
		return nil, err
	}

	client, err := newSMClient(context.Background(), clientOptions...)
	if err != nil {
		return nil, customerror.NewFailedToError(
			"initialize GCP Secret Manager client",
			customerror.WithError(err),
		)
	}

	gcpsm.client = client

	if err := validation.Validate(gcpsm); err != nil {
		return nil, err
	}

	return gcpsm, nil
}

// New creates a Google Cloud Secret Manager provider using Application Default
// Credentials.
func New(
	override, rawValue bool,
	config *Config,
	secretInformation *SecretInformation,
) (provider.IProvider, error) {
	return NewWithConfig(override, rawValue, config, secretInformation)
}

//////
// Helpers.
//////

func parseSecretData(secretString string) (map[string]interface{}, bool) {
	value := secretString

	for range 4 {
		var secretData map[string]interface{}
		if err := json.Unmarshal([]byte(value), &secretData); err == nil {
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

func secretKey(secretName string) string {
	if lastSlash := strings.LastIndex(secretName, "/"); lastSlash >= 0 {
		return secretName[lastSlash+1:]
	}

	return secretName
}

func isNotFound(err error) bool {
	return status.Code(err) == codes.NotFound
}
